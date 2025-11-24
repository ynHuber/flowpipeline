// based on https://github.com/kyleseneker/go-traceroute/tree/main?tab=MIT-1-ov-file (MIT license)

package maptrifficflows

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const DEBUGMODE = false

// TracerouteConfig holds the configuration for the traceroute operation
type TracerouteConfig struct {
	PacketSize        int
	FirstTTL          int
	MaxTTL            int
	BasePort          int
	WaitTimeMs        int
	DestIP            net.IP
	MaxEmptyResponses int
}

// runTraceroute runs the traceroute operation based on the given configuration
func runTraceroute(config *TracerouteConfig) (error, []net.IP) {
	log.Trace().Msgf("traceroute to %s, %d hops max, %d byte packets\n", config.DestIP, config.MaxTTL, config.PacketSize)
	ipPath := []net.IP{}
	// Create ICMP packet listener
	var (
		destString string
		recvConn   *icmp.PacketConn
		err        error
	)
	v4 := config.DestIP.To4()
	if v4 != nil {
		recvConn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
		destString = v4.String()
	} else {
		v6 := config.DestIP.To16()
		if v6 == nil {
			return fmt.Errorf("Invalid IP-Address: %s", config.DestIP.String()), nil
		}
		recvConn, err = icmp.ListenPacket("ip6:icmp", "::")
		destString = v6.String()
	}

	if err != nil {
		return fmt.Errorf("could not create receive socket: %v", err), nil
	}
	defer recvConn.Close()

	destinationReached := false
	emptyResponsesInARow := 0

	// Iterate through TTL values
	for ttl := config.FirstTTL; ttl < config.FirstTTL+config.MaxTTL && !destinationReached; ttl++ {
		respondingIP, allFailed, err := sendProbes(ttl, config, recvConn, destString)
		if err != nil {
			return err, ipPath
		}

		if !allFailed {
			if !respondingIP.Equal(net.IP{}) {
				ipPath = append(ipPath, respondingIP) // Print results
				emptyResponsesInARow = 0
			} else {
				emptyResponsesInARow++
			}
			if DEBUGMODE {
				output := getOutput(respondingIP.String())
				printResults(ttl, output, respondingIP.String())
			}
		}

		// Break if we have reached the destination after all probes for this TTL
		if respondingIP.String() == destString {
			destinationReached = true
		}

		if emptyResponsesInARow > config.MaxEmptyResponses {
			return nil, ipPath
		}
	}

	return nil, ipPath
}

// sendProbes sends probes for a given TTL and returns the responding IP and results
func sendProbes(ttl int, config *TracerouteConfig, recvConn *icmp.PacketConn, destString string) (net.IP, bool, error) {
	respondingIP := net.IP{}
	failed := true

	dstAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", config.DestIP, config.BasePort+ttl))
	if err != nil {
		return respondingIP, failed, err
	}

	sendConn, err := net.DialUDP("udp4", nil, dstAddr)
	if err != nil {
		return respondingIP, failed, err
	}

	p := ipv4.NewPacketConn(sendConn)
	if err := p.SetTTL(ttl); err != nil {
		sendConn.Close()
		return respondingIP, failed, err
	}

	_, err = sendConn.Write(make([]byte, config.PacketSize))
	sendConn.Close()
	if err != nil {
		return respondingIP, failed, err
	}

	reply := make([]byte, 1500)
	err = recvConn.SetReadDeadline(time.Now().Add(time.Duration(config.WaitTimeMs) * time.Millisecond))
	if err != nil {
		return respondingIP, failed, err
	}

	n, peer, err := recvConn.ReadFrom(reply)
	if err != nil {
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			//reached first timout exiting
			return respondingIP, false, nil
		}
		return respondingIP, false, fmt.Errorf("could not read ICMP message: %v", err)
	}

	icmpMessage, err := icmp.ParseMessage(1, reply[:n])
	if err != nil {
		return respondingIP, failed, err
	}

	peerIP := peer.(*net.IPAddr)

	switch icmpMessage.Type {
	case ipv4.ICMPTypeTimeExceeded, ipv4.ICMPTypeDestinationUnreachable:
		respondingIP = peerIP.IP
		failed = false
	case ipv4.ICMPTypeEchoReply:
		if peerIP.IP.String() == destString {
			respondingIP = peerIP.IP
			failed = false
		} else {
		}
	default:
		failed = false
	}

	return respondingIP, failed, nil
}

func getOutput(respondingIP string) string {
	if respondingIP == "" {
		return ""
	}

	hosts, err := net.LookupAddr(respondingIP)
	if err != nil || len(hosts) == 0 {
		return respondingIP
	}

	return strings.TrimSuffix(hosts[0], ".")
}

// printResults prints the traceroute results for a given TTL
func printResults(ttl int, output, respondingIP string) {
	if respondingIP != "" {
		fmt.Printf("%d  %s (%s)  ", ttl, output, respondingIP)
	} else {
		fmt.Printf("%d  ", ttl)
	}
	fmt.Println()
}
