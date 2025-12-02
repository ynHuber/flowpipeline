package normalize

import (
	"sync"
	"testing"

	"codeberg.org/BelWue/flowpipeline/pb"
	"codeberg.org/BelWue/flowpipeline/pipeline"
)

// Normalize Segment test, in-flow SampleingRate test
func Test_Downsample_Segment(t *testing.T) {
	pipeline := pipeline.NewFromConfig([]byte(`---
    - segment: downsample
      config:
        samplingrate: 10`))

	//Test if pipeline is setup correctly
	segment0 := pipeline.SegmentList[0]
	in, out, drop := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	switch segment := segment0.(type) {
	case *Downsample:
		if segment.SamplingRate != 10 {
			t.Error("[error] Downsample not initialized correctly")
			t.Fail()
		}
		segment.Rewire(in, out)
		segment.SubscribeDrops(drop)
	default:
		t.Error("[error] pipeline not initialized correctly")
		t.Fail()
	}

	outs := 0
	drops := 0

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment0.Run(wg)

	for i := 0; i < 1000; i++ {
		in <- &pb.EnrichedFlow{Proto: 17, InIf: 1, OutIf: 1, DstAddr: []byte{192, 168, 88, 142}, Packets: 15000, Bytes: 3000000, SamplingRate: uint64(i)}
		select {
		case msg := <-out:
			if msg.SamplingRate != uint64(10*i) {
				t.Error("[error] Downsampling not adjusting sampling rate correctly")
			}
			outs++
		case <-drop:
			drops++
		}
	}
	if drops != 900 || outs != 100 {
		t.Error("[error] Downsampling not working correctly")
	}

	close(in)
	wg.Wait()
}
