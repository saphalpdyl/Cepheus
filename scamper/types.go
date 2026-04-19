package scamper

import (
	"bufio"
	"log/slog"
	"net"
	"os/exec"
	"sync"
)

type Scamper struct {
	BinPath    string
	SocketPath string
	PPS        int
	Cmd        *exec.Cmd
	Conn       net.Conn
	mu         sync.Mutex
	scanner    *bufio.Scanner

	logger *slog.Logger
}

type TraceHop struct {
	Addr string  `json:"addr"`
	RTT  float64 `json:"rtt"`
	TTL  int     `json:"probe_ttl"`
}

type TraceResult struct {
	Type string     `json:"type"`
	Src  string     `json:"src"`
	Dst  string     `json:"dst"`
	Hops []TraceHop `json:"hops"`
}

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
