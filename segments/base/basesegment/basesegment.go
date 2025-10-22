// The `basesegment` serves as a basis for any segment implementations. Segments embedding this
// base segment only need the New and the Run methods to be compliant to the segment
// interface.
package basesegment

import (
	"syscall"

	"codeberg.org/BelWue/flowpipeline/pb"
	"codeberg.org/BelWue/flowpipeline/pipeline/config"
)

type BaseSegment struct {
	In  <-chan *pb.EnrichedFlow
	Out chan<- *pb.EnrichedFlow
}

// This function rewires this Segment with the provided channels. This is
// typically called only by pipeline.New() and present in any Segment
// implementation embedding the BaseSegment.
// The peculiar implementation of passing the full channel list and providing
// indexes is due to the fact that controlflow segments may want to skip
// segments and thus need to have all later references available as well.
func (segment *BaseSegment) Rewire(in chan *pb.EnrichedFlow, out chan *pb.EnrichedFlow) {
	segment.In = in
	segment.Out = out
}

// This functions shutdown Parent Pipeline segments on the given syscall.
// It is used for intended termination within pipeline function, e.g. end pipeline on read from file.
func (segment *BaseSegment) ShutdownParentPipeline() {
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}

func (segment *BaseSegment) Close() {
	//placeholder since most segments dont need to do anything
}

func (segment *BaseSegment) AddCustomConfig(config.SegmentRepr) {
	//placeholder since most segments dont have a custom sturctured config
}
