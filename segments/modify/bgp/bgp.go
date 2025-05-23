// Enriches flows with infos from BGP.
package bgp

import (
	"io/ioutil"
	"net"
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/BelWue/bgp_routeinfo/routeinfo"
	"github.com/BelWue/flowpipeline/pb"
	"github.com/BelWue/flowpipeline/segments"
	"gopkg.in/yaml.v2"
)

type Bgp struct {
	segments.BaseSegment
	FileName        string // required
	FallbackRouter  string // optional, default is "" (i.e., none or disabled), this will determine the BGP session that is used when SamplerAddress has no corresponding session
	UseFallbackOnly bool   // optional, default is false, this will disable looking for SamplerAddress BGP sessions

	routeInfoServer routeinfo.RouteInfoServer
}

func (segment Bgp) New(config map[string]string) segments.Segment {
	rsconfig, err := ioutil.ReadFile(config["filename"])
	if err != nil {
		log.Error().Err(err).Msg(" Bgp: Error reading BGP session config file: ")
		return nil
	}
	var rs routeinfo.RouteInfoServer
	err = yaml.Unmarshal(rsconfig, &rs)
	if err != nil {
		log.Error().Err(err).Msg(" Bgp: Error parsing BGP session configuration YAML: ")
		return nil
	}

	if fallback, present := config["fallbackrouter"]; present {
		if _, ok := rs.Routers[fallback]; !ok {
			log.Error().Msgf("Bgp: No fallback router named '%s' has been configured.", fallback)
			return nil
		}
	}

	fallbackonly, err := strconv.ParseBool(config["usefallbackonly"])
	if err != nil {
		log.Info().Msg("Bgp: 'usefallbackonly' set to default 'false'.")
	}
	if fallbackonly && config["fallbackrouter"] == "" {
		log.Error().Msgf("Bgp: Forcing fallback requires a fallbackrouter parameter.")
		return nil
	}

	newSegment := &Bgp{
		FileName:        config["filename"],
		FallbackRouter:  config["fallbackrouter"],
		UseFallbackOnly: fallbackonly,
		routeInfoServer: rs,
	}
	return newSegment
}

func (segment *Bgp) Run(wg *sync.WaitGroup) {
	defer func() {
		close(segment.Out)
		wg.Done()
	}()

	segment.routeInfoServer.Init()
	defer func() {
		segment.routeInfoServer.Stop()
	}()

	for msg := range segment.In {
		// The following conversions to String are stupid, but it is
		// what gobgp requires at the end of this call hierarchy.
		var routeInfos []routeinfo.RouteInfo
		if segment.UseFallbackOnly {
			routeInfos = segment.routeInfoServer.Routers[segment.FallbackRouter].Lookup(msg.DstAddrObj().String())
		} else {
			if router, ok := segment.routeInfoServer.Routers[msg.SamplerAddressObj().String()]; ok {
				routeInfos = router.Lookup(msg.DstAddrObj().String())
			} else if segment.FallbackRouter != "" {
				routeInfos = segment.routeInfoServer.Routers[segment.FallbackRouter].Lookup(msg.DstAddrObj().String())
			} else {
				segment.Out <- msg
				continue
			}
		}
		for _, path := range routeInfos {
			if !path.Best {
				continue
			}
			msg.AsPath = path.AsPath
			msg.Med = path.Med
			msg.LocalPref = path.LocalPref
			switch path.Validation {
			case routeinfo.Valid:
				msg.ValidationStatus = pb.EnrichedFlow_Valid
			case routeinfo.NotFound:
				msg.ValidationStatus = pb.EnrichedFlow_NotFound
			case routeinfo.Invalid:
				msg.ValidationStatus = pb.EnrichedFlow_Invalid
			default:
				msg.ValidationStatus = pb.EnrichedFlow_Unknown
			}
			// for router exported netflow, the following are likely overwriting their own annotations
			msg.DstAs = path.AsPath[len(path.AsPath)-1]
			msg.NextHopAs = path.AsPath[0]
			if nh := net.ParseIP(path.NextHop); nh != nil {
				msg.NextHop = nh
			}
			break
		}
		// we could look at the routing for the SrcAddr here...
		segment.Out <- msg
	}
}

func init() {
	segment := &Bgp{}
	segments.RegisterSegment("bgp", segment)
}
