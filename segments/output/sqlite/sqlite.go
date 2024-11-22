//go:build cgo
// +build cgo

// Dumps all incoming flow messages to a local sqlite database. The schema used
// for this is preset.
package sqlite

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"

	"github.com/bwNetFlow/flowpipeline/pb"
	"github.com/bwNetFlow/flowpipeline/segments"
)

type Sqlite struct {
	segments.BaseSegment
	db              *sql.DB
	fieldTypes      []string
	fieldNames      []string
	createStatement string
	insertStatement string

	FileName  string // required
	Fields    string // optional comma-separated list of fields to export, default is "", meaning all fields
	BatchSize int    // optional how many flows to hold in memory between INSERTs, default is 1000
}

// Every Segment must implement a New method, even if there isn't any config
// it is interested in.
func (segment Sqlite) New(config map[string]string) segments.Segment {
	newsegment := &Sqlite{}

	if config["filename"] == "" {
		log.Println("[error] Sqlite: This segment requires a 'filename' parameter.")
		return nil
	}
	_, err := sql.Open("sqlite3", config["filename"])
	if err != nil {
		log.Printf("[error] Sqlite: Could not open DB file at %s.", config["filename"])
		return nil
	}
	newsegment.FileName = config["filename"]

	newsegment.BatchSize = 1000
	if config["batchsize"] != "" {
		if parsedBatchSize, err := strconv.ParseUint(config["batchsize"], 10, 32); err == nil {
			if parsedBatchSize == 0 {
				log.Println("[error] Sqlite: Batch size 0 is not allowed. Set this in relation to the expected flows per second.")
				return nil
			}
			newsegment.BatchSize = int(parsedBatchSize)
		} else {
			log.Println("[error] Sqlite: Could not parse 'batchsize' parameter, using default 1000.")
		}
	} else {
		log.Println("[info] Sqlite: 'batchsize' set to default '1000'.")
	}

	// determine field set
	if config["fields"] != "" {
		protofields := reflect.TypeOf(pb.EnrichedFlow{})
		conffields := strings.Split(config["fields"], ",")
		for _, field := range conffields {
			protofield, found := protofields.FieldByName(field)
			if !found {
				log.Printf("[error] sqlite: Field '%s' specified in 'fields' does not exist.", field)
				return nil
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

	// use field set to pre-gen statements
	// create
	var fields []string
	for i, fieldname := range newsegment.fieldNames {
		switch newsegment.fieldTypes[i] {
		case "uint64", "uint32":
			fields = append(fields, fieldname+" INTEGER")
		default:
			fields = append(fields, fieldname+" TEXT")
		}
	}
	newsegment.createStatement = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS flows (%s);`, strings.Join(fields, ","))

	// insert
	qmList := make([]string, 0, len(newsegment.fieldNames))
	for i := 0; i < len(newsegment.fieldNames); i++ {
		qmList = append(qmList, "?")
	}
	valueStrings := make([]string, 0, len(newsegment.fieldNames))
	valueStrings = append(valueStrings, fmt.Sprintf("(%s)", strings.Join(qmList, ",")))
	newsegment.insertStatement = fmt.Sprintf("INSERT INTO flows (%s) VALUES %s", strings.Join(newsegment.fieldNames, ","), strings.Join(valueStrings, ","))

	return newsegment
}

func (segment *Sqlite) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	var err error
	segment.db, err = sql.Open("sqlite3", segment.FileName)
	if err != nil {
		log.Panic(err) // this has already been checked in New
	}
	defer segment.db.Close()

	tx, err := segment.db.Begin()
	if err != nil {
		log.Panicf("[error] Sqlite: Could not start initiation transaction with error: %+v", err)
	}
	_, err = tx.Exec(segment.createStatement)
	if err != nil {
		log.Panicf("[error] Sqlite: Could not create database, check field configuration: %+v", err)
	}
	tx.Commit()

	var unsaved []*pb.EnrichedFlow

	for msg := range segment.In {
		unsaved = append(unsaved, msg)
		if len(unsaved) >= segment.BatchSize {
			err := segment.bulkInsert(unsaved)
			if err != nil {
				log.Printf("[error] %s", err)
			}
			unsaved = []*pb.EnrichedFlow{}
		}
		segment.Out <- msg
	}
	segment.bulkInsert(unsaved)
}

func (segment Sqlite) bulkInsert(unsavedFlows []*pb.EnrichedFlow) error {
	if len(unsavedFlows) == 0 {
		return nil
	}
	tx, err := segment.db.Begin()
	if err != nil {
		log.Printf("[error] Sqlite: Error starting transaction for current batch of %d flows: %+v", len(unsavedFlows), err)
	}
	for _, msg := range unsavedFlows {
		valueArgs := make([]interface{}, 0, len(segment.fieldNames))
		values := reflect.ValueOf(msg).Elem()
		for i, fieldname := range segment.fieldNames {
			protofield := values.FieldByName(fieldname)
			switch segment.fieldTypes[i] {
			case "[]uint8": // this is neccessary for proper formatting
				ipstring := net.IP(protofield.Interface().([]uint8)).String()
				if ipstring == "<nil>" {
					ipstring = ""
				}
				valueArgs = append(valueArgs, ipstring)
			case "string": // this is because doing nothing is also much faster than Sprint
				valueArgs = append(valueArgs, protofield.Interface().(string))
			default:
				valueArgs = append(valueArgs, fmt.Sprint(protofield))
			}
		}
		_, err := tx.Exec(segment.insertStatement, valueArgs...)
		if err != nil {
			log.Printf("[error] Sqlite: Error inserting flow into transaction: %+v", err)
		}
	}
	tx.Commit()
	return nil
}

func init() {
	segment := &Sqlite{}
	segments.RegisterSegment("sqlite", segment)
}
