// The `downsample` segment samples every nth flow
// and adjusts the sampling rate to reflect the new "real" sampling rate
package normalize

import (
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"

	"codeberg.org/BelWue/flowpipeline/segments"
	"codeberg.org/BelWue/flowpipeline/segments/base/basefiltersegment"
)

type Downsample struct {
	basefiltersegment.BaseFilterSegment
	SamplingRate uint // defines the sampling rate by which flows should be sampled down
}

func (segment Downsample) New(config map[string]string) segments.Segment {
	if config["samplingrate"] == "" {
		log.Fatal().Msg("Downsample: Missing required parameter `samplingrate`")
		return nil
	}
	parsedSamplingRate, err := strconv.ParseUint(config["samplingrate"], 10, 32)
	if err != nil || parsedSamplingRate <= 0 {
		log.Fatal().Err(err).Msgf("Downsample: Failed to parse samplingrate %s", config["samplingrate"])
	}
	return &Downsample{
		SamplingRate: uint(parsedSamplingRate),
	}
}

func (segment *Downsample) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	counter := uint(1)
	for msg := range segment.In {
		if counter >= segment.SamplingRate {
			msg.SamplingRate *= uint64(segment.SamplingRate)
			segment.Out <- msg
			counter = uint(1)
		} else {
			segment.Drops <- msg
			counter++
		}
	}
}

func init() {
	segment := &Downsample{}
	segments.RegisterSegment("downsample", segment)
}
