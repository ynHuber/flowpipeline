// TODO: Compile this using:
// `go build -buildmode=plugin ./examples/plugin/printcustom.go`
package maptrifficflows

import (
	"context"
	"encoding/csv"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"codeberg.org/BelWue/flowpipeline/pb"
	"codeberg.org/BelWue/flowpipeline/segments"
	"codeberg.org/BelWue/flowpipeline/segments/base/basetextoutputsegment"
	"github.com/rs/zerolog/log"
)

// Used to generate simple BelWü specific traffic maps in csv-form (SrcRouter,DestRouter,Amount,Number of Packets,Duration,Protocol)
type MapTrafficFlows struct {
	basetextoutputsegment.BaseTextOutputSegment
	sync.Mutex

	writer          *csv.Writer
	resolver        *net.Resolver
	coreRouterNames []string
	resolverCache   *ResolverCache

	linecount       int
	writeBufferSize int
}

type ResolverCache struct {
	sync.RWMutex

	cache map[[16]byte]*ResolverCacheResult
}

func (r *ResolverCache) LookupAddr(addr net.IP) *ResolverCacheResult {
	addrkey := addr.To16()
	if len(addrkey) != 16 {
		return &ResolverCacheResult{Err: errors.New("Conversion to ipv6 failed")}
	}
	var addrkeyO [16]byte
	copy(addrkeyO[:], addrkey)

	r.RLock()
	res := r.cache[addrkeyO]
	r.RUnlock()
	if res != nil {
		return res
	}
	r.Lock()
	//retry in case multiple want to wlock at once
	res = r.cache[addrkeyO]
	if res != nil {
		r.Unlock()
		return res
	}
	lookupResultAddr, lookupResultErr := net.DefaultResolver.LookupAddr(context.Background(), addr.String())
	resolverCacheResult := ResolverCacheResult{
		Addr: lookupResultAddr,
		Err:  lookupResultErr,
	}
	r.cache[addrkeyO] = &resolverCacheResult
	r.Unlock()
	return r.cache[addrkeyO]
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
		writeBufferSize: 500,
		linecount:       0,
		resolverCache: &ResolverCache{
			cache: map[[16]byte]*ResolverCacheResult{},
		},
	}
	file, err := segment.GetOutput(config)
	if err != nil {
		log.Error().Err(err).Msg("MapTrafficFlow: File specified in 'filename' is not accessible: ")
		return nil
	}
	log.Info().Msgf("MapTrafficFlow: configured output to %s", file.Name())

	newsegment.initCoreRouterList()

	heading := []string{"SrcRouter", "DestRouter", "Amount", "Number of Packets", "Duration", "Protocol", "TimeFlowStart"}
	newsegment.writer = csv.NewWriter(file)
	if err := newsegment.writer.Write(heading); err != nil {
		log.Error().Err(err).Msg("Csv: Failed to write to destination:")
		return nil
	}
	newsegment.writer.Flush()

	return newsegment
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

	for msg := range segment.In {
		go func() {
			srcRouterName := segment.getSrcRouterName(msg)
			destRouterName := segment.getDstRouterName(msg)
			flowDuration := getFlowDuration(msg)
			record := []string{
				srcRouterName,
				destRouterName,
				strconv.FormatUint(msg.Bytes, 10),
				strconv.FormatUint(msg.Packets, 10),
				strconv.FormatUint(flowDuration, 10),
				strconv.FormatUint(uint64(msg.Proto), 10)}
			segment.write(record)
			segment.Out <- msg
		}()
	}
}

func (segment *MapTrafficFlows) write(record []string) {
	segment.Lock()
	defer segment.Unlock()
	segment.writer.Write(record)
	segment.linecount++
	if segment.linecount > segment.writeBufferSize {
		segment.linecount = 0
		segment.writer.Flush()
	}
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

func (s *MapTrafficFlows) getFinalCoreRouterName(iP net.IP) string {
	//ToDo: caching??
	//traceroute ip
	err, route := runTraceroute(&TracerouteConfig{
		DestIP:            iP,
		PacketSize:        40,
		FirstTTL:          1,
		MaxTTL:            64,
		BasePort:          33434,
		WaitTimeMs:        250,
		MaxEmptyResponses: 3,
	})
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

func (s *MapTrafficFlows) checkIfIsCoreRouter(address net.IP) (bool, *string) {
	names, err := s.LookupAddr(address)
	if err != nil {
		log.Trace().Err(err).Msgf("Failed to look up address %s", address.String())
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
