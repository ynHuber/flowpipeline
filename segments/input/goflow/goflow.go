// Captures Netflow v9 and feeds flows to the following segments. Currently,
// this segment only uses a limited subset of goflow2 functionality.
// If no configuration option is provided a sflow and a netflow collector will be started.
// netflowLagcy is also built in but currently not tested.
package goflow

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/bwNetFlow/flowpipeline/pb"
	"github.com/bwNetFlow/flowpipeline/segments"
	"google.golang.org/protobuf/proto"

	_ "github.com/netsampler/goflow2/v2/format/binary"
	"github.com/netsampler/goflow2/v2/utils/debug"

	"github.com/netsampler/goflow2/v2/transport"
	_ "github.com/netsampler/goflow2/v2/transport/file"
	_ "github.com/netsampler/goflow2/v2/transport/kafka"

	"github.com/netsampler/goflow2/v2/metrics"
	rawproducer "github.com/netsampler/goflow2/v2/producer/raw"
	"github.com/netsampler/goflow2/v2/utils"
)

type Goflow struct {
	segments.BaseSegment
	Listen     []url.URL // optional, default config value for this slice is "sflow://:6343,netflow://:2055"
	Workers    uint64    // optional, amunt of workers to spawn for each endpoint, default is 1
	Blocking   bool      //optional, default is false
	QueueSize  int       //default is 1000000
	NumSockets int       //default is 1
	goflow_in  chan *pb.EnrichedFlow
}

func (segment Goflow) New(config map[string]string) segments.Segment {

	var listen = "sflow://:6343,netflow://:2055"
	if config["listen"] != "" {
		listen = config["listen"]
	}

	var listenAddressesSlice []url.URL
	for _, listenAddress := range strings.Split(listen, ",") {
		listenAddrUrl, err := url.Parse(listenAddress)
		if err != nil {
			log.Printf("[error] Goflow: error parsing listenAddresses: %e", err)
			return nil
		}
		// Check if given Port can be parsed to int
		_, err = strconv.ParseUint(listenAddrUrl.Port(), 10, 64)
		if err != nil {
			log.Printf("[error] Goflow: Port %s could not be converted to integer", listenAddrUrl.Port())
			return nil
		}

		switch listenAddrUrl.Scheme {
		case "netflow", "sflow", "nfl":
			log.Printf("[info] Goflow: Scheme %s supported.", listenAddrUrl.Scheme)
		default:
			log.Printf("[error] Goflow: Scheme %s not supported.", listenAddrUrl.Scheme)
			return nil
		}

		listenAddressesSlice = append(listenAddressesSlice, *listenAddrUrl)
	}
	log.Printf("[info] Goflow: Configured for for %s", listen)

	var workers uint64 = 1
	if config["workers"] != "" {
		if parsedWorkers, err := strconv.ParseUint(config["workers"], 10, 32); err == nil {
			workers = parsedWorkers
			if workers == 0 {
				log.Println("[error] Goflow: Limiting workers to 0 will not work. Remove this segment or use a higher value >= 1.")
				return nil
			}
		} else {
			log.Println("[error] Goflow: Could not parse 'workers' parameter, using default 1.")
		}
	} else {
		log.Println("[info] Goflow: 'workers' set to default '1'.")
	}

	return &Goflow{
		Listen:  listenAddressesSlice,
		Workers: workers,
	}
}

func (segment *Goflow) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	segment.goflow_in = make(chan *pb.EnrichedFlow)
	segment.startGoFlow(&channelDriver{segment.goflow_in})
	for {
		select {
		case msg, ok := <-segment.goflow_in:
			if !ok {
				// do not return here, as this might leave the
				// segment.In channel blocking in our
				// predecessor segment
				segment.goflow_in = nil // make unavailable for select
				// TODO: think about restarting goflow?
			}
			segment.Out <- msg
		case msg, ok := <-segment.In:
			if !ok {
				return
			}
			segment.Out <- msg
		}
	}
}

