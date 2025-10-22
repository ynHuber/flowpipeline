// The `trafficspecifictoptalkers` segement is simmilar to the `toptalker-metrics` segment,
// but allows filtering for specific protocols. The use of nested filters is supported to
// allow for a more efficient filtering.
//
// Filters with a specified `traffictyp` will be exported if they reach the configured thresholds.
// The traffic specific toptalker metrics segement is simmilar to the `toptalker-metrics` segment,
// but allows filtering for specific protocols. The use of nested filters is supported to
// allow for a more efficient filtering.

// Filters with a specified `traffictyp` will be exported if they reach the configured thresholds.
// The segment allows forwarding all traffic to a matched ip to a subpipeline.
// See the [example configuration](https://codeberg.org/BelWue/flowpipeline/src/branch/main/examples/configurations/analysis/traffic-specific-toptalker.yml)
// for an example using that functionality
package trafficspecifictoptalkers

import (
	"sync"

	"github.com/rs/zerolog/log"

	"codeberg.org/BelWue/flowpipeline/pb"
	"codeberg.org/BelWue/flowpipeline/pipeline"
	"codeberg.org/BelWue/flowpipeline/pipeline/config"
	"codeberg.org/BelWue/flowpipeline/pipeline/config/evaluation_mode"
	"codeberg.org/BelWue/flowpipeline/segments"
	"codeberg.org/BelWue/flowpipeline/segments/analysis/toptalkersmetrics"
	"codeberg.org/BelWue/flowpipeline/segments/base/basesegment"
	"codeberg.org/BelWue/flowpipeline/segments/filter/flowfilter"
	"github.com/BelWue/flowfilter/parser"
)

type TrafficSpecificToptalkers struct {
	basesegment.BaseSegment
	toptalkersmetrics.PrometheusParams
	ThresholdMetricDefinition []*ThresholdMetric
	EvaluationMode            evaluation_mode.EvaluationMode // optional, default is "destination", options are "destination", "source", "both", "connection"
	MatchingPipeline          *pipeline.Pipeline
}

type ThresholdMetric struct {
	toptalkersmetrics.PrometheusMetricsParams

	Database         *toptalkersmetrics.ToptalkerDatabase
	SubDefinitions   []*ThresholdMetric
	Expression       *parser.Expression
	FilterDefinition string
}

func (segment TrafficSpecificToptalkers) New(config map[string]string) segments.Segment {
	newSegment := &TrafficSpecificToptalkers{}
	newSegment.InitDefaultPrometheusParams()
	if config["endpoint"] == "" {
		log.Info().Msg("ToptalkersMetrics: Missing configuration parameter 'endpoint'. Using default port \":8080\"")
	} else {
		newSegment.Endpoint = config["endpoint"]
	}

	if config["metricspath"] == "" {
		log.Info().Msg("ToptalkersMetrics: Missing configuration parameter 'metricspath'. Using default path \"/metrics\"")
	} else {
		newSegment.FlowdataPath = config["metricspath"]
	}
	if config["flowdatapath"] == "" {
		log.Info().Msg("ThresholdToptalkersMetrics: Missing configuration parameter 'flowdatapath'. Using default path \"/flowdata\"")
	} else {
		newSegment.FlowdataPath = config["flowdatapath"]
	}
	if config["evaluationmode"] == "" && config["relevantaddress"] != "" {
		log.Warn().Msg("ThresholdToptalkersMetrics: Using deprecated parameter 'relevantaddress' - please use evaluationmode instead")
		config["evaluationmode"] = config["relevantaddress"]
	}

	if config["evaluationmode"] == "" {
		log.Info().Msg("ThresholdToptalkersMetrics: 'evaluationmode' set to default 'destination'.")
		newSegment.EvaluationMode = evaluation_mode.Destination
	} else {
		if config["evaluationmode"] == "both" {
			log.Warn().Msg("ThresholdToptalkersMetrics: using depected evaluation mode 'both' - please use 'Source and Destination' instead")
		}
		evaluationMode := evaluation_mode.ParseEvaluationMode(config["evaluationmode"])

		if evaluationMode == evaluation_mode.Unknown {
			log.Error().Msg("ThresholdToptalkersMetrics: Could not parse 'evaluationmode', using default value 'destination'.")
			evaluationMode = evaluation_mode.Destination
		}
		newSegment.EvaluationMode = evaluationMode

	}

	return newSegment
}

