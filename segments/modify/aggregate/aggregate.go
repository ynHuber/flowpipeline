// The `aggregate` segment aggregates flows with the same
// `SrcAddr`,`DstAddr`,`SrcPort`,`DstPort`,`Proto`,`IPTos`and`InIface`
// into one flow by running a local flowcache with configurable timeouts.
//
// Caution: The aggregation is done for all sampleAddresses replacing them with an empty address
package aggregate

import (
	"net"
	"sync"
	"time"

	"codeberg.org/BelWue/flowpipeline/segments"
	"codeberg.org/BelWue/flowpipeline/segments/base/basesegment"
	"github.com/rs/zerolog/log"
)

type Aggregate struct {
	basesegment.BaseSegment

	cache           *FlowExporter
	ActiveTimeout   time.Duration // optional (default=1m - Configures when aggregated flows are exported)
	InactiveTimeout time.Duration // optional (default=15s - Configures export timespan after which aggregate flows are exported if no new flows are added to it)
}

func (segment Aggregate) New(config map[string]string) segments.Segment {
	var (
		err error
	)
	if config["activeTimeout"] != "" {
		segment.ActiveTimeout, err = time.ParseDuration(config["activeTimeout"])
		if err != nil {
			log.Error().Msgf("Aggregate: Could not parse active timeout '%s' - using default 1m", config["activeTimeout"])
			segment.ActiveTimeout = time.Duration(1 * time.Minute)
		}
	} else {
		log.Info().Msg("Aggregate: No active timeout set - using default 1m")
		segment.ActiveTimeout = time.Duration(1 * time.Minute)
	}

	if config["inactiveTimeout"] != "" {
		segment.InactiveTimeout, err = time.ParseDuration(config["inactiveTimeout"])
		if err != nil {
			log.Error().Msgf("Aggregate: Could not parse inactive timeout '%s' - using default 15s", config["inactiveTimeout"])
			segment.ActiveTimeout = time.Duration(15 * time.Second)
		}
	} else {
		log.Info().Msg("Aggregate: No inactive timeout set - using default 15s")
		segment.InactiveTimeout = time.Duration(15 * time.Second)
	}

	return &Aggregate{
		ActiveTimeout:   segment.ActiveTimeout,
		InactiveTimeout: segment.InactiveTimeout,
	}
}

func (segment *Aggregate) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	segment.cache = NewFlowExporterWithTimeoutDurations(segment.ActiveTimeout, segment.InactiveTimeout)
	segment.cache.Start(net.IP([]byte{}), net.HardwareAddr{}) //TODO: rewrite to not use a custom sampler address
	for {
		select {
		case msg, ok := <-segment.In:
			if !ok {
				return
			}
			segment.cache.InsertFlow(msg)
		case msg, ok := <-segment.cache.Flows:
			if !ok {
				return
			}
			segment.Out <- msg
		}
	}
}

func init() {
	segment := &Aggregate{}
	segments.RegisterSegment("aggregate", segment)
}
