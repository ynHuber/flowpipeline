// Dumps all incoming flow messages to a clickhouse database. The schema used
// is selected according to the preset parameter.
package clickhouse_segment

import (
	"database/sql"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwNetFlow/flowpipeline/pb"
	"github.com/bwNetFlow/flowpipeline/segments"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

type Clickhouse struct {
	segments.BaseSegment
	db              *sql.DB
	createStatement string
	insertStatement string

	DSN       string // required
	Preset    string // optional, what schema to use, currently only the option and default is "flowhouse"
	BatchSize int    // optional how many flows to hold in memory between INSERTs, default is 1000

	bulkInsert func(unsavedFlows []*pb.EnrichedFlow) error
}

// Every Segment must implement a New method, even if there isn't any config
// it is interested in.
func (segment Clickhouse) New(config map[string]string) segments.Segment {
	newsegment := &Clickhouse{}

	if config["dsn"] == "" {
		log.Println("[error] Clickhouse: Parameter 'dsn' is required.")
		return nil
	} else {
		newsegment.DSN = config["dsn"]
	}

	newsegment.BatchSize = 1000
	if config["batchsize"] != "" {
		if parsedBatchSize, err := strconv.ParseUint(config["batchsize"], 10, 32); err == nil {
			if parsedBatchSize == 0 {
				log.Println("[error] Clickhouse: Batch size 0 is not allowed. Set this in relation to the expected flows per second.")
				return nil
			}
			newsegment.BatchSize = int(parsedBatchSize)
		} else {
			log.Println("[error] Clickhouse: Could not parse 'batchsize' parameter, using default 1000.")
		}
	} else {
		log.Println("[info] Clickhouse: 'batchsize' set to default '1000'.")
	}

	// determine field set
	newsegment.Preset = strings.ToLower(config["preset"])
	switch newsegment.Preset {
	case "flowhouse":
		newsegment.createStatement = `CREATE TABLE IF NOT EXISTS flows (
			agent           IPv6,
			int_in          String,
			int_out         String,
			src_ip_addr     IPv6,
			dst_ip_addr     IPv6,
			src_ip_pfx_addr IPv6,
			src_ip_pfx_len  UInt8,
			dst_ip_pfx_addr IPv6,
			dst_ip_pfx_len  UInt8,
			nexthop         IPv6,
			next_asn        UInt32,
			src_asn         UInt32,
			dst_asn         UInt32,
			ip_protocol     UInt8,
			src_port        UInt16,
			dst_port        UInt16,
			timestamp       DateTime,
			size            UInt64,
			packets         UInt64,
			samplerate      UInt64
		) ENGINE = MergeTree()
		PARTITION BY toStartOfTenMinutes(timestamp)
		ORDER BY (timestamp)
		TTL timestamp + INTERVAL 14 DAY
		SETTINGS index_granularity = 8192`
		newsegment.insertStatement = `INSERT INTO flows (
			agent,
			int_in,
			int_out,
			src_ip_addr,
			dst_ip_addr,
			src_ip_pfx_addr,
			src_ip_pfx_len,
			dst_ip_pfx_addr,
			dst_ip_pfx_len,
			nexthop,
			next_asn,
			src_asn,
			dst_asn,
			ip_protocol,
			src_port,
			dst_port,
			timestamp,
			size,
			packets,
			samplerate
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? , ?, ?, ?)`
		newsegment.bulkInsert = newsegment.bulkInsertFlowhouse
	default:
		log.Printf("[error] Clickhouse: Unknown preset selected.")
		return nil
	}

	return newsegment
}

func (segment *Clickhouse) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	var err error
	segment.db, err = sql.Open("clickhouse", segment.DSN)
	if err != nil {
		log.Panicf("[error] Clickhouse: Could not open database with error: %+v", err)
	}
	defer segment.db.Close()

	tx, err := segment.db.Begin()
	if err != nil {
		log.Panicf("[error] Clickhouse: Could not start initiation transaction with error: %+v", err)
	}
	_, err = tx.Exec(segment.createStatement)
	if err != nil {
		log.Panicf("[error] Clickhouse: Could not create database, check field configuration: %+v", err)
	}
	tx.Commit()

	var unsaved []*pb.EnrichedFlow

	for msg := range segment.In {
		unsaved = append(unsaved, msg)
		if len(unsaved) >= segment.BatchSize {
			err := segment.bulkInsert(unsaved)
			if err != nil {
				log.Printf("[error] %s", err)
			}
			unsaved = []*pb.EnrichedFlow{}
		}
		segment.Out <- msg
	}
	segment.bulkInsert(unsaved)
}

func (segment Clickhouse) bulkInsertFlowhouse(unsavedFlows []*pb.EnrichedFlow) error {
	if len(unsavedFlows) == 0 {
		return nil
	}
	tx, err := segment.db.Begin()
	if err != nil {
		log.Printf("[error] Clickhouse: Error starting transaction for current batch of %d flows: %+v", len(unsavedFlows), err)
	}
	for _, msg := range unsavedFlows {
		var srcPfx, dstPfx net.IP
		if msg.IsIPv6() {
			srcPfx = net.IPNet{IP: net.IP(msg.SrcAddr), Mask: net.CIDRMask(int(msg.SrcNet), int(32-msg.SrcNet))}.IP
			dstPfx = net.IPNet{IP: net.IP(msg.DstAddr), Mask: net.CIDRMask(int(msg.DstNet), int(32-msg.DstNet))}.IP
		} else {
			srcPfx = net.IPNet{IP: net.IP(msg.SrcAddr), Mask: net.CIDRMask(int(msg.SrcNet), int(128-msg.SrcNet))}.IP
			dstPfx = net.IPNet{IP: net.IP(msg.DstAddr), Mask: net.CIDRMask(int(msg.DstNet), int(128-msg.DstNet))}.IP
		}
		valueArgs := []any{
			net.IP(msg.SamplerAddress),
			msg.SrcIfDesc,
			msg.DstIfDesc,
			net.IP(msg.SrcAddr),
			net.IP(msg.DstAddr),
			srcPfx,
			uint8(msg.SrcNet),
			dstPfx,
			uint8(msg.DstNet),
			net.IP(msg.NextHop),
			msg.NextHopAs,
			msg.SrcAs,
			msg.DstAs,
			uint8(msg.Proto),
			uint16(msg.SrcPort),
			uint16(msg.DstPort),
			time.Now(),
			msg.Bytes,
			msg.Packets,
			msg.SamplingRate,
		}
		_, err := tx.Exec(segment.insertStatement, valueArgs...)
		if err != nil {
			log.Printf("[error] Clickhouse: Error inserting flow into transaction: %+v", err)
		}
	}
	tx.Commit()
	return nil
}

func init() {
	segment := &Clickhouse{}
	segments.RegisterSegment("clickhouse", segment)
}
