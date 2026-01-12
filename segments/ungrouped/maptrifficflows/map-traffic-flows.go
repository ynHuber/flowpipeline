// TODO: Compile this using:
// `go build -buildmode=plugin ./examples/plugin/printcustom.go`
package maptrifficflows

import (
	"context"
	"encoding/csv"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/BelWue/flowpipeline/pb"
	"codeberg.org/BelWue/flowpipeline/segments"
	"codeberg.org/BelWue/flowpipeline/segments/base/basesegment"
	"github.com/rs/zerolog/log"
)

// Used to generate simple BelWü specific traffic maps in csv-form (SrcRouter,DestRouter,Amount,Number of Packets,Duration,Protocol)
type MapTrafficFlows struct {
	basesegment.BaseSegment
	sync.Mutex

	filePath                string
	file                    *os.File
	writer                  *csv.Writer
	resolver                *net.Resolver
	coreRouterNames         []string
	resolverCache           *ResolverCache
	coreRouterResolverCache *FinalCoreRouterResolverCache
	linecount               int
	TraceRouteMutex         *sync.Mutex

	writeBufferSize int
	stopChan        chan struct{}
}

type ResolverCache struct {
	sync.RWMutex

	cache map[[16]byte]*ResolverCacheResult
}

type FinalCoreRouterResolverCache struct {
	sync.RWMutex
	cacheLocks map[[16]byte]*sync.Mutex
	cache      *RouterNameCacheMap
}

type RouterNameCacheMap struct {
	sync.RWMutex
	cacheMap map[[16]byte]string
}

func (r *RouterNameCacheMap) getValue(key [16]byte) (string, bool) {
	r.RLock()
	v, b := r.cacheMap[key]
	r.RUnlock()
	return v, b
}

func (r *RouterNameCacheMap) addValue(key [16]byte, value string) {
	r.Lock()
	r.cacheMap[key] = value
	r.Unlock()
}

func (s *MapTrafficFlows) getFinalCoreRouterName(iP net.IP) string {
	addrKey := ByteKeyForAddr(iP)
	addrLock := s.coreRouterResolverCache.getLockForKey(addrKey)
	addrLock.Lock()
	finalCoreRouterName, ok := s.coreRouterResolverCache.cache.getValue(addrKey)
	if ok {
		addrLock.Unlock()
		return finalCoreRouterName
	}

	finalCoreRouterName = s.resolveFinalCoreRouterName(iP)
	s.coreRouterResolverCache.cache.addValue(addrKey, finalCoreRouterName)
	addrLock.Unlock()
	return finalCoreRouterName
}

func (s *MapTrafficFlows) resolveFinalCoreRouterName(iP net.IP) string {
	err, route := runTraceroute(&TracerouteConfig{
		DestIP:            iP,
		PacketSize:        40,
		FirstTTL:          1,
		MaxTTL:            64,
		BasePort:          33434,
		WaitTimeMs:        250,
		MaxEmptyResponses: 3,
	}, s.TraceRouteMutex)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to run traceroute")
	}

	//iterate backwards over list:
	maxHopsIndex := len(route) - 1
	for addressId := range route {
		//	check if host name is in router.csv
		hop := route[maxHopsIndex-(addressId)]
		if isCoreRouter, coreRouterName := s.checkIfIsCoreRouter(hop); isCoreRouter {
			return *coreRouterName
		}
	}

	// -> return first match router name
	return "Unknown"
}

func (f *FinalCoreRouterResolverCache) getLockForKey(addrKey [16]byte) *sync.Mutex {
	f.RLock()
	addrLock, ok := f.cacheLocks[addrKey]
	f.RUnlock()
	if ok {
		return addrLock
	}

	f.Lock()
	//retry in case it was resolved while waiting for the lock
	addrLock, ok = f.cacheLocks[addrKey]
	if !ok {
		addrLock = &sync.Mutex{}
		f.cacheLocks[addrKey] = addrLock
	}
	f.Unlock()
	return addrLock
}
func (r *ResolverCache) LookupAddr(addr net.IP) *ResolverCacheResult {
	addrKey := ByteKeyForAddr(addr)
	r.RLock()
	res := r.cache[addrKey]
	r.RUnlock()
	if res != nil {
		return res
	}
	r.Lock()
	//retry in case multiple want to wlock at once
	res = r.cache[addrKey]
	if res != nil {
		r.Unlock()
		return res
	}
	lookupResultAddr, lookupResultErr := net.DefaultResolver.LookupAddr(context.Background(), addr.String())
	resolverCacheResult := ResolverCacheResult{
		Addr: lookupResultAddr,
		Err:  lookupResultErr,
	}
	r.cache[addrKey] = &resolverCacheResult
	r.Unlock()
	return &resolverCacheResult
}

