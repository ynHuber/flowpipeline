package toptalkersmetrics

import (
	"testing"

	"codeberg.org/BelWue/flowpipeline/pipeline"
	"codeberg.org/BelWue/flowpipeline/pipeline/config/evaluation_mode"
)

func TestSegment_EvaluationMode_initialization_connection(t *testing.T) {
	tests := map[string]evaluation_mode.EvaluationMode{
		"":                       evaluation_mode.Destination, //default if not set
		"destination":            evaluation_mode.Destination,
		"source and destination": evaluation_mode.SourceAndDestination,
		"source":                 evaluation_mode.Source,
		"connection":             evaluation_mode.Connection,
	}

	for setting, evalMode := range tests {
		var config string
		if setting != "" {
			config = `---
- segment: toptalkersmetrics
  config:
    endpoint: ":8085"
    evaluationmode: "` + setting + `"
`
		} else {
			config = `---
- segment: toptalkersmetrics
  config:
    endpoint: ":8085"
`
		}
		print(config)
		pipeline := pipeline.NewFromConfig([]byte(config))

		//grab counter segment to validate the test result
		if seg, ok := pipeline.SegmentList[0].(*ToptalkersMetrics); ok {
			if seg.EvaluationMode != evalMode {
				t.Error("Evaluation mode " + setting + " not parsed correctly for segment toptalkersmetrics")
			}
		} else {
			t.Error("Segment toptalkersmetrics not initializing correctly")
		}
	}
}
