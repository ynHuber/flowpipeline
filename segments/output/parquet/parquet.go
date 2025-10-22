package parquet

import (
	"sync"

	"codeberg.org/BelWue/flowpipeline/segments"
	"codeberg.org/BelWue/flowpipeline/segments/base/basetextoutputsegment"
	"github.com/rs/zerolog/log"
)

type Parquet struct {
	basetextoutputsegment.BaseTextOutputSegment
}

func (segment Parquet) New(config map[string]string) segments.Segment {
	newsegment := &Parquet{}

	_, err := segment.GetOutput(config)
	if err != nil {
		log.Error().Err(err).Msg("Parquet: File specified in 'filename' is not accessible")
		return nil
	}

	return newsegment
}

func (segment *Parquet) Run(wg *sync.WaitGroup) {
}

func init() {
	segment := &Parquet{}
	segments.RegisterSegment("parquet", segment)
}
