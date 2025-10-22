package toptalkersmetrics

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/BelWue/flowpipeline/pb"

	e "codeberg.org/BelWue/flowpipeline/pipeline/config/evaluation_mode"
)

type Record interface {
	Append(*pb.EnrichedFlow)
	GetMetrics(buckets int, bucketDuration int) (float64, float64, float64, float64, string)
	AboveThreshold() *atomic.Bool
	tick(thresholdBuckets int, bucketDuration int, thresholdBps uint64, thresholdPps uint64)
	isEmpty() bool
}

type DefaultRecord struct {
	sync.RWMutex

	FwdBytes             []uint64
	FwdPackets           []uint64
	DropBytes            []uint64
	DropPackets          []uint64
	capacity             int
	pointer              int
	AboveThresholdAtomic atomic.Bool
	Display              string
}

type TwoWayRecord struct {
	DefaultRecord
	SrcAddr string
	DstAddr string
}

type ToptalkerDatabase struct {
	sync.RWMutex

	entries            ToptalkerDatabaseEntries
	TrafficType        string
	thresholdBps       uint64
	thresholdPps       uint64
	buckets            int
	BucketDuration     int // seconds
	ReportBuckets      int
	thresholdBuckets   int
	cleanupCounter     int
	cleanupWindowSizes int
	promExporter       *PrometheusExporter
	evaluationMode     e.EvaluationMode
	stopCleanupC       chan struct{}
	stopClockC         chan struct{}
}

type ToptalkerDatabaseEntries interface {
	getTypedRecord(key []byte, trafficType string) (Record, bool)
	upsertRecord(key []byte, value Record, trafficType string)
	getAllEntries() <-chan Record
	cleanup()
	count() int
	init()
}

type SingleIpEntries struct {
	data *map[string]*map[[16]byte]Record
}

type DoubleIpEntries struct {
	data *map[string]*map[[32]byte]Record
}

func (d *SingleIpEntries) getTypedRecord(key []byte, trafficType string) (Record, bool) {
	var key16 [16]byte
	copy(key16[:], key)
	return getTypedRecord(d.data, key16, trafficType)
}

func (d *DoubleIpEntries) getTypedRecord(key []byte, trafficType string) (Record, bool) {
	var key32 [32]byte
	copy(key32[:], key)
	return getTypedRecord(d.data, key32, trafficType)
}

func getTypedRecord[IntType comparable](entryMaps *map[string]*map[IntType]Record, uintkey IntType, trafficType string) (Record, bool) {
	if entryMaps == nil {
		return nil, false
	}
	trafficMap := (*entryMaps)[trafficType]
	if trafficMap == nil {
		return nil, false
	}
	record, found := (*trafficMap)[uintkey]
	return record, found
}

func (d *SingleIpEntries) upsertRecord(key []byte, value Record, trafficType string) {
	var key16 [16]byte
	copy(key16[:], key)
	upsertRecord(d.data, key16, value, trafficType)
}

func (d *DoubleIpEntries) upsertRecord(key []byte, value Record, trafficType string) {
	var key32 [32]byte
	copy(key32[:], key)
	upsertRecord(d.data, key32, value, trafficType)
}

func upsertRecord[IntType comparable](entryMaps *map[string]*map[IntType]Record, uintkey IntType, value Record, trafficType string) {
	trafficMap := (*entryMaps)[trafficType]
	if trafficMap == nil {
		trafficMap = &map[IntType]Record{}
		(*entryMaps)[trafficType] = trafficMap
	}
	(*trafficMap)[uintkey] = value
}

func (d *SingleIpEntries) count() int {
	return count(d.data)
}

func (d *DoubleIpEntries) count() int {
	return count(d.data)
}

func count[IntType comparable](recordMaps *map[string]*map[IntType]Record) int {
	entryCount := 0
	for _, recordMap := range *recordMaps {
		entryCount += len(*recordMap)
	}
	return entryCount
}

func (d *SingleIpEntries) cleanup() {
	cleanup(d.data)
}

func (d *DoubleIpEntries) cleanup() {
	cleanup(d.data)
}

func (d *SingleIpEntries) init() {
	d.data = &map[string]*map[[16]byte]Record{}
}

func (d *DoubleIpEntries) init() {
	d.data = &map[string]*map[[32]byte]Record{}
}

func cleanup[IntType comparable](recordMaps *map[string]*map[IntType]Record) {
	for _, recordMap := range *recordMaps {
		go cleanupRecordMap(recordMap)
	}
}