func ByteKeyForAddr(addr net.IP) [16]byte {
	addr16 := addr.To16()
	var addrkey [16]byte
	copy(addrkey[:], addr16)
	return addrkey
}

type ResolverCacheResult struct {
	Addr []string
	Err  error
}

func (segment *MapTrafficFlows) New(config map[string]string) segments.Segment {
	if os.Geteuid() != 0 {
		log.Fatal().Msg("MapTrafficFlows: insufficient privileges for internalk traceroute: try running with sudo")
		return nil
	}

	newsegment := &MapTrafficFlows{
		resolver:        &net.Resolver{},
		writeBufferSize: 250,
		resolverCache: &ResolverCache{
			cache: map[[16]byte]*ResolverCacheResult{},
		},
		coreRouterResolverCache: NewFinalCoreRouterResolverCache(),
		stopChan:                make(chan struct{}),
		TraceRouteMutex:         &sync.Mutex{},
	}
	if config["filename"] != "" {
		newsegment.filePath = config["filename"]
	} else {
		newsegment.filePath = "trafficmappings"
	}
	os.MkdirAll(newsegment.filePath, 0755)

	log.Info().Msgf("MapTrafficFlow: configured output to %s", newsegment.filePath)

	newsegment.initCoreRouterList()
	return newsegment
}

func NewFinalCoreRouterResolverCache() *FinalCoreRouterResolverCache {
	return &FinalCoreRouterResolverCache{
		cacheLocks: map[[16]byte]*sync.Mutex{},
		cache: &RouterNameCacheMap{
			cacheMap: map[[16]byte]string{},
		},
	}
}

func (segment *MapTrafficFlows) initCoreRouterList() {
	file, err := os.Open("corerouters.csv")
	if err != nil {
		log.Fatal().Err(err).Msgf("Error opening file: %s", file.Name())
		return
	}
	defer file.Close()

	// Create a CSV reader
	reader := csv.NewReader(file)

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal().Err(err).Msgf("Error reading csv file: %s", file.Name())
		return
	}

	for i, record := range records {
		if i == 0 {
			// Skip header row
			continue
		}
		segment.coreRouterNames = append(segment.coreRouterNames, record[0])
	}
}

func (segment *MapTrafficFlows) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	//Best to use aggregated flows
	segment.initNewWriter()
	go segment.StartBackgroundWriterRefresh()

	for msgIn := range segment.In {
		go func(msg *pb.EnrichedFlow) {
			defer func() {
				if processingCrash := recover(); processingCrash != nil {
					log.Error().Msgf("ProcessFlowSrcDst crashed: %v", processingCrash)
					panic(processingCrash)
				}
			}()
			srcRouterName := segment.getSrcRouterName(msg)
			destRouterName := segment.getDstRouterName(msg)
			flowDuration := getFlowDuration(msg)
			record := []string{
				srcRouterName,
				destRouterName,
				strconv.FormatUint(msg.Bytes, 10),
				strconv.FormatUint(msg.Packets, 10),
				strconv.FormatUint(flowDuration, 10),
				strconv.FormatUint(uint64(msg.Proto), 10),
				strconv.FormatUint(msg.TimeFlowStart, 10),
			} //"SrcRouter", "DestRouter", "Amount", "Number of Packets", "Duration", "Protocol", "TimeFlowStart", "SrcAddr", "DstAddr"
			segment.write(record)
		}(msgIn)
		segment.Out <- msgIn
	}
}

func (s *MapTrafficFlows) StartBackgroundWriterRefresh() {
	ticker := time.NewTicker(20 * time.Second)
	for {
		select {
		case <-ticker.C:
			s.Lock()
			// Flush the old writer
			s.writer.Flush()
			if err := s.writer.Error(); err != nil {
				log.Error().Err(err).Msg("CSV writing error:")
			}
			err := s.file.Close()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error closing file: %v\n", err)
			}
			s.initNewWriter()
			s.Unlock()

		case <-s.stopChan:
			ticker.Stop()
			s.Lock()
			s.writer.Flush()
			s.file.Close()
			s.Unlock()
			return
		}
	}
}

