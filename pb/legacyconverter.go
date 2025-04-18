package pb

import "math"

func (legacy *LegacyEnrichedFlow) ConvertToEnrichedFlow() *EnrichedFlow {
	enrichedFlow := EnrichedFlow{
		Type:             EnrichedFlow_FlowType(legacy.Type),
		TimeReceived:     legacy.TimeReceived,
		SequenceNum:      legacy.SequenceNum,
		SamplingRate:     legacy.SamplingRate,
		FlowDirection:    legacy.FlowDirection,
		SamplerAddress:   legacy.SamplerAddress,
		TimeFlowStart:    legacy.TimeFlowStart,
		TimeFlowEnd:      legacy.TimeFlowEnd,
		Bytes:            legacy.Bytes,
		Packets:          legacy.Packets,
		SrcAddr:          legacy.SrcAddr,
		DstAddr:          legacy.DstAddr,
		Etype:            legacy.Etype,
		Proto:            legacy.Proto,
		SrcPort:          legacy.SrcPort,
		DstPort:          legacy.DstPort,
		InIf:             legacy.InIf,
		OutIf:            legacy.OutIf,
		SrcMac:           legacy.SrcMac,
		DstMac:           legacy.DstMac,
		SrcVlan:          legacy.SrcVlan,
		DstVlan:          legacy.DstVlan,
		VlanId:           legacy.VlanId,
		IngressVrfId:     legacy.IngressVrfID,
		EgressVrfId:      legacy.EgressVrfID,
		IpTos:            legacy.IPTos,
		ForwardingStatus: legacy.ForwardingStatus,
		IpTtl:            legacy.IPTTL,
		TcpFlags:         legacy.TCPFlags,
		IcmpType:         legacy.IcmpType,
		IcmpCode:         legacy.IcmpCode,
		Ipv6FlowLabel:    legacy.IPv6FlowLabel,
		FragmentId:       legacy.FragmentId,
		FragmentOffset:   legacy.FragmentOffset,
		BiFlowDirection:  legacy.BiFlowDirection,
		SrcAs:            legacy.SrcAS,
		DstAs:            legacy.DstAS,
		NextHop:          legacy.NextHop,
		NextHopAs:        legacy.NextHopAS,
		SrcNet:           legacy.SrcNet,
		DstNet:           legacy.DstNet,
		HasMpls:          legacy.HasMPLS,
		MplsCount:        legacy.MPLSCount,
		Mpls_1Ttl:        legacy.MPLS1TTL,
		Mpls_1Label:      legacy.MPLS1Label,
		Mpls_2Ttl:        legacy.MPLS2TTL,
		Mpls_2Label:      legacy.MPLS2Label,
		Mpls_3Ttl:        legacy.MPLS3TTL,
		Mpls_3Label:      legacy.MPLS3Label,
		MplsLastTtl:      legacy.MPLSLastTTL,
		MplsLastLabel:    legacy.MPLSLastLabel,
		SrcCountry:       legacy.SrcCountry,
		DstCountry:       legacy.DstCountry,

		AsPath: legacy.ASPath,
		//state:                             legacy.state,
		sizeCache:                     legacy.sizeCache,
		unknownFields:                 legacy.unknownFields,
		PacketBytesMin:                legacy.PacketBytesMin,
		PacketBytesMax:                legacy.PacketBytesMax,
		PacketBytesMean:               legacy.PacketBytesMean,
		PacketBytesStdDev:             legacy.PacketBytesStdDev,
		PacketIATMin:                  legacy.PacketIATMin,
		PacketIATMax:                  legacy.PacketIATMax,
		PacketIATMean:                 legacy.PacketIATMean,
		PacketIATStdDev:               legacy.PacketIATStdDev,
		HeaderBytes:                   legacy.HeaderBytes,
		FINFlagCount:                  legacy.FINFlagCount,
		SYNFlagCount:                  legacy.SYNFlagCount,
		RSTFlagCount:                  legacy.RSTFlagCount,
		PSHFlagCount:                  legacy.PSHFlagCount,
		ACKFlagCount:                  legacy.ACKFlagCount,
		URGFlagCount:                  legacy.URGFlagCount,
		CWRFlagCount:                  legacy.CWRFlagCount,
		ECEFlagCount:                  legacy.ECEFlagCount,
		PayloadPackets:                legacy.PayloadPackets,
		TimeActiveMin:                 legacy.TimeActiveMin,
		TimeActiveMax:                 legacy.TimeActiveMax,
		TimeActiveMean:                legacy.TimeActiveMean,
		TimeActiveStdDev:              legacy.TimeActiveStdDev,
		TimeIdleMin:                   legacy.TimeIdleMin,
		TimeIdleMax:                   legacy.TimeIdleMax,
		TimeIdleMean:                  legacy.TimeIdleMean,
		TimeIdleStdDev:                legacy.TimeIdleStdDev,
		Cid:                           legacy.Cid,
		CidString:                     legacy.CidString,
		SrcCid:                        legacy.SrcCid,
		DstCid:                        legacy.DstCid,
		SrcAddrAnon:                   EnrichedFlow_AnonymizedType(legacy.SrcAddrAnon),
		DstAddrAnon:                   EnrichedFlow_AnonymizedType(legacy.DstAddrAnon),
		SrcAddrPreservedLen:           legacy.SrcAddrPreservedLen,
		DstAddrPreservedLen:           legacy.DstAddrPreservedLen,
		SamplerAddrAnon:               EnrichedFlow_AnonymizedType(legacy.SamplerAddrAnon),
		SamplerAddrPreservedPrefixLen: legacy.SamplerAddrPreservedPrefixLen,
		Med:                           legacy.Med,
		LocalPref:                     legacy.LocalPref,
		ValidationStatus:              EnrichedFlow_ValidationStatusType(legacy.ValidationStatus),
		RemoteCountry:                 legacy.RemoteCountry,
		Normalized:                    EnrichedFlow_NormalizedType(legacy.Normalized),
		ProtoName:                     legacy.ProtoName,
		RemoteAddr:                    EnrichedFlow_RemoteAddrType(legacy.RemoteAddr),
		SrcHostName:                   legacy.SrcHostName,
		DstHostName:                   legacy.DstHostName,
		NextHopHostName:               legacy.NextHopHostName,
		SrcASName:                     legacy.SrcASName,
		DstASName:                     legacy.DstASName,
		NextHopASName:                 legacy.NextHopASName,
		SamplerHostName:               legacy.SamplerHostName,
		SrcIfName:                     legacy.SrcIfName,
		SrcIfDesc:                     legacy.SrcIfDesc,
		SrcIfSpeed:                    legacy.SrcIfSpeed,
		DstIfName:                     legacy.DstIfName,
		DstIfDesc:                     legacy.DstIfDesc,
		DstIfSpeed:                    legacy.DstIfSpeed,
		Note:                          legacy.Note,
		SourceIP:                      legacy.SourceIP,
		DestinationIP:                 legacy.DestinationIP,
		NextHopIP:                     legacy.NextHopIP,
		SamplerIP:                     legacy.SamplerIP,
		SourceMAC:                     legacy.SourceMAC,
		DestinationMAC:                legacy.DestinationMAC,
	}
	enrichedFlow.SyncMissingTimeStamps()
	return &enrichedFlow
}