func (d *SingleIpEntries) getAllEntries() <-chan Record {
	return getAllEntries(d.data)
}

func (d *DoubleIpEntries) getAllEntries() <-chan Record {
	return getAllEntries(d.data)
}

func getAllEntries[IntType comparable](recordMaps *map[string]*map[IntType]Record) <-chan Record {
	out := make(chan Record)
	go func() {
		defer func() {
			close(out)
		}()
		if recordMaps == nil {
			return
		}
		for _, trafficMap := range *recordMaps {
			if trafficMap != nil {
				for _, record := range *trafficMap {
					out <- record
				}
			}
		}
	}()
	return out
}

func cleanupRecordMap[K comparable](recordMap *map[K]Record) {
	if recordMap != nil {
		for key, record := range *recordMap {
			if record.isEmpty() {
				delete(*recordMap, key)
			}
		}
	}
}

func NewDatabase(params PrometheusMetricsParams, promExporter *PrometheusExporter, evaluationMode e.EvaluationMode) ToptalkerDatabase {
	var entries ToptalkerDatabaseEntries
	if evaluationMode == e.Connection {
		entries = &DoubleIpEntries{}
	} else {
		entries = &SingleIpEntries{}
	}
	entries.init()

	return ToptalkerDatabase{
		entries:            entries,
		thresholdBps:       params.ThresholdBps,
		thresholdPps:       params.ThresholdPps,
		thresholdBuckets:   params.ThresholdBuckets,
		cleanupWindowSizes: params.CleanupWindowSizes,
		cleanupCounter:     params.Buckets * params.CleanupWindowSizes, // cleanup every N windows
		promExporter:       promExporter,
		buckets:            params.Buckets,
		ReportBuckets:      params.ReportBuckets,
		TrafficType:        params.TrafficType,
		BucketDuration:     params.BucketDuration,
		stopCleanupC:       make(chan struct{}),
		stopClockC:         make(chan struct{}),
		evaluationMode:     evaluationMode,
	}
}

func (db *ToptalkerDatabase) GetRecord(key []byte) Record {
	return db.GetTypedRecord("", key) //only have one map using empty string for untyped records
}

func (db *ToptalkerDatabase) GetTypedRecord(typeLabel string, key []byte) Record {
	db.Lock()
	defer db.Unlock()
	record, found := db.entries.getTypedRecord(key, typeLabel)
	if !found || record == nil {
		switch db.evaluationMode {
		case e.Connection:
			mid := len(key) / 2 // two concatinated ips
			srcAddr := net.IP(key[:mid]).String()
			dstAddr := net.IP(key[mid:]).String()
			displayString := fmt.Sprintf("%s <-> %s", srcAddr, dstAddr)
			record = NewTwoWayRecord(db.ReportBuckets, displayString, srcAddr, dstAddr)
		default:
			displayString := net.IP(key).String()
			record = NewDefaultRecord(db.ReportBuckets, displayString)
		}
		db.entries.upsertRecord(key, record, typeLabel)
	}
	return record
}

func NewTwoWayRecord(windowSize int, display string, srcAddr string, dstAddr string) Record {
	record := &TwoWayRecord{
		DefaultRecord: *NewDefaultRecord(windowSize, display),
		SrcAddr:       srcAddr,
		DstAddr:       dstAddr,
	}
	return record
}

func NewDefaultRecord(windowSize int, display string) *DefaultRecord {
	record := &DefaultRecord{
		FwdBytes:    make([]uint64, windowSize),
		FwdPackets:  make([]uint64, windowSize),
		DropBytes:   make([]uint64, windowSize),
		DropPackets: make([]uint64, windowSize),
		capacity:    windowSize,
		pointer:     0,
		Display:     display,
	}
	return record
}

func (record *DefaultRecord) AboveThreshold() *atomic.Bool {
	return &record.AboveThresholdAtomic
}

func (record *DefaultRecord) Append(msg *pb.EnrichedFlow) {
	bytes := msg.Bytes
	packets := msg.Packets
	statusFwd := msg.IsForwarded()
	record.Lock()
	defer record.Unlock()
	if statusFwd {
		record.FwdBytes[record.pointer] += bytes
		record.FwdPackets[record.pointer] += packets
	} else {
		record.DropBytes[record.pointer] += bytes
		record.DropPackets[record.pointer] += packets
	}
}