func (segment *TrafficSpecificToptalkers) AddCustomConfig(segmentReprs config.SegmentRepr) {
	for _, definition := range segmentReprs.Config.ThresholdMetricDefinition {
		metric, err := segment.metricFromDefinition(definition)
		if err != nil {
			log.Error().Err(err).Msg("ThresholdToptalkersMetrics: Failed to add custom config")
		}
		segment.ThresholdMetricDefinition = append(segment.ThresholdMetricDefinition, metric)
	}
	if segmentReprs.MatchingPipeline != nil {
		segments := pipeline.SegmentsFromRepr(segmentReprs.MatchingPipeline)
		segment.MatchingPipeline = pipeline.New(segments...)
	}
}

func (segment *TrafficSpecificToptalkers) metricFromDefinition(definition *config.ThresholdMetricConfig) (*ThresholdMetric, error) {
	var err error
	metric := ThresholdMetric{}
	metric.PrometheusMetricsParamsDefinition = definition.PrometheusMetricsParamsDefinition
	metric.FilterDefinition = definition.FilterDefinition
	metric.InitDefaultPrometheusMetricParams()

	metric.Expression, err = parser.Parse(definition.FilterDefinition)
	if err != nil {
		log.Error().Err(err).Msgf("ThresholdToptalkersMetrics: Syntax error in filter expression\"%s\"", definition.FilterDefinition)
		return nil, err
	}

	for _, subDefinition := range definition.SubDefinitions {
		subMetric, err := segment.metricFromDefinition(subDefinition)
		if err != nil {
			return nil, err
		}
		metric.SubDefinitions = append(metric.SubDefinitions, subMetric)
	}

	return &metric, nil
}

func (segment *TrafficSpecificToptalkers) Run(wg *sync.WaitGroup) {
	var allDatabases *[]*toptalkersmetrics.ToptalkerDatabase
	defer func() {
		if segment.MatchingPipeline != nil {
			segment.MatchingPipeline.Close()
		}
		close(segment.Out)
		for _, db := range *allDatabases {
			db.StopTimers()
		}
		wg.Done()
	}()
	if segment.MatchingPipeline != nil {
		segment.MatchingPipeline.AutoDrain()
		segment.MatchingPipeline.Start()
	}

	var promExporter = toptalkersmetrics.PrometheusExporter{}
	promExporter.Initialize()

	allDatabases = segment.initDatabasesAndCollector(promExporter)

	//start timers
	promExporter.ServeEndpoints(&segment.PrometheusParams)
	for _, db := range *allDatabases {
		go db.Clock()
		go db.Cleanup()
	}

	filter := &flowfilter.Filter{}
	log.Info().Msgf("Threshold Metric Report runing on %s", segment.Endpoint)
	if segment.MatchingPipeline != nil {
		for msg := range segment.In {
			promExporter.KafkaMessageCount.Inc()
			for _, filterDef := range segment.ThresholdMetricDefinition {
				segment.addMessageToMatchingToptalkers(msg, filterDef, filter)
			}
			if segment.MatchingPipeline != nil && segment.IpInToptalkers(msg, filter) {
				segment.MatchingPipeline.In <- msg
			}
			segment.Out <- msg
		}
	} else {
		for msg := range segment.In {
			promExporter.KafkaMessageCount.Inc()
			for _, filterDef := range segment.ThresholdMetricDefinition {
				segment.addMessageToMatchingToptalkers(msg, filterDef, filter)
			}
			segment.Out <- msg
		}
	}
}

func (segment *TrafficSpecificToptalkers) IpInToptalkers(msg *pb.EnrichedFlow, filter *flowfilter.Filter) bool {
	for _, metricDef := range segment.ThresholdMetricDefinition {
		if segment.IpInToptalkersOfMetric(metricDef, msg, filter) {
			return true
		}
	}
	return false
}

