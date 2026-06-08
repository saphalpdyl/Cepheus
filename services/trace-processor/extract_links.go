package traceprocessor

import (
	"fmt"
	"sort"

	"cepheus/libs/common"
)

type ParsedHop struct {
	Addr      string
	ProbeID   int
	ProbeTTL  int
	ReplyTTL  int
	IsTimeout bool
	IcmpCode  int
	RTT       float64
}

type TraceLink struct {
	ProbeID int
	SrcIP   *string
	DstIP   *string
	TTLGap  int

	// For link to/from a Z node ( unresponsive hop )
	IsSrcRespond bool
	IsDstRespond bool

	DiffRTT *float64 // nil if either end is a timeout
}

func extractLinks(payload common.TraceDataTracePayload) []TraceLink {
	groups := make(map[int][]ParsedHop)

	for _, hop := range payload.Hops {
		groups[hop.ProbeID] = append(groups[hop.ProbeID], ParsedHop{
			Addr:      hop.Addr,
			ProbeID:   hop.ProbeID,
			ProbeTTL:  hop.ProbeTTL,
			ReplyTTL:  hop.ReplyTTL,
			IcmpCode:  hop.IcmpCode,
			IsTimeout: false,
			RTT:       hop.Rtt,
		})
	}

	for _, hop := range payload.NoHops {
		groups[hop.ProbeID] = append(groups[hop.ProbeID], ParsedHop{
			Addr:      "",
			ProbeID:   hop.ProbeID,
			ProbeTTL:  hop.ProbeTTL,
			IsTimeout: true,
		})
	}

	for k, hops := range groups {
		hopsCopy := make([]ParsedHop, len(hops))
		copy(hopsCopy, hops)
		sort.Slice(hopsCopy, func(i, j int) bool {
			return hopsCopy[i].ProbeTTL < hopsCopy[j].ProbeTTL
		})
		groups[k] = hopsCopy
	}

	// Deduplication
	for k, hops := range groups {
		deduped := make([]ParsedHop, 0, len(hops))
		for i, h := range hops {
			if i == 0 || h.ProbeTTL != hops[i-1].ProbeTTL {
				deduped = append(deduped, h)
			}
		}

		groups[k] = deduped
	}

	seen := make(map[string]bool)
	var links []TraceLink

	for probeID, hops := range groups {
		for i := 1; i < len(hops); i++ {
			src, dst := hops[i-1], hops[i]

			link := TraceLink{
				ProbeID:      probeID,
				TTLGap:       dst.ProbeTTL - src.ProbeTTL,
				IsSrcRespond: !src.IsTimeout,
				IsDstRespond: !dst.IsTimeout,
			}

			if !src.IsTimeout {
				link.SrcIP = &src.Addr
			} 

			if !dst.IsTimeout {
				link.DstIP = &dst.Addr;
			}

			if !src.IsTimeout && !dst.IsTimeout {
				delta := dst.RTT - src.RTT
				link.DiffRTT = &delta
			}

			key := fmt.Sprintf("%d:%s->%s", probeID, src.Addr, dst.Addr)
			if seen[key] {
				continue
			}
			seen[key] = true

			links = append(links, link)
		}
	}

	return links
}