func (record *DefaultRecord) isEmpty() bool {
	record.RLock()
	defer record.RUnlock()
	for i := 0; i < record.capacity; i++ {
		if record.FwdPackets[i] > 0 || record.DropPackets[i] > 0 {
			return false
		}
	}
	return true
}

func (record *DefaultRecord) GetMetrics(buckets int, bucketDuration int) (float64, float64, float64, float64, string) {
	// buckets == 0 means "look at the whole window"
	if buckets == 0 {
		buckets = record.capacity
	}
	sumFwdBytes := uint64(0)
	sumFwdPackets := uint64(0)
	sumDropBytes := uint64(0)
	sumDropPackets := uint64(0)
	record.RLock()
	defer record.RUnlock()
	pos := record.pointer
	for i := 0; i < buckets; i++ {
		if pos <= 0 {
			pos = record.capacity - 1
		} else {
			pos--
		}
		sumFwdBytes += record.FwdBytes[pos]
		sumFwdPackets += record.FwdPackets[pos]
		sumDropBytes += record.DropBytes[pos]
		sumDropPackets += record.DropPackets[pos]
	}
	sumFwdBps := float64(sumFwdBytes*8) / float64(buckets*bucketDuration)
	sumFwdPps := float64(sumFwdPackets) / float64(buckets*bucketDuration)
	sumDropBps := float64(sumDropBytes*8) / float64(buckets*bucketDuration)
	sumDropPps := float64(sumDropPackets) / float64(buckets*bucketDuration)
	return sumFwdBps, sumFwdPps, sumDropBps, sumDropPps, record.Display
}

func (record *DefaultRecord) tick(thresholdBuckets int, bucketDuration int, thresholdBps uint64, thresholdPps uint64) {
	record.Lock()
	defer record.Unlock()
	// advance pointer to the next position
	record.pointer++
	if record.pointer >= record.capacity {
		record.pointer = 0
	}
	// calculate averages and check thresholds
	if thresholdBuckets == 0 {
		// thresholdBuckets == 0 means "look at the whole window"
		thresholdBuckets = record.capacity
	}
	var sumBytes uint64
	var sumPackets uint64
	pos := record.pointer
	for i := 0; i < thresholdBuckets; i++ {
		if pos <= 0 {
			pos = record.capacity - 1
		} else {
			pos--
		}
		sumBytes = sumBytes + record.FwdBytes[pos] + record.DropBytes[pos]
		sumPackets = sumPackets + record.FwdPackets[pos] + record.DropPackets[pos]
	}
	bps := uint64(float64(sumBytes*8) / float64(bucketDuration*thresholdBuckets))
	pps := uint64(float64(sumPackets) / float64(bucketDuration*thresholdBuckets))
	if (bps > thresholdBps) && (pps > thresholdPps) {
		record.AboveThresholdAtomic.Store(true)
	} else {
		record.AboveThresholdAtomic.Store(false)
	}
	// clear the current bucket
	record.FwdBytes[record.pointer] = 0
	record.FwdPackets[record.pointer] = 0
	record.DropBytes[record.pointer] = 0
	record.DropPackets[record.pointer] = 0
}

func (db *ToptalkerDatabase) Clock() {
	ticker := time.NewTicker(time.Duration(db.BucketDuration) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			db.Lock()
			for record := range db.entries.getAllEntries() {
				record.tick(db.thresholdBuckets, db.BucketDuration, db.thresholdBps, db.thresholdPps)
			}
			db.Unlock()
		case <-db.stopClockC:
			return
		}
	}
}

func (db *ToptalkerDatabase) Cleanup() {
	ticker := time.NewTicker(time.Duration(db.BucketDuration*db.buckets) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			db.Lock()
			db.cleanupCounter--
			if db.cleanupCounter <= 0 {
				db.cleanupCounter = db.buckets * db.cleanupWindowSizes
				db.entries.cleanup()
			}
			db.promExporter.dbSize.Set(float64(db.entries.count()))
			db.Unlock()
		case <-db.stopCleanupC:
			return
		}
	}
}

func (db *ToptalkerDatabase) StopTimers() {
	var stopmessage struct{}
	db.stopClockC <- stopmessage
}

func (db *ToptalkerDatabase) GetAllRecords() <-chan Record {
	out := make(chan Record)
	go func() {
		db.Lock()
		defer func() {
			db.Unlock()
			close(out)
		}()
		for record := range db.entries.getAllEntries() {
			out <- record
		}
	}()
	return out
}