func (segment *TrafficSpecificToptalkers) IpInToptalkersOfMetric(metricDef *ThresholdMetric, msg *pb.EnrichedFlow, filter *flowfilter.Filter) bool {
	if metricDef.TrafficType != "" {

		var keys [][]byte
		switch segment.EvaluationMode {
		case evaluation_mode.Source:
			keys = [][]byte{msg.SrcAddrObj().To16()}
		case evaluation_mode.Destination:
			keys = [][]byte{msg.DstAddrObj().To16()}
		case evaluation_mode.SourceAndDestination:
			keys = [][]byte{msg.SrcAddrObj().To16(), msg.DstAddrObj().To16()}
		case evaluation_mode.Connection:
			keys = [][]byte{append(msg.SrcAddrObj().To16(), msg.DstAddrObj().To16()...)}
		case evaluation_mode.Unknown:
			//default = Destination
			keys = [][]byte{msg.DstAddrObj().To16()}
		}

		for _, key := range keys {
			record := metricDef.Database.GetTypedRecord(metricDef.TrafficType, key)
			if record.AboveThreshold().Load() {
				return true
			}
		}
	}

	//also check subfilters
	for _, subdefinition := range metricDef.SubDefinitions {
		if segment.IpInToptalkersOfMetric(subdefinition, msg, filter) {
			return true
		}
	}
	return false
}

func (segment *TrafficSpecificToptalkers) initDatabasesAndCollector(promExporter toptalkersmetrics.PrometheusExporter) *[]*toptalkersmetrics.ToptalkerDatabase {
	allDatabases := []*toptalkersmetrics.ToptalkerDatabase{}
	for _, filterDef := range segment.ThresholdMetricDefinition {
		databases := segment.initDatabasesForFilter(filterDef, &promExporter)
		allDatabases = append(allDatabases, databases...)
	}

	collector := toptalkersmetrics.NewPrometheusCollector(allDatabases)
	promExporter.FlowReg.MustRegister(collector)
	return &allDatabases
}

func (segment *TrafficSpecificToptalkers) initDatabasesForFilter(filterDef *ThresholdMetric, promExporter *toptalkersmetrics.PrometheusExporter) []*toptalkersmetrics.ToptalkerDatabase {
	databases := []*toptalkersmetrics.ToptalkerDatabase{}
	if filterDef.TrafficType != "" { //defined a metric that should be in prometheus
		database := toptalkersmetrics.NewDatabase(filterDef.PrometheusMetricsParams, promExporter, segment.EvaluationMode)

		filterDef.Database = &database
		databases = append(databases, &database)
	}
	for _, subDef := range filterDef.SubDefinitions {
		databases = append(databases, segment.initDatabasesForFilter(subDef, promExporter)...)
	}
	return databases
}

func (s *TrafficSpecificToptalkers) addMessageToMatchingToptalkers(msg *pb.EnrichedFlow, definition *ThresholdMetric, filter *flowfilter.Filter) {
	if match, _ := filter.CheckFlow(definition.Expression, msg); match {
		// Update Counters if definition has a prometheus label defined
		if definition.TrafficType != "" {
			var keys []([]byte)
			switch s.EvaluationMode {
			case evaluation_mode.Source:
				keys = [][]byte{msg.SrcAddrObj().To16()}
			case evaluation_mode.Destination:
				keys = [][]byte{msg.DstAddrObj().To16()}
			case evaluation_mode.SourceAndDestination:
				keys = [][]byte{msg.SrcAddrObj().To16(), msg.DstAddrObj().To16()}
			case evaluation_mode.Connection:
				keys = [][]byte{append(msg.SrcAddrObj().To16(), msg.DstAddrObj().To16()...)}
			case evaluation_mode.Unknown:
				//default = Destination
				keys = [][]byte{msg.DstAddrObj().To16()}
			}
			for _, key := range keys {
				record := definition.Database.GetTypedRecord(definition.TrafficType, key)
				record.Append(msg)
			}
		}

		//also check subfilters
		for _, subdefinition := range definition.SubDefinitions {
			s.addMessageToMatchingToptalkers(msg, subdefinition, filter)
		}
	}
}

func init() {
	segmentName := "trafficspecifictoptalkers"
	deprecatedSegmentName := "traffic_specific_toptalkers"

	segment := &TrafficSpecificToptalkers{}
	segments.RegisterSegment(segmentName, segment)

	deprecatedSegment := segments.CreateSegmentDeprecationWrapper(segment, deprecatedSegmentName, segmentName)
	segments.RegisterSegment(deprecatedSegmentName, deprecatedSegment)
}
