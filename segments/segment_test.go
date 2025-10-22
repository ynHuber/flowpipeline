package segments

import (
	"strings"
	"sync"
	"syscall"
	"testing"

	"codeberg.org/BelWue/flowpipeline/pb"
	"codeberg.org/BelWue/flowpipeline/pipeline/config"
)

// creating a local test segment since no other segments can be imported here without cyclic dependencies
type TestingSegment struct {
	Counter int
	In      <-chan *pb.EnrichedFlow
	Out     chan<- *pb.EnrichedFlow
}

func (segment *TestingSegment) Rewire(in chan *pb.EnrichedFlow, out chan *pb.EnrichedFlow) {
	segment.In = in
	segment.Out = out
}

func (segment *TestingSegment) ShutdownParentPipeline() {
	syscall.Kill(syscall.Getpid(), syscall.SIGINT)
}

func (segment *TestingSegment) Close() {
	//placeholder since this segment doesn't need to do anything
}

func (segment *TestingSegment) AddCustomConfig(config.SegmentRepr) {
	//placeholder since this segment doesn't have a custom structured config
}

func (segment TestingSegment) New(config map[string]string) Segment {
	return &TestingSegment{
		Counter: 0,
	}
}

func (segment *TestingSegment) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()
	for msg := range segment.In {
		segment.Counter++
		segment.Out <- msg
	}
}

func TestSegmentRegistration(t *testing.T) {
	s := &TestingSegment{}
	segmentName := "testingsegment_TestSegmentRegistration"

	RegisterSegment(segmentName, s)
	foundSegment := LookupSegment(segmentName)
	if foundSegment != s {
		t.Error("Segment not found")
	}

	foundSegment = LookupSegment(strings.ToUpper(segmentName))
	if foundSegment != s {
		t.Error("Segmentlookup should be case-insensitive")
	}
}

func TestDeprecationwrapper(t *testing.T) {
	msg := &pb.EnrichedFlow{}

	s := &TestingSegment{
		Counter: 0,
	}
	segmentName := "testingsegment_TestDeprecationwrapper"
	segmentDeprecatedName := "deprecatedtestingsegment_TestDeprecationwrapper"

	d := CreateSegmentDeprecationWrapper(s, segmentName, segmentDeprecatedName)

	RegisterSegment(segmentName, s)
	RegisterSegment(segmentDeprecatedName, d)

	//s.Counter == 0
	//Check if the original segment works as expected
	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	LookupSegment(segmentName).Rewire(in, out)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go LookupSegment(segmentName).Run(wg)

	in <- msg
	<-out
	if s.Counter != 1 {
		t.Error("TestingSegment not counting package")
	}

	//counter == 1
	//Check if deprecated segment is correctly forwarding everything to the original segment
	LookupSegment(segmentDeprecatedName).Rewire(in, out)
	wg.Add(1)
	go LookupSegment(segmentDeprecatedName).Run(wg)

	in <- msg
	<-out
	if s.Counter != 2 {
		t.Error("TestingSegment not counting package send to deprecation wrapper")
	}
}

func TestParallelizedSegment(t *testing.T) {
	msg := &pb.EnrichedFlow{}
	parallelSegmentName := "p_TestParallelizedSegment"

	s1 := &TestingSegment{
		Counter: 0,
	}
	s2 := &TestingSegment{
		Counter: 0,
	}

	p := &ParallelizedSegment{}
	p.AddSegment(s1)
	p.AddSegment(s2)

	RegisterSegment(parallelSegmentName, p)
	in, out := make(chan *pb.EnrichedFlow), make(chan *pb.EnrichedFlow)
	LookupSegment(parallelSegmentName).Rewire(in, out)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go LookupSegment(parallelSegmentName).Run(wg)

	in <- msg
	if !((s1.Counter == 1 && s2.Counter == 0) || (s1.Counter == 0 && s2.Counter == 1)) { //nolint:staticcheck // ignoring De Morgan's law on purpose for readability
		t.Error("Message should only be processed by one subsegment of the parallelized segment")
	}
	//at this point the segment processing the first input should still be busy, since the message wasn't yet retrieved from its output channel
	in <- msg
	<-out
	<-out
	if s1.Counter != 1 || s2.Counter != 1 {
		t.Error("Each subsegment should have processed a message at this point")
	}
}
