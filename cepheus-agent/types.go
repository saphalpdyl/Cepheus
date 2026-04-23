package cepheusagent

import (
	"cepheus/api"
	"time"
)

type ReportPayload struct {
	Payload       api.ProbeResult `json:"payload"`
	SerialID      string          `json:"serial_id"`
	SentTimestamp time.Time       `json:"sent_timestamp"`
}
