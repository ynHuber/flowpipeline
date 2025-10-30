package aggregate

import (
	"sync"
	"testing"
	"time"

	"codeberg.org/BelWue/flowpipeline/pb"
	"codeberg.org/BelWue/flowpipeline/pipeline"
)

func TestInit(t *testing.T) {
	pipeline := pipeline.NewFromConfig([]byte(`---
- segment: aggregate
  config:
    activeTimeout: 50ms
    inactiveTimeout: 50ms
`))

	//Test if pipeline is setup correctly
	segment0 := pipeline.SegmentList[0]
	if a, ok := segment0.(*Aggregate); !ok ||
		(a.ActiveTimeout != 50*time.Millisecond) ||
		(a.InactiveTimeout != 50*time.Millisecond) {
		t.Fatal("[error] aggregate segment not initialized correctly from pipeline config")
	}

	flowIn := pb.EnrichedFlow{SrcAddr: []byte{10, 0, 0, 1}, Bytes: 1, Packets: 1}

	pipeline.Start()

	//test timeout setup
	start := time.Now()
	go func() {
		pipeline.In <- &flowIn
	}()
	select {
	case flowOut := <-pipeline.Out:
		earliestExpectedEndTime := start.Add(50 * time.Millisecond)
		if earliestExpectedEndTime.After(time.Now()) {
			t.Fatalf("Forwarded %s before timeout", time.Until(earliestExpectedEndTime).String())
		}
		if flowOut.SrcAddrObj().String() != flowIn.SrcAddrObj().String() || flowOut.Bytes != flowIn.Bytes {
			t.Fatal("forwarded flow mismatch")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timed out waiting for passthrough forwarded flow")
	}

}

func Test_Aggregate(t *testing.T) {
	pipeline := pipeline.NewFromConfig([]byte(`---
- segment: aggregate
  config:
    inactiveTimeout: 50ms
    activeTimeout: 160ms
`))

	flowIn1 := pb.EnrichedFlow{SrcAddr: []byte{10, 0, 0, 1}, Bytes: 1, Packets: 1, TimeFlowEnd: uint64(time.Now().Unix()), TimeReceived: uint64(time.Now().Unix())}

	pipeline.Start()
	//test timeout setup
	start := time.Now()

	//Add first flow 3 time right at the beginning -> inactive flow afterwards
	pipeline.In <- &flowIn1
	pipeline.In <- &flowIn1
	pipeline.In <- &flowIn1

	flowOut := <-pipeline.Out
	if flowOut.SrcAddrObj().String() == flowIn1.SrcAddrObj().String() {
		earliestExpectedEndTime := start.Add(50 * time.Millisecond)
		if earliestExpectedEndTime.After(time.Now()) {
			t.Fatalf("Inactive Timeout forwarded %s before timeout", time.Until(earliestExpectedEndTime).String())
		}
	} else {
		t.Fatal("forwarded flow mismatch")
	}

	//Add first flow 1 time right at the beginning and then every 25ms until the active timeout is reached
	pipeline.In <- &pb.EnrichedFlow{SrcAddr: []byte{10, 0, 0, 2}, Bytes: 1, Packets: 1, TimeFlowEnd: uint64(time.Now().Unix()), TimeReceived: uint64(time.Now().Unix())}
	flow2count := 1
	mutex := sync.RWMutex{}
	for {
		select {
		case flowOut := <-pipeline.Out:
			mutex.Lock()
			if flowOut.SrcAddrObj().String() == "10.0.0.2" {
				earliestExpectedEndTime := start.Add(160 * time.Millisecond)
				if earliestExpectedEndTime.After(time.Now()) {
					t.Fatalf("Active Timeout forwarded %s before timeout", time.Until(earliestExpectedEndTime).String())
				}
				if flowOut.Bytes != uint64(flow2count) || flowOut.Packets != uint64(flow2count) {
					t.Fatal("Bad aggregation")
				}
			} else {
				t.Fatal("forwarded flow mismatch")
			}
			mutex.Unlock()
			return
		case <-time.After(25 * time.Millisecond):
			mutex.Lock()
			pipeline.In <- &pb.EnrichedFlow{SrcAddr: []byte{10, 0, 0, 2}, Bytes: 1, Packets: 1, TimeFlowEnd: uint64(time.Now().Unix()), TimeReceived: uint64(time.Now().Unix())}
			flow2count++

			mutex.Unlock()
		}
	}
}
