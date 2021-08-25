package normalize

import (
	"log"
	"os"
	"testing"

	"github.com/bwNetFlow/flowpipeline/segments"
	flow "github.com/bwNetFlow/protobuf/go"
	"github.com/hashicorp/logutils"
)

func TestMain(m *testing.M) {
	log.SetOutput(&logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"info", "warning", "error"},
		MinLevel: logutils.LogLevel("info"),
		Writer:   os.Stderr,
	})
	code := m.Run()
	os.Exit(code)
}

// Normalize Segment test, in-flow SampleingRate test
func TestSegment_Normalize_inFlowSamplingRate(t *testing.T) {
	result := segments.TestSegment("normalize", map[string]string{},
		&flow.FlowMessage{SamplingRate: 32, Bytes: 1})
	if result.Bytes != 32 {
		t.Error("Segment Normalize is not working with in-flow SamplingRate.")
	}
}

// Normalize Segment test, fallback SampleingRate test
func TestSegment_Normalize_fallbackSamplingRate(t *testing.T) {
	result := segments.TestSegment("normalize", map[string]string{"fallback": "42"},
		&flow.FlowMessage{SamplingRate: 0, Bytes: 1})
	if result.Bytes != 42 {
		t.Error("Segment Normalize is not working with fallback SamplingRate.")
	}
}

// Normalize Segment test, no fallback SampleingRate test
func TestSegment_Normalize_noFallbackSamplingRate(t *testing.T) {
	result := segments.TestSegment("normalize", map[string]string{},
		&flow.FlowMessage{SamplingRate: 0, Bytes: 1})
	if result.Bytes != 1 {
		t.Error("Segment Normalize is not working with fallback SamplingRate.")
	}
}
