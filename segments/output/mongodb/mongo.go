//go:build cgo
// +build cgo

// Dumps all incoming flow messages to a local mongodb database using a capped collection to limit the used disk space
package mongodb

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/bwNetFlow/flowpipeline/pb"
	"github.com/bwNetFlow/flowpipeline/segments"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Mongodb struct {
	segments.BaseSegment
	mongodbUri     string
	dbCollection   *mongo.Collection
	fieldTypes     []string
	fieldNames     []string
	ringbufferSize int64

	databaseName   string // default flowdata
	collectionName string // default ringbuffer
	Fields         string // optional comma-separated list of fields to export, default is "", meaning all fields
	BatchSize      int    // optional how many flows to hold in memory between INSERTs, default is 1000
}

// Every Segment must implement a New method, even if there isn't any config
// it is interested in.
func (segment Mongodb) New(configx map[string]string) segments.Segment {
	newsegment := &Mongodb{}

	newsegment, err := fillSegmentWithConfig(newsegment, configx)
	if err != nil {
		log.Printf("[error] '%s'", err.Error())
		return nil
	}

	ctx := context.Background()

	//Test if db connection works
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(newsegment.mongodbUri))
	if err == nil {
		//test if the connection was acutally sucessful
		err = client.Ping(ctx, options.Client().ReadPreference)
	}
	if err != nil {
		log.Printf("[error] mongoDB: Could not open DB connection due to '%s", err.Error())
		return nil
	}
	db := client.Database(newsegment.databaseName)

	// collection in the mongdo should be capped to limit the used disk space
	convertToCappedCollection(db, newsegment)
	return newsegment
}

func (segment *Mongodb) Run(wg *sync.WaitGroup) {
	ctx := context.Background()
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(segment.mongodbUri))
	if err != nil {
		log.Panic(err) // this has already been checked in New
	}
	db := client.Database(segment.databaseName)
	segment.dbCollection = db.Collection(segment.collectionName)

	defer client.Disconnect(ctx)

	var unsaved []*pb.EnrichedFlow

	for msg := range segment.In {
		unsaved = append(unsaved, msg)
		if len(unsaved) >= segment.BatchSize {
			err := segment.bulkInsert(unsaved, ctx)
			if err != nil {
				log.Printf("[error] %s", err)
			}
			unsaved = []*pb.EnrichedFlow{}
		}
		segment.Out <- msg
	}
	segment.bulkInsert(unsaved, ctx)
}

func fillSegmentWithConfig(newsegment *Mongodb, config map[string]string) (*Mongodb, error) {
	if config == nil {
		return newsegment, errors.New("missing configuration for segment mongodb")
	}

	if config["mongodb_uri"] == "" {
		return newsegment, errors.New("mongoDB: mongodb_uri not defined")
	}
	newsegment.mongodbUri = config["mongodb_uri"]

	if config["database"] == "" {
		log.Println("[INFO] mongoDB: no database defined - using default value (flowdata)")
		config["database"] = "flowdata"
	}
	newsegment.databaseName = config["database"]

	if config["collection"] == "" {
		log.Println("[INFO] mongoDB: no collection defined - using default value (ringbuffer)")
		config["collection"] = "ringbuffer"
	}
	newsegment.collectionName = config["collection"]

	var ringbufferSize int64 = 10737418240
	if config["max_disk_usage"] == "" {
		log.Println("[INFO] mongoDB: no ring buffer size defined - using default value (10GB)")
	} else {
		size, err := sizeInBytes(config["max_disk_usage"])
		if err == nil {
			log.Println("[INFO] mongoDB: setting ring buffer size to " + config["max_disk_usage"])
			ringbufferSize = size
		} else {
			log.Println("[Warning] mongoDB: failed setting ring buffer size to " + config["max_disk_usage"] + " - using default as fallback (10GB)")
		}
	}
	newsegment.ringbufferSize = ringbufferSize

	newsegment.BatchSize = 1000
	if config["batchsize"] != "" {
		if parsedBatchSize, err := strconv.ParseUint(config["batchsize"], 10, 32); err == nil {
			if parsedBatchSize == 0 {
				return newsegment, errors.New("MongoDO: Batch size 0 is not allowed. Set this in relation to the expected flows per second")
			}
			newsegment.BatchSize = int(parsedBatchSize)
		} else {
			log.Println("[error] MongoDO: Could not parse 'batchsize' parameter, using default 1000.")
		}
	} else {
		log.Println("[info] MongoDO: 'batchsize' set to default '1000'.")
	}

	// determine field set
	if config["fields"] != "" {
		protofields := reflect.TypeOf(pb.EnrichedFlow{})
		conffields := strings.Split(config["fields"], ",")
		for _, field := range conffields {
			protofield, found := protofields.FieldByName(field)
			if !found {
				return newsegment, errors.New("csv: Field specified in 'fields' does not exist")
			}
			newsegment.fieldNames = append(newsegment.fieldNames, field)
			newsegment.fieldTypes = append(newsegment.fieldTypes, protofield.Type.String())
		}
	} else {
		protofields := reflect.TypeOf(pb.EnrichedFlow{})
		// +-3 skips over protobuf state, sizeCache and unknownFields
		newsegment.fieldNames = make([]string, protofields.NumField()-3)
		newsegment.fieldTypes = make([]string, protofields.NumField()-3)
		for i := 3; i < protofields.NumField(); i++ {
			field := protofields.Field(i)
			newsegment.fieldNames[i-3] = field.Name
			newsegment.fieldTypes[i-3] = field.Type.String()
		}
		newsegment.Fields = config["fields"]
	}

	return newsegment, nil
}

