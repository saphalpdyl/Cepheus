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