func (flow *EnrichedFlow) ConvertToLegacyEnrichedFlow() *LegacyEnrichedFlow {
	flow.SyncMissingTimeStamps()

	legacyFlow := LegacyEnrichedFlow{
		Type:             LegacyEnrichedFlow_FlowType(flow.Type),
		TimeReceived:     flow.TimeReceived,
		SequenceNum:      flow.SequenceNum,
		SamplingRate:     flow.SamplingRate,
		FlowDirection:    flow.FlowDirection,
		SamplerAddress:   flow.SamplerAddress,
		Bytes:            flow.Bytes,
		Packets:          flow.Packets,
		SrcAddr:          flow.SrcAddr,
		DstAddr:          flow.DstAddr,
		Etype:            flow.Etype,
		Proto:            flow.Proto,
		SrcPort:          flow.SrcPort,
		DstPort:          flow.DstPort,
		InIf:             flow.InIf,
		OutIf:            flow.OutIf,
		SrcMac:           flow.SrcMac,
		DstMac:           flow.DstMac,
		SrcVlan:          flow.SrcVlan,
		DstVlan:          flow.DstVlan,
		VlanId:           flow.VlanId,
		IngressVrfID:     flow.IngressVrfId,
		EgressVrfID:      flow.EgressVrfId,
		IPTos:            flow.IpTos,
		ForwardingStatus: flow.ForwardingStatus,
		IPTTL:            flow.IpTtl,
		TCPFlags:         flow.TcpFlags,
		IcmpType:         flow.IcmpType,
		IcmpCode:         flow.IcmpCode,
		IPv6FlowLabel:    flow.Ipv6FlowLabel,
		FragmentId:       flow.FragmentId,
		FragmentOffset:   flow.FragmentOffset,
		BiFlowDirection:  flow.BiFlowDirection,
		SrcAS:            flow.SrcAs,
		DstAS:            flow.DstAs,
		NextHop:          flow.NextHop,
		NextHopAS:        flow.NextHopAs,
		SrcNet:           flow.SrcNet,
		DstNet:           flow.DstNet,
		HasMPLS:          flow.HasMpls,
		MPLSCount:        flow.MplsCount,
		MPLS1TTL:         flow.Mpls_1Ttl,
		MPLS1Label:       flow.Mpls_1Label,
		MPLS2TTL:         flow.Mpls_2Ttl,
		MPLS2Label:       flow.Mpls_2Label,
		MPLS3TTL:         flow.Mpls_3Ttl,
		MPLS3Label:       flow.Mpls_3Label,
		MPLSLastTTL:      flow.MplsLastTtl,
		MPLSLastLabel:    flow.MplsLastLabel,
		SrcCountry:       flow.SrcCountry,
		DstCountry:       flow.DstCountry,

		TimeFlowStart: flow.TimeFlowStart,
		TimeFlowEnd:   flow.TimeFlowEnd,

		ASPath:                        flow.AsPath,
		sizeCache:                     flow.sizeCache,
		unknownFields:                 flow.unknownFields,
		PacketBytesMin:                flow.PacketBytesMin,
		PacketBytesMax:                flow.PacketBytesMax,
		PacketBytesMean:               flow.PacketBytesMean,
		PacketBytesStdDev:             flow.PacketBytesStdDev,
		PacketIATMin:                  flow.PacketIATMin,
		PacketIATMax:                  flow.PacketIATMax,
		PacketIATMean:                 flow.PacketIATMean,
		PacketIATStdDev:               flow.PacketIATStdDev,
		HeaderBytes:                   flow.HeaderBytes,
		FINFlagCount:                  flow.FINFlagCount,
		SYNFlagCount:                  flow.SYNFlagCount,
		RSTFlagCount:                  flow.RSTFlagCount,
		PSHFlagCount:                  flow.PSHFlagCount,
		ACKFlagCount:                  flow.ACKFlagCount,
		URGFlagCount:                  flow.URGFlagCount,
		CWRFlagCount:                  flow.CWRFlagCount,
		ECEFlagCount:                  flow.ECEFlagCount,
		PayloadPackets:                flow.PayloadPackets,
		TimeActiveMin:                 flow.TimeActiveMin,
		TimeActiveMax:                 flow.TimeActiveMax,
		TimeActiveMean:                flow.TimeActiveMean,
		TimeActiveStdDev:              flow.TimeActiveStdDev,
		TimeIdleMin:                   flow.TimeIdleMin,
		TimeIdleMax:                   flow.TimeIdleMax,
		TimeIdleMean:                  flow.TimeIdleMean,
		TimeIdleStdDev:                flow.TimeIdleStdDev,
		Cid:                           flow.Cid,
		CidString:                     flow.CidString,
		SrcCid:                        flow.SrcCid,
		DstCid:                        flow.DstCid,
		SrcAddrAnon:                   LegacyEnrichedFlow_AnonymizedType(flow.SrcAddrAnon),
		DstAddrAnon:                   LegacyEnrichedFlow_AnonymizedType(flow.DstAddrAnon),
		SrcAddrPreservedLen:           flow.SrcAddrPreservedLen,
		DstAddrPreservedLen:           flow.DstAddrPreservedLen,
		SamplerAddrAnon:               LegacyEnrichedFlow_AnonymizedType(flow.SamplerAddrAnon),
		SamplerAddrPreservedPrefixLen: flow.SamplerAddrPreservedPrefixLen,
		Med:                           flow.Med,
		LocalPref:                     flow.LocalPref,
		ValidationStatus:              LegacyEnrichedFlow_ValidationStatusType(flow.ValidationStatus),
		RemoteCountry:                 flow.RemoteCountry,
		Normalized:                    LegacyEnrichedFlow_NormalizedType(flow.Normalized),
		ProtoName:                     flow.ProtoName,
		RemoteAddr:                    LegacyEnrichedFlow_RemoteAddrType(flow.RemoteAddr),
		SrcHostName:                   flow.SrcHostName,
		DstHostName:                   flow.DstHostName,
		NextHopHostName:               flow.NextHopHostName,
		SrcASName:                     flow.SrcASName,
		DstASName:                     flow.DstASName,
		NextHopASName:                 flow.NextHopASName,
		SamplerHostName:               flow.SamplerHostName,
		SrcIfName:                     flow.SrcIfName,
		SrcIfDesc:                     flow.SrcIfDesc,
		SrcIfSpeed:                    flow.SrcIfSpeed,
		DstIfName:                     flow.DstIfName,
		DstIfDesc:                     flow.DstIfDesc,
		DstIfSpeed:                    flow.DstIfSpeed,
		Note:                          flow.Note,
		SourceIP:                      flow.SourceIP,
		DestinationIP:                 flow.DestinationIP,
		NextHopIP:                     flow.NextHopIP,
		SamplerIP:                     flow.SamplerIP,
		SourceMAC:                     flow.SourceMAC,
		DestinationMAC:                flow.DestinationMAC,
	}
	return &legacyFlow

}

