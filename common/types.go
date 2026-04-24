package common

import (
	"encoding/json"
	"time"
)

type ProbeResult struct {
	TaskID    string          `json:"task_id"`
	ProbeType ProbeType       `json:"probe_type"`
	Kind      string          `json:"kind"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type ReportPayload struct {
	Payload       ProbeResult `json:"payload"`
	SerialID      string      `json:"serial_id"`
	SentTimestamp time.Time   `json:"sent_timestamp"`
}

// Used to differentiate data from different probes in dispatcher.go
type ProbeType string

const (
	ProbeTypeStamp ProbeType = "stamp"
	ProbeTypeTrace ProbeType = "trace"
)

// STAMP Data
type StampData struct {
	Target   string  `json:"target"`
	Port     int     `json:"port"`
	Sent     int     `json:"sent"`
	Received int     `json:"received"`
	Loss     float64 `json:"loss"`
	AvgRTT   int64   `json:"avg_rtt"`
	MinRTT   int64   `json:"min_rtt"`
	MaxRTT   int64   `json:"max_rtt"`
	P50RTT   int64   `json:"p50_rtt"`
	P95RTT   int64   `json:"p95_rtt"`
}

// TraceRoute Data
type TraceProbeMethod string

const (
	TraceMethodICMPParis TraceProbeMethod = "icmp-paris"
	TraceMethodUDPParis  TraceProbeMethod = "udp-paris"
	TraceMethodTCP       TraceProbeMethod = "tcp"
)

type TraceProbeType string

const (
	TraceProbeTypeTrace   TraceProbeType = "trace"
	TraceProbeTypeTraceLb TraceProbeType = "tracelb"
)

type TraceData struct {
	Type   TraceProbeType   `json:"type"`
	Method TraceProbeMethod `json:"method"`
	Format string           `json:"format"`
	Data   json.RawMessage  `json:"data"` // TODO: Later make this into its own struct
}