func (segment Mongodb) bulkInsert(unsavedFlows []*pb.EnrichedFlow, ctx context.Context) error {
	// not using transactions due to limitations of capped collectiction
	// ("You cannot write to capped collections in transactions."
	// https://www.mongodb.com/docs/manual/core/capped-collections/)
	if len(unsavedFlows) == 0 {
		return nil
	}
	unsavedFlowData := bson.A{}
	for _, msg := range unsavedFlows {
		singleFlowData := bson.M{}
		values := reflect.ValueOf(msg).Elem()
		for i, fieldname := range segment.fieldNames {
			protofield := values.FieldByName(fieldname)
			switch segment.fieldTypes[i] {
			case "[]uint8": // this is neccessary for proper formatting
				ipstring := net.IP(protofield.Interface().([]uint8)).String()
				if ipstring == "<nil>" {
					ipstring = ""
				}
				singleFlowData[fieldname] = ipstring
			case "string": // this is because doing nothing is also much faster than Sprint
				singleFlowData[fieldname] = protofield.Interface().(string)
			default:
				singleFlowData[fieldname] = fmt.Sprint(protofield)
			}
		}
		unsavedFlowData = append(unsavedFlowData, singleFlowData)
	}
	_, err := segment.dbCollection.InsertMany(ctx, unsavedFlowData)
	if err != nil {
		log.Println("[error] mongoDB: Failed to insert due to " + err.Error())
		return err
	}
	return nil
}

func init() {
	segment := &Mongodb{}
	segments.RegisterSegment("mongodb", segment)
}

func sizeInBytes(sizeStr string) (int64, error) {
	// Split into number and unit
	parts := strings.Fields(sizeStr)
	if len(parts) > 2 || len(parts) < 1 {
		return 0, fmt.Errorf("invalid size format")
	}

	size, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}

	if len(parts) == 1 {
		return size, nil
	}

	// Calculate bytes if a size was provided
	unit := strings.ToUpper(parts[1])
	switch unit {
	case "B":
		return size, nil
	case "KB":
		return size * 1024, nil
	case "MB":
		return size * 1024 * 1024, nil
	case "GB":
		return size * 1024 * 1024 * 1024, nil
	case "TB":
		return size * 1024 * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}

/************************************************************************************************
** Checks if the collection segment.collectionName in the db is a capped collection
** If not it converts it to a capped collection with the size segment.ringbufferSize
*************************************************************************************************/
func convertToCappedCollection(db *mongo.Database, segment *Mongodb) error {
	ctx := context.Background()

	collStats := db.RunCommand(ctx, bson.D{{Key: "collStats", Value: segment.collectionName}})

	var collInfo struct {
		Name    string `bson:"name"`
		Capped  bool   `bson:"capped"`
		MaxSize int32  `bson:"maxSize"`
		Count   int64  `bson:"count"`
		Size    int64  `bson:"size"`
	}

	if collStats.Err() != nil {
		log.Printf("[Error] Failed to check Collection '%s' due to: '%s'\n", segment.collectionName, collStats.Err().Error())
		return collStats.Err()
	}

	if err := collStats.Decode(&collInfo); err != nil {
		return fmt.Errorf("failed to decode collection info: %v", err)
	}

	if collInfo.Count == 0 {
		// Create a new capped collection
		cappedOptions := options.CreateCollection().SetCapped(true).SetSizeInBytes(segment.ringbufferSize)
		err := db.CreateCollection(ctx, segment.collectionName, cappedOptions)
		if err != nil {
			return fmt.Errorf("failed to create capped collection: %v", err)
		}

		log.Printf("[Debug] Capped collection '%s' created successfully.\n", segment.collectionName)
		return nil
	}

	if !collInfo.Capped {
		log.Printf("[Warning] Collection '%s' is not capped. Starting converting it...\n", segment.collectionName)
		db.RunCommand(ctx, bson.D{
			{Key: "convertToCapped", Value: segment.collectionName},
			{Key: "size", Value: segment.ringbufferSize},
		})
		return nil
	}

	log.Printf("[INFO] Collection '%s' is already capped.\n", segment.collectionName)
	if collInfo.MaxSize != int32(segment.ringbufferSize) {
		log.Printf("[Warning] Changing max size of collection '%s' from '%d' to '%d'.\n", segment.collectionName, collInfo.MaxSize, segment.ringbufferSize)
		db.RunCommand(ctx, bson.D{
			{Key: "collMod", Value: segment.collectionName},
			{Key: "cappedSize", Value: segment.ringbufferSize},
		})
	}
	return nil
}
