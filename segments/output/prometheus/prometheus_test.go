package prometheus

import (
	"testing"

	"codeberg.org/BelWue/flowpipeline/pb"
	"codeberg.org/BelWue/flowpipeline/segments"
)

// Prometheus Segment test, passthrough test only
func TestSegment_PrometheusExporter_passthrough(t *testing.T) {
	result := segments.TestSegment("prometheus", map[string]string{"endpoint": ":8080"},
		&pb.EnrichedFlow{})
	if result == nil {
		t.Error("([error] Segment Prometheus is not passing through flows.")
	}
}
