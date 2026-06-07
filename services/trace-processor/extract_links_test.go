package traceprocessor

import (
	"encoding/json"
	"testing"

	"cepheus/libs/common"
)

// mkPayload builds a TraceDataTracePayload from a JSON document. The Hops and
// NoHops fields are anonymous structs, so unmarshalling is the cleanest way to
// construct fixtures without re-declaring their types.
func mkPayload(t *testing.T, doc string) common.TraceDataTracePayload {
	t.Helper()
	var p common.TraceDataTracePayload
	if err := json.Unmarshal([]byte(doc), &p); err != nil {
		t.Fatalf("failed to unmarshal payload fixture: %v", err)
	}
	return p
}

// findLink returns the link matching the given probe/src/dst, or nil.
func findLink(links []TraceLink, probeID int, src, dst string) *TraceLink {
	for i := range links {
		l := links[i]
		if l.ProbeID == probeID && l.SrcIP == src && l.DstIP == dst {
			return &links[i]
		}
	}
	return nil
}

func TestExtractLinks_Empty(t *testing.T) {
	links := extractLinks(common.TraceDataTracePayload{})
	if len(links) != 0 {
		t.Fatalf("expected no links, got %d", len(links))
	}
}

func TestExtractLinks_SingleHopNoLink(t *testing.T) {
	// A probe with a single hop has no adjacent pair, so produces no link.
	p := mkPayload(t, `{
		"hops": [
			{"addr": "10.0.0.1", "probe_ttl": 1, "probe_id": 1, "rtt": 1.0, "reply_ttl": 64}
		]
	}`)

	links := extractLinks(p)
	if len(links) != 0 {
		t.Fatalf("expected no links for single hop, got %d", len(links))
	}
}

func TestExtractLinks_BasicChain(t *testing.T) {
	p := mkPayload(t, `{
		"hops": [
			{"addr": "10.0.0.1", "probe_ttl": 1, "probe_id": 1, "rtt": 1.0, "reply_ttl": 64, "icmp_code": 0},
			{"addr": "10.0.0.2", "probe_ttl": 2, "probe_id": 1, "rtt": 3.5, "reply_ttl": 63, "icmp_code": 0},
			{"addr": "10.0.0.3", "probe_ttl": 3, "probe_id": 1, "rtt": 5.0, "reply_ttl": 62, "icmp_code": 0}
		]
	}`)

	links := extractLinks(p)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d: %+v", len(links), links)
	}

	l1 := findLink(links, 1, "10.0.0.1", "10.0.0.2")
	if l1 == nil {
		t.Fatal("missing link 10.0.0.1 -> 10.0.0.2")
	}
	if l1.TTLGap != 1 {
		t.Errorf("TTLGap = %d, want 1", l1.TTLGap)
	}
	if !l1.IsSrcRespond || !l1.IsDstRespond {
		t.Errorf("expected both ends responding, got src=%v dst=%v", l1.IsSrcRespond, l1.IsDstRespond)
	}
	if l1.DiffRTT == nil {
		t.Fatal("expected non-nil DiffRTT")
	}
	if got := *l1.DiffRTT; got != 2.5 {
		t.Errorf("DiffRTT = %v, want 2.5", got)
	}
}

func TestExtractLinks_SortsByProbeTTL(t *testing.T) {
	// Hops arrive out of TTL order; links should still follow ascending TTL.
	p := mkPayload(t, `{
		"hops": [
			{"addr": "10.0.0.3", "probe_ttl": 3, "probe_id": 1, "rtt": 5.0, "reply_ttl": 62},
			{"addr": "10.0.0.1", "probe_ttl": 1, "probe_id": 1, "rtt": 1.0, "reply_ttl": 64},
			{"addr": "10.0.0.2", "probe_ttl": 2, "probe_id": 1, "rtt": 3.0, "reply_ttl": 63}
		]
	}`)

	links := extractLinks(p)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}

	if findLink(links, 1, "10.0.0.1", "10.0.0.2") == nil {
		t.Error("missing ordered link 10.0.0.1 -> 10.0.0.2")
	}
	if findLink(links, 1, "10.0.0.2", "10.0.0.3") == nil {
		t.Error("missing ordered link 10.0.0.2 -> 10.0.0.3")
	}
}

