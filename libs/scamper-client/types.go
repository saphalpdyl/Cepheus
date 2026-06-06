package scamper_client

type ScamperClientConfig struct {
	BinPath    string
	SocketPath string
	PPS        uint32
	Window     uint32

	Format ScamperFormat
}

type ScamperFormat string

const (
	ScamperFormatJSON  ScamperFormat = "json"
	ScamperFormatWarts ScamperFormat = "warts"
	ScamperFormatText  ScamperFormat = "text"
)

type ScamperResult interface {
	ScamperResult()
}

// === Error results ===
type ErrorResult struct {
	Err     error
	Message string
}

// === Error results ===

// === Any results ===
type ReaderResult struct {
	// internal
	Data []byte
}

// === Any results ===

// === Traceroute results ===
type TraceHop struct {
	Addr string
	RTT  float64
	TTL  int
}

type TraceResult struct {
	Type string
	Src  string
	Dst  string
	Hops []TraceHop
}

func (t *TraceResult) ScamperResult() {}

func (h *TraceHop) ToMap() map[string]any {
	return map[string]any{
		"addr":      h.Addr,
		"rtt":       h.RTT,
		"probe_ttl": h.TTL,
	}
}

func (r *TraceResult) ToMap() map[string]any {
	hops := make([]map[string]any, len(r.Hops))
	for i := range r.Hops {
		hops[i] = r.Hops[i].ToMap()
	}
	return map[string]any{
		"type": r.Type,
		"src":  r.Src,
		"dst":  r.Dst,
		"hops": hops,
	}
}

// === Traceroute results ===