func (flow *EnrichedFlow) SyncMissingTimeStamps() {
	fillTimeIfEmptyWithFallback(&flow.TimeFlowEnd, &flow.TimeFlowEndNs, 1e-9, &flow.TimeFlowEndMs, 1e-3)
	fillTimeIfEmptyWithFallback(&flow.TimeFlowEndMs, &flow.TimeFlowEndNs, 1e-6, &flow.TimeFlowEnd, 1e3)
	fillTimeIfEmptyWithFallback(&flow.TimeFlowEndNs, &flow.TimeFlowEndMs, 1e6, &flow.TimeFlowEnd, 1e9)

	fillTimeIfEmptyWithFallback(&flow.TimeFlowStart, &flow.TimeFlowStartNs, 1e-9, &flow.TimeFlowStartMs, 1e-3)
	fillTimeIfEmptyWithFallback(&flow.TimeFlowStartMs, &flow.TimeFlowStartNs, 1e-6, &flow.TimeFlowStart, 1e3)
	fillTimeIfEmptyWithFallback(&flow.TimeFlowStartNs, &flow.TimeFlowStartMs, 1e6, &flow.TimeFlowStart, 1e9)

	fillTimeIfEmpty(&flow.TimeReceived, &flow.TimeReceivedNs, 1e-9)
	fillTimeIfEmpty(&flow.TimeReceivedNs, &flow.TimeReceived, 1e9)
}

/**
** Fills time1 with the value of time2 using factor12 and time3 as fallback if time2 is empty
**/
func fillTimeIfEmptyWithFallback(time1 *uint64, time2 *uint64, factor12 float64, time3 *uint64, factor13 float64) {
	fillTimeIfEmpty(time1, time2, factor12)
	fillTimeIfEmpty(time1, time3, factor13)
}

/**
** Fills time1 with the value of time2 using factor12
**/
func fillTimeIfEmpty(time1 *uint64, time2 *uint64, factor12 float64) {
	if *time1 != 0 {
		return
	}
	if *time2 != 0 {
		*time1 = uint64(math.Round(float64(*time2) * factor12))
	}
}