func TestExtractLinks_TimeoutHop(t *testing.T) {
	// Middle hop is a timeout (no_hops). Links touching it must mark the
	// unresponsive end and leave DiffRTT nil.
	p := mkPayload(t, `{
		"hops": [
			{"addr": "10.0.0.1", "probe_ttl": 1, "probe_id": 1, "rtt": 1.0, "reply_ttl": 64},
			{"addr": "10.0.0.3", "probe_ttl": 3, "probe_id": 1, "rtt": 5.0, "reply_ttl": 62}
		],
		"no_hops": [
			{"probe_ttl": 2, "probe_id": 1}
		]
	}`)

	links := extractLinks(p)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d: %+v", len(links), links)
	}

	// 10.0.0.1 -> Z (timeout, empty addr)
	toZ := findLink(links, 1, "10.0.0.1", "")
	if toZ == nil {
		t.Fatal("missing link into timeout hop")
	}
	if !toZ.IsSrcRespond {
		t.Error("expected src to respond")
	}
	if toZ.IsDstRespond {
		t.Error("expected dst (timeout) to not respond")
	}
	if toZ.DiffRTT != nil {
		t.Errorf("expected nil DiffRTT for timeout link, got %v", *toZ.DiffRTT)
	}

	// Z -> 10.0.0.3
	fromZ := findLink(links, 1, "", "10.0.0.3")
	if fromZ == nil {
		t.Fatal("missing link out of timeout hop")
	}
	if fromZ.IsSrcRespond {
		t.Error("expected src (timeout) to not respond")
	}
	if !fromZ.IsDstRespond {
		t.Error("expected dst to respond")
	}
	if fromZ.DiffRTT != nil {
		t.Errorf("expected nil DiffRTT for timeout link, got %v", *fromZ.DiffRTT)
	}
}

func TestExtractLinks_Deduplicates(t *testing.T) {
	// The same src->dst pair appears twice within one probe (e.g. repeated
	// attempts); only one link should be emitted.
	p := mkPayload(t, `{
		"hops": [
			{"addr": "10.0.0.1", "probe_ttl": 1, "probe_id": 1, "rtt": 1.0, "reply_ttl": 64},
			{"addr": "10.0.0.2", "probe_ttl": 2, "probe_id": 1, "rtt": 2.0, "reply_ttl": 63},
			{"addr": "10.0.0.1", "probe_ttl": 3, "probe_id": 1, "rtt": 1.1, "reply_ttl": 64},
			{"addr": "10.0.0.2", "probe_ttl": 4, "probe_id": 1, "rtt": 2.1, "reply_ttl": 63}
		]
	}`)

	links := extractLinks(p)

	count := 0
	for _, l := range links {
		if l.SrcIP == "10.0.0.1" && l.DstIP == "10.0.0.2" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 10.0.0.1 -> 10.0.0.2 to appear once, got %d (links: %+v)", count, links)
	}
}

func TestExtractLinks_MultipleProbesIndependent(t *testing.T) {
	// Two probes each linking the same addresses produce separate links keyed
	// per probe (dedup is scoped to probe ID).
	p := mkPayload(t, `{
		"hops": [
			{"addr": "10.0.0.1", "probe_ttl": 1, "probe_id": 1, "rtt": 1.0, "reply_ttl": 64},
			{"addr": "10.0.0.2", "probe_ttl": 2, "probe_id": 1, "rtt": 2.0, "reply_ttl": 63},
			{"addr": "10.0.0.1", "probe_ttl": 1, "probe_id": 2, "rtt": 1.0, "reply_ttl": 64},
			{"addr": "10.0.0.2", "probe_ttl": 2, "probe_id": 2, "rtt": 2.0, "reply_ttl": 63}
		]
	}`)

	links := extractLinks(p)
	if len(links) != 2 {
		t.Fatalf("expected 2 links across probes, got %d: %+v", len(links), links)
	}

	if findLink(links, 1, "10.0.0.1", "10.0.0.2") == nil {
		t.Error("missing link for probe 1")
	}
	if findLink(links, 2, "10.0.0.1", "10.0.0.2") == nil {
		t.Error("missing link for probe 2")
	}
}

func TestExtractLinks_TTLGap(t *testing.T) {
	// A non-contiguous TTL gap should be reflected in TTLGap.
	p := mkPayload(t, `{
		"hops": [
			{"addr": "10.0.0.1", "probe_ttl": 1, "probe_id": 1, "rtt": 1.0, "reply_ttl": 64},
			{"addr": "10.0.0.5", "probe_ttl": 5, "probe_id": 1, "rtt": 9.0, "reply_ttl": 60}
		]
	}`)

	links := extractLinks(p)
	l := findLink(links, 1, "10.0.0.1", "10.0.0.5")
	if l == nil {
		t.Fatal("missing link")
	}
	if l.TTLGap != 4 {
		t.Errorf("TTLGap = %d, want 4", l.TTLGap)
	}
}