type channelDriver struct {
	out chan *pb.EnrichedFlow
}

func (d *channelDriver) Send(key, data []byte) error {
	msg := &pb.EnrichedFlow{}
	// TODO: can we shave of this Unmarshal here and the Marshal in line 138
	if err := proto.Unmarshal(data, msg); err != nil {
		log.Println("[error] Goflow: Conversion error for received flow.")
		return nil
	}
	d.out <- msg
	return nil
}

func (d *channelDriver) Close(context.Context) error {
	close(d.out)
	return nil
}

type myProtobufDriver struct {
}

func (d *myProtobufDriver) Format(data interface{}) ([]byte, []byte, error) {
	msg, ok := data.(proto.Message)
	if !ok {
		return nil, nil, fmt.Errorf("message is not protobuf")
	}
	// TODO: can we shave of this Marshal here and the Unmarshal in line 116
	b, err := proto.Marshal(msg)
	return nil, b, err
}

func (d *myProtobufDriver) Prepare() error             { return nil }
func (d *myProtobufDriver) Init(context.Context) error { return nil }

func init() {
	segment := &Goflow{}
	segments.RegisterSegment("goflow", segment)
}

func (segment *Goflow) startGoFlow(transport transport.TransportInterface) {
	formatter := &myProtobufDriver{}
	var pipes []utils.FlowPipe

	for _, listenAddrUrl := range segment.Listen {
		go func(listenAddrUrl url.URL) {
			var err error

			hostname := listenAddrUrl.Hostname()
			port, _ := strconv.ParseUint(listenAddrUrl.Port(), 10, 64)

			if segment.NumSockets == 0 {
				segment.NumSockets = 1
			}

			if segment.QueueSize == 0 {
				segment.QueueSize = 1000000
			}

			cfg := &utils.UDPReceiverConfig{
				Sockets:          segment.NumSockets,
				Workers:          int(segment.Workers),
				QueueSize:        segment.QueueSize,
				Blocking:         segment.Blocking,
				ReceiverCallback: metrics.NewReceiverMetric(),
			}
			recv, err := utils.NewUDPReceiver(cfg)
			if err != nil {
				log.Println("error creating UDP receiver", slog.String("error", err.Error()))
				os.Exit(1)
			}

			if err != nil {
				slog.Error("error transporter", slog.String("error", err.Error()))
				os.Exit(1)
			}

			cfgPipe := &utils.PipeConfig{
				Format:           formatter,
				Transport:        transport,
				Producer:         &rawproducer.RawProducer{},
				NetFlowTemplater: metrics.NewDefaultPromTemplateSystem, // wrap template system to get Prometheus info
			}

			var pipeline utils.FlowPipe
			switch scheme := listenAddrUrl.Scheme; scheme {
			case "netflow":
				pipeline = utils.NewNetFlowPipe(cfgPipe)
				log.Printf("[info] Goflow: Listening for Netflow v9 on port %d...", port)
			case "sflow":
				pipeline = utils.NewSFlowPipe(cfgPipe)
				log.Printf("[info] Goflow: Listening for sflow on port %d...", port)
			case "flow":
				pipeline = utils.NewFlowPipe(cfgPipe)
				log.Printf("[info] Goflow: Listening for netflow legacy on port %d...", port)
			default:
				log.Fatal("scheme does not exist", slog.String("error", listenAddrUrl.Scheme))
			}

			decodeFunc := pipeline.DecodeFlow
			// intercept panic and generate error
			decodeFunc = debug.PanicDecoderWrapper(decodeFunc)
			// wrap decoder with Prometheus metrics
			decodeFunc = metrics.PromDecoderWrapper(decodeFunc, listenAddrUrl.Scheme)
			pipes = append(pipes, pipeline)

			err = recv.Start(hostname, int(port), decodeFunc)

			if err != nil {
				log.Fatalf("[error] Goflow: %s", err.Error())
			}

		}(listenAddrUrl)
	}
}
