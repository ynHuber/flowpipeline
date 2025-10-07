// The `toptalkers_metrics` segment calculates statistics about traffic levels
// per IP address and exports them in OpenMetrics format via HTTP.
//
// Traffic is counted in bits per second and packets per second, categorized into
// forwarded and dropped traffic. By default, only the destination IP addresses
// are accounted, but the configuration allows using the source IP address,
// both addresses or the connection. For `both addresses`, a flows number of bytes and packets are
// counted for both addresses. `connection` is used to look a specific combinations
// of "source -> target". Note that watching connections or addresses outside of your network
// can lead to high RAM usage - especially during ddos attacks.
//
// Thresholds for bits per second or packets per second can be configured. Only
// metrics for addresses that exceeded this threshold during the last window size
// are exported. This can be used for detection of unusual or unwanted traffic
// levels. This can also be used as a flow filter: While the average traffic for
// an address is above threshold, flows are passed, other flows are dropped.
//
// The averages are calculated with a sliding window. The window size (in number
// of buckets) and the bucket duration can be configured. By default, it uses
// 60 buckets of 1 second each (1 minute of sliding window). Optionally, the
// window size for the exported metrics calculation and for the threshold check
// can be configured differently.
//
// The parameter "traffictype" is passed as OpenMetrics label, so this segment
// can be used multiple times in one pipeline without metrics getting mixed up.
package toptalkers_metrics

import (
	"sync"

	"codeberg.org/BelWue/flowpipeline/pipeline/config/evaluation_mode"
	"codeberg.org/BelWue/flowpipeline/segments"
	"github.com/rs/zerolog/log"
)

type ToptalkersMetrics struct {
	segments.BaseFilterSegment
	PrometheusMetricsParams
	PrometheusParams
	EvaluationMode evaluation_mode.EvaluationMode // optional, default is "destination", options are "destination", "source", "both", "connection"
}

func (segment ToptalkersMetrics) New(config map[string]string) segments.Segment {
	newsegment := &ToptalkersMetrics{}
	newsegment.InitDefaultPrometheusParams()
	newsegment.InitDefaultPrometheusMetricParams()

	err := newsegment.ParsePrometheusConfig(config)
	if err != nil {
		log.Error().Err(err).Msg("ToptalkersMetrics: Failed parsing prometheus config")
		return nil
	}
	if config["endpoint"] == "" {
		log.Info().Msg("ToptalkersMetrics: Missing configuration parameter 'endpoint'. Using default port ':8080'")
	} else {
		newsegment.Endpoint = config["endpoint"]
	}

	if config["metricspath"] == "" {
		log.Info().Msg("ToptalkersMetrics: Missing configuration parameter 'metricspath'. Using default path 'metrics'")
	} else {
		newsegment.MetricsPath = config["metricspath"]
	}
	if config["flowdatapath"] == "" {
		log.Info().Msg("ToptalkersMetrics: Missing configuration parameter 'flowdatapath'. Using default path 'flowdata'")
	} else {
		newsegment.FlowdataPath = config["flowdatapath"]
	}
	return newsegment
}

func (segment *ToptalkersMetrics) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	var promExporter = PrometheusExporter{}

	database := NewDatabase(segment.PrometheusMetricsParams, &promExporter, segment.EvaluationMode)
	promExporter.Initialize()
	collector := NewPrometheusCollector([]*ToptalkerDatabase{&database})
	promExporter.FlowReg.MustRegister(collector)
	promExporter.ServeEndpoints(&segment.PrometheusParams)

	go database.Clock()
	go database.Cleanup()

	for msg := range segment.In {
		promExporter.KafkaMessageCount.Inc()
		var keys [][]byte
		switch segment.EvaluationMode {
		case evaluation_mode.Source:
			keys = [][]byte{msg.SrcAddr}
		case evaluation_mode.Destination:
			keys = [][]byte{msg.DstAddr}
		case evaluation_mode.SourceAndDestination:
			keys = [][]byte{msg.SrcAddr, msg.DstAddr}
		case evaluation_mode.Connection:
			keys = [][]byte{append(msg.SrcAddr, msg.DstAddr...)}
		case evaluation_mode.Unknown:
			//default = Destination
			keys = [][]byte{msg.DstAddr}
		}
		forward := false
		for _, key := range keys {
			record := database.GetRecord(key)
			record.Append(msg)
			if record.AboveThreshold().Load() {
				forward = true
			}
		}
		if forward {
			segment.Out <- msg
		} else if segment.Drops != nil {
			segment.Drops <- msg
		}
	}
}

func init() {
	segment := &ToptalkersMetrics{}
	segments.RegisterSegment("toptalkers_metrics", segment)
}