// Create new file with new timestamp
func (s *MapTrafficFlows) initNewWriter() {
	var err error
	s.linecount = 0
	currentFilename := fmt.Sprintf("%s/%s.csv", s.filePath, time.Now().Format("2006-01-02_15-04-05"))
	s.file, err = os.Create(currentFilename)
	if err != nil {
		panic(err)
	}

	// Create new buffered writer
	s.writer = csv.NewWriter(s.file)
	heading := []string{"SrcRouter", "DestRouter", "Amount", "Number of Packets", "Duration", "Protocol", "TimeFlowStart"}
	if err := s.writer.Write(heading); err != nil {
		log.Error().Err(err).Msg("Csv: Failed to write to destination:")
		panic(err)
	}
}

func (segment *MapTrafficFlows) write(record []string) {
	segment.Lock()
	segment.writer.Write(record)
	segment.linecount++
	if segment.linecount > segment.writeBufferSize {
		segment.linecount = 0
		segment.writer.Flush()
		if err := segment.writer.Error(); err != nil {
			log.Error().Err(err).Msg("CSV writing error:")
		}
	}
	segment.Unlock()
}

func getFlowDuration(msg *pb.EnrichedFlow) uint64 {
	msg.SyncMissingTimeStamps()
	return msg.GetTimeFlowEnd() - msg.GetTimeFlowStart()
}

func (s *MapTrafficFlows) getDstRouterName(msg *pb.EnrichedFlow) string {
	switch msg.FlowDirection {
	case 0: // flow is ingress on border interface
		return s.getFinalCoreRouterName(msg.DstAddrObj())
	case 1: // flow is egress on border interface
		isCoreRouter, coreRouterName := s.checkIfIsCoreRouter(msg.SamplerAddressObj())
		if isCoreRouter {
			return *coreRouterName
		}
		return "SampleRouter is not a core router"
	default:
		log.Fatal().Msg("Bad flowdirection")
		return "Unknown"
	}
}

func (s *MapTrafficFlows) checkIfIsCoreRouter(address net.IP) (bool, *string) {
	names, err := s.LookupAddr(address)
	if err != nil {
		return false, nil
	}
	for _, name := range names {
		if isCoreRouter, coreRouterName := s.isCoreRouter(name); isCoreRouter {
			return true, coreRouterName
		}
	}
	return false, nil
}

func (s *MapTrafficFlows) LookupAddr(addr net.IP) ([]string, error) {
	returnResult := s.resolverCache.LookupAddr(addr)
	return returnResult.Addr, returnResult.Err
}

func (s *MapTrafficFlows) isCoreRouter(name string) (bool, *string) {
	for _, coreRouterName := range s.coreRouterNames {
		if strings.HasPrefix(name, coreRouterName) {
			return true, &coreRouterName
		}
	}
	return false, nil
}

func (s *MapTrafficFlows) getSrcRouterName(msg *pb.EnrichedFlow) string {
	switch msg.FlowDirection {
	case 0: // flow is ingress on border interface
		isCoreRouter, coreRouterName := s.checkIfIsCoreRouter(msg.SamplerAddressObj())
		if isCoreRouter {
			return *coreRouterName
		}
		return "SampleRouter is not a core router"
	case 1: // flow is egress on border interface
		return s.getFinalCoreRouterName(msg.SrcAddrObj())
	default:
		log.Fatal().Msg("Bad flowdirection")
		return "Unknown"
	}
}

/***************************
* Returns the router name based on reverse dns lookup
****************************/
func (s *MapTrafficFlows) getRouterNames(address net.IP) []string {
	name, err := s.LookupAddr(address)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to look up address %s", address.String())
	} else if name != nil {
		return name
	}
	return []string{}
}

func init() {
	segment := &MapTrafficFlows{}
	segments.RegisterSegment("maptrafficflows", segment)
}

func (segment *MapTrafficFlows) Close() {
	segment.stopChan <- struct{}{}
	close(segment.stopChan)
}
