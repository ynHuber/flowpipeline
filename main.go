// The flowpipeline utility unifies all bwNetFlow functionality and
// provides configurable pipelines to process flows in any manner.
//
// The main entrypoint accepts command line flags to point to a configuration
// file and to establish the log level.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"plugin"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"codeberg.org/BelWue/flowpipeline/pipeline"

	_ "codeberg.org/BelWue/flowpipeline/segments/output/http"

	_ "codeberg.org/BelWue/flowpipeline/segments/controlflow/branch"

	_ "codeberg.org/BelWue/flowpipeline/segments/filter/drop"
	_ "codeberg.org/BelWue/flowpipeline/segments/filter/elephant"

	_ "codeberg.org/BelWue/flowpipeline/segments/filter/flowfilter"

	_ "codeberg.org/BelWue/flowpipeline/segments/input/bpf"
	_ "codeberg.org/BelWue/flowpipeline/segments/input/goflow"
	_ "codeberg.org/BelWue/flowpipeline/segments/input/kafkaconsumer"
	_ "codeberg.org/BelWue/flowpipeline/segments/input/packet"
	_ "codeberg.org/BelWue/flowpipeline/segments/input/replay"
	_ "codeberg.org/BelWue/flowpipeline/segments/input/stdin"
	_ "codeberg.org/BelWue/flowpipeline/segments/ungrouped/diskbuffer"

	_ "codeberg.org/BelWue/flowpipeline/segments/meta/delay_monitoring"

	_ "codeberg.org/BelWue/flowpipeline/segments/modify/addcid" //nolint:staticcheck // deprecated, use addnetid
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/addnetid"
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/addrstrings"
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/aggregate"
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/anonymize"
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/aslookup"
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/bgp"
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/dropfields"
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/geolocation"
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/normalize"
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/protomap"
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/remoteaddress"
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/reversedns"
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/snmp"
	_ "codeberg.org/BelWue/flowpipeline/segments/modify/sync_timestamps"

	_ "codeberg.org/BelWue/flowpipeline/segments/output/clickhouse"
	_ "codeberg.org/BelWue/flowpipeline/segments/output/csv"
	_ "codeberg.org/BelWue/flowpipeline/segments/output/influx"
	_ "codeberg.org/BelWue/flowpipeline/segments/output/json"
	_ "codeberg.org/BelWue/flowpipeline/segments/output/kafkaproducer"
	_ "codeberg.org/BelWue/flowpipeline/segments/output/lumberjack"
	_ "codeberg.org/BelWue/flowpipeline/segments/output/mongodb"
	_ "codeberg.org/BelWue/flowpipeline/segments/output/parquet"
	_ "codeberg.org/BelWue/flowpipeline/segments/output/prometheus"
	_ "codeberg.org/BelWue/flowpipeline/segments/output/sqlite"

	_ "codeberg.org/BelWue/flowpipeline/segments/print/count"
	_ "codeberg.org/BelWue/flowpipeline/segments/print/printdots"
	_ "codeberg.org/BelWue/flowpipeline/segments/print/printflowdump"
	_ "codeberg.org/BelWue/flowpipeline/segments/print/toptalkers"

	_ "codeberg.org/BelWue/flowpipeline/segments/analysis/toptalkersmetrics"
	_ "codeberg.org/BelWue/flowpipeline/segments/analysis/trafficspecifictoptalkers"
)

var Version string

type flagArray []string

func (i *flagArray) String() string {
	return strings.Join(*i, ",")
}

func (i *flagArray) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	var pluginPaths flagArray
	flag.Var(&pluginPaths, "p", "Path to load segment plugins from, can be specified multiple times")
	logLevel := flag.String("l", "warning", "Loglevel: one of 'debug', 'info', 'warning' or 'error'")
	concurrency := flag.Uint("n", 1, "Number of concurrent pipelines to spawn. Set to 0 to enable automatic setting according to GOMAXPROCS. Only the default value 1 guarantees a stable order of the flows in and out of flowpipeline.")
	version := flag.Bool("v", false, "print version")
	prettyLogging := flag.Bool("j", false, "Json log")
	configFile := flag.String("c", "config.yml", "location of the config file in yml format")
	flag.Parse()

	if *version {
		fmt.Println(Version)
		return
	}

	if !*prettyLogging {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime})
	}
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerologLogLevel(logLevel))

	for _, path := range pluginPaths {
		_, err := plugin.Open(path)
		if err != nil {
			if err.Error() == "plugin: not implemented" {
				log.Error().Msg("Loading plugins is unsupported when running a static, not CGO-enabled binary.")
			} else {
				log.Error().Err(err).Msgf("Problem loading the specified plugin '%s'", path)
			}
			return
		} else {
			log.Info().Msgf("Loaded plugin: %s", path)
		}
	}

	config, err := os.ReadFile(*configFile)
	if err != nil {
		log.Error().Err(err).Msg("Reading config file: ")
		return
	}

	var pipelineCount int
	if *concurrency == 0 {
		pipelineCount = runtime.GOMAXPROCS(0)
	} else {
		pipelineCount = int(*concurrency)
	}

	segmentReprs := pipeline.SegmentReprsFromConfig(config)
	for i := 0; i < pipelineCount; i++ {
		segments := pipeline.SegmentsFromRepr(segmentReprs)
		pipe := pipeline.New(segments...)
		if pipe == nil {
			log.Fatal().Msg("An error occured during pipeline initialization - Exiting")
			return
		}
		pipe.Start()
		pipe.AutoDrain()
		defer pipe.Close()
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGINT)
	signal.Notify(sigs, os.Interrupt, os.Interrupt)
	<-sigs
	log.Info().Msg("Received exit signal")
	go func() {
		<-time.After(time.Duration(15 * time.Second))
		log.Fatal().Msg("Failed to shut down gracefully - force quitting")
		os.Exit(5)
	}()
}

func zerologLogLevel(logLevel *string) zerolog.Level {
	if logLevel != nil && *logLevel != "" {
		switch *logLevel {
		case "trace":
			log.Info().Msg("Using log level 'trace'")
			return zerolog.TraceLevel
		case "debug":
			log.Info().Msg("Using log level 'debug'")
			return zerolog.DebugLevel
		case "info":
			log.Info().Msg("Using log level 'info'")
			return zerolog.InfoLevel
		case "warning":
			return zerolog.WarnLevel
		case "error":
			return zerolog.ErrorLevel
		case "fatal":
			return zerolog.FatalLevel
		case "panic":
			return zerolog.PanicLevel
		default:
			log.Warn().Msgf("Unknown log level '%s' using default 'info'", *logLevel)
		}
	} else {
		log.Info().Msg("Using default log level 'info'")
	}

	return zerolog.InfoLevel
}
