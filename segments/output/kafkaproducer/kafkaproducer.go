// Produces all received flows to Kafka instance. This segment is based on the
// kafkaconnector library:
// https://github.com/bwNetFlow/kafkaconnector
package kafkaproducer

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/bwNetFlow/flowpipeline/pb"
	"github.com/bwNetFlow/flowpipeline/segments"
	"google.golang.org/protobuf/proto"
)

// All configuration parameters are the same as in the kafkaconsumer segment,
// except for the 'topicsuffix' parameter. This parameter, if set, acts as a
// suffix that is appended to the topic that this segment will produce a given
// flow to. As a static suffix would not make much sense, it is interpreted as
// a flow message field name, which will be used to create different topics
// based on field contents. For instance, setting `topicsuffix: Proto` will
// yield separate topics for each different protocol number occuring in all
// flows. Usually, a sensible application is usage with customer ids (`Cid`).
//
// For more info, see examples/splitter in the repo.

// FIXME: use sarama directly here
type KafkaProducer struct {
	segments.BaseSegment
	Server      string // required
	Topic       string // required
	TopicSuffix string // optional, default is empty
	User        string // required if auth is true
	Pass        string // required if auth is true
	Tls         bool   // optional, default is true
	Auth        bool   // optional, default is true
	Legacy      bool   // optional, default is false

	saramaConfig *sarama.Config
}

func (segment KafkaProducer) New(config map[string]string) segments.Segment {
	var err error
	newsegment := &KafkaProducer{}
	newsegment.saramaConfig = sarama.NewConfig()

	if config["server"] == "" || config["topic"] == "" {
		log.Println("[error] KafkaProducer: Missing required configuration parameters.")
		return nil
	} else {
		newsegment.Server = config["server"]
		newsegment.Topic = config["topic"]
	}

	var legacy bool = false
	if config["legacy"] != "" {
		if parsedTls, err := strconv.ParseBool(config["legacy"]); err == nil {
			legacy = parsedTls
		} else {
			log.Println("[Error] KafkaProducer: Could not parse 'legacy' parameter, using default false.")
		}
	} else {
		log.Println("[Debug] KafkaProducer: 'legacy' set to default false.")
	}
	newsegment.Legacy = legacy

	// set some unconfigurable defaults
	newsegment.saramaConfig.Producer.RequiredAcks = sarama.WaitForLocal       // Only wait for the leader to ack
	newsegment.saramaConfig.Producer.Compression = sarama.CompressionSnappy   // Compress messages
	newsegment.saramaConfig.Producer.Flush.Frequency = 500 * time.Millisecond // Flush batches every 500ms
	newsegment.saramaConfig.Producer.Return.Successes = false                 // this would block until we've read the ACK, just don't
	newsegment.saramaConfig.Producer.Return.Errors = false                    // this would block until we've read the error, but we wouldn't retry anyways

	// TODO: parse and set kafka version
	newsegment.saramaConfig.Version, err = sarama.ParseKafkaVersion("2.4.0")
	if err != nil {
		log.Panicf("Error parsing Kafka version: %v", err)
	}

	// parse config and setup TLS
	var useTls bool = true
	if config["tls"] != "" {
		if parsedTls, err := strconv.ParseBool(config["tls"]); err == nil {
			useTls = parsedTls
		} else {
			log.Println("[error] KafkaProducer: Could not parse 'tls' parameter, using default true.")
		}
	} else {
		log.Println("[info] KafkaProducer: 'tls' set to default true.")
	}
	newsegment.Tls = useTls
	if newsegment.Tls {
		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			log.Panicf("TLS Error: %v", err)
		}
		newsegment.saramaConfig.Net.TLS.Enable = true
		newsegment.saramaConfig.Net.TLS.Config = &tls.Config{RootCAs: rootCAs}
	} else {
		log.Println("[info] KafkaProducer: Disabled TLS, operating unencrypted.")
	}

	// parse config and setup auth
	var useAuth bool = true
	if config["auth"] != "" {
		if parsedAuth, err := strconv.ParseBool(config["auth"]); err == nil {
			useAuth = parsedAuth
		} else {
			log.Println("[error] KafkaProducer: Could not parse 'auth' parameter, using default true.")
		}
	} else {
		log.Println("[info] KafkaProducer: 'auth' set to default true.")
	}

	// parse and configure credentials, if applicable
	if useAuth && (config["user"] == "" || config["pass"] == "") {
		log.Println("[error] KafkaProducer: Missing required configuration parameters for auth.")
		return nil
	} else {
		newsegment.User = config["user"]
		newsegment.Pass = config["pass"]
	}

	// use these credentials
	newsegment.Auth = useAuth
	if newsegment.Auth {
		newsegment.saramaConfig.Net.SASL.Enable = true
		newsegment.saramaConfig.Net.SASL.User = newsegment.User
		newsegment.saramaConfig.Net.SASL.Password = newsegment.Pass
		log.Printf("[info] KafkaProducer: Authenticating as user '%s'.", newsegment.User)
	} else {
		newsegment.saramaConfig.Net.SASL.Enable = false
		log.Println("[info] KafkaProducer: Disabled auth.")
	}

	// warn if we're leaking credentials
	if newsegment.Auth && !newsegment.Tls {
		log.Println("[warning] KafkaProducer: Authentication will be done in plain text!")
	}

	// parse special target topic handling information
	if config["topicsuffix"] != "" {
		fmsg := reflect.ValueOf(pb.EnrichedFlow{})
		field := fmsg.FieldByName(config["topicsuffix"])
		if !field.IsValid() {
			log.Println("[error] KafkaProducer: The 'topicsuffix' is not a valid FlowMessage field.")
			return nil
		}
		fieldtype := field.Type().String()
		if fieldtype != "string" && fieldtype != "uint32" && fieldtype != "uint64" {
			log.Println("[error] KafkaProducer: TopicSuffix must be of type uint or string.")
			return nil
		}
		newsegment.TopicSuffix = config["topicsuffix"]
	} else {
		log.Println("[info] KafkaProducer: 'topicsuffix' set to default disabled.")
	}

	return newsegment
}

