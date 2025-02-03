package pass

import (
	"io/ioutil"
	"log"
	"os"
	"sync"
	"testing"

	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
)

// Pass Segment test, passthrough test
func TestSegment_Pass(t *testing.T) {
	result := segments.TestSegment("pass", map[string]string{},
		&pb.EnrichedFlow{Type: 3})
	if result.Type != 3 {
		t.Error("Segment Pass is not working.")
	}
}

// Pass Segment benchmark passthrough
func BenchmarkPass(b *testing.B) {
	log.SetOutput(ioutil.Discard)
	os.Stdout, _ = os.Open(os.DevNull)

	segment := Pass{}

	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	segment.Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go segment.Run(wg)

	for n := 0; n < b.N; n++ {
		in <- &pb.EnrichedFlow{}
		_ = <-out
	}
	close(in)
}
