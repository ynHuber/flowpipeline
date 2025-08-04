// TODO: Compile this using:
// `go build -buildmode=plugin ./examples/plugin/configuration/plugin/swap-fields.go`
package main

import (
	"sync"

	"codeberg.org/BelWue/flowpipeline/segments"
	"codeberg.org/BelWue/flowpipeline/segments/base/basesegment"
)

type SwapFields struct {
	basesegment.BaseSegment
}

func (segment SwapFields) New(config map[string]string) segments.Segment {
	return &SwapFields{}
}

func (segment *SwapFields) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	for msg := range segment.In {
		if msg.FlowDirection == 0 { // Incoming
			msg.SrcAddr = msg.SamplerAddress // set Source as the point where the flow entered the network
		}
		if msg.FlowDirection == 1 { // Outgoing
			msg.DstAddr = msg.SamplerAddress // set Dest as the point where the flow left the network
		}
		segment.Out <- msg
	}
}

func init() {
	segment := &SwapFields{}
	segments.RegisterSegment("SwapFields", segment)
}
