// The `basefiltersegment` serves as a basis for segments implementing the FilterSegment-interface.
// It extends the BaseSegment by adding an additional channel for dropped flows as well as the required access methods
package basefiltersegment

import (
	"codeberg.org/BelWue/flowpipeline/pb"
	"codeberg.org/BelWue/flowpipeline/segments/base/basesegment"
)

// An extended basis for Segment implementations in the filter group. It
// contains the necessities to process filtered (dropped) flows.
type BaseFilterSegment struct {
	basesegment.BaseSegment
	Drops chan<- *pb.EnrichedFlow
}

// Set a return channel for dropped flow messages. Segments need to be wary of
// this channel closing when producing messages to this channel. This method is
// only called by the flowpipeline tool from the controlflow/branch segment to
// implement the then/else branches, otherwise this functionality is unused.
func (segment *BaseFilterSegment) SubscribeDrops(drops chan<- *pb.EnrichedFlow) {
	segment.Drops = drops
}
