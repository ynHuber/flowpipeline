package segments

import (
	"strings"
	"sync"
	"testing"

	"codeberg.org/BelWue/flowpipeline/pb"
)

// creating a local test segment since no other segments can be imported here without cyclic dependencies
type TestingSegment struct {
	BaseSegment
	Counter int
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
	segmentName := "testingsegment"

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
	segmentName := "testingsegment"
	segmentDeprecatedName := "deprecatedtestingsegment"

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
	parallelSegmentName := "p"

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
	if !((s1.Counter == 1 && s2.Counter == 0) || (s1.Counter == 0 && s2.Counter == 1)) {
		t.Error("Message should only be processed by one subsegment of the parallelized segment")
	}
	//at this point the segment processing the first input should still be busy, since the message wasn't yet retrieved from its output channel
	in <- msg
	<-out
	<-out
	if !(s1.Counter == 1 && s2.Counter == 1) {
		t.Error("Each subsegment should have processed a message at this point")
	}
}
