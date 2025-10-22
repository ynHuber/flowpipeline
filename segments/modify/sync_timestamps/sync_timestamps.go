// The segment `sync_timestamps` tries to fill empty time fields using existing ones.
// It works on the following fields:
// - TimeFlowStart:
//   - TimeFlowStart
//   - TimeFlowStartMs
//   - TimeFlowStartNs
//
// - TimeFlowEnd:
//   - TimeFlowEnd
//   - TimeFlowEndMs
//   - TimeFlowEndNs
//
// - TimeReceived:
//   - TimeReceived
//   - TimeReceivedNs
package sync_timestamps

import (
	"sync"

	"codeberg.org/BelWue/flowpipeline/segments"
	"codeberg.org/BelWue/flowpipeline/segments/base/basesegment"
)

type SyncTimestamps struct {
	basesegment.BaseSegment
}

func (segment SyncTimestamps) New(config map[string]string) segments.Segment {
	return &SyncTimestamps{}
}

func (segment *SyncTimestamps) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	for msg := range segment.In {
		msg.SyncMissingTimeStamps()
		segment.Out <- msg
	}
}

func init() {
	segmentName := "synctimestamps"
	deprecatedSegmentName := "sync_timestamps"

	segment := &SyncTimestamps{}
	segments.RegisterSegment(segmentName, segment)

	deprecatedSegment := segments.CreateSegmentDeprecationWrapper(segment, deprecatedSegmentName, segmentName)
	segments.RegisterSegment(deprecatedSegmentName, deprecatedSegment)
}