func (segment *KafkaProducer) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	producer, err := sarama.NewAsyncProducer(strings.Split(segment.Server, ","), segment.saramaConfig)

	for msg := range segment.In {
		segment.Out <- msg
		var binary []byte
		if segment.Legacy {
			if binary, err = proto.Marshal(msg); err != nil {
				log.Printf("[error] KafkaProducer: Error encoding protobuf. %s", err)
				continue
			}
		} else {
			protoProducerMessage := pb.ProtoProducerMessage{}
			protoProducerMessage.EnrichedFlow = *msg
			if binary, err = protoProducerMessage.MarshalBinary(); err != nil {
				log.Printf("[error] KafkaProducer: Error encoding protobuf. %s", err)
				continue
			}
		}

		if segment.TopicSuffix == "" {
			producer.Input() <- &sarama.ProducerMessage{
				Topic: segment.Topic,
				Value: sarama.ByteEncoder(binary),
			}
		} else {
			fmsg := reflect.ValueOf(msg).Elem()
			field := fmsg.FieldByName(segment.TopicSuffix)
			var suffix string
			switch field.Type().String() {
			case "uint32": // this is because FormatUint is much faster than Sprint
				suffix = strconv.FormatUint(uint64(field.Interface().(uint32)), 10)
			case "uint64": // this is because FormatUint is much faster than Sprint
				suffix = strconv.FormatUint(uint64(field.Interface().(uint64)), 10)
			case "string": // this is because doing nothing is also much faster than Sprint
				suffix = field.Interface().(string)
			default:
				log.Println("[error] KafkaProducer: TopicSuffix must be of type uint or string.")
				segment.ShutdownParentPipeline()
				return
			}
			producer.Input() <- &sarama.ProducerMessage{
				Topic: segment.Topic + "-" + suffix,
				Value: sarama.ByteEncoder(binary),
			}
		}
	}
}

func init() {
	segment := &KafkaProducer{}
	segments.RegisterSegment("kafkaproducer", segment)
}
