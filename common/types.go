package common

import (
	"time"
)

type ProbeResult struct {
	TaskID    string         `json:"task_id"`
	ProbeType ProbeType      `json:"probe_type"`
	Kind      string         `json:"kind"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
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
	AvgRTT   float64 `json:"avg_rtt"`
	MinRTT   float64 `json:"min_rtt"`
	MaxRTT   float64 `json:"max_rtt"`
	P50RTT   float64 `json:"p50_rtt"`
	P95RTT   float64 `json:"p95_rtt"`
}
