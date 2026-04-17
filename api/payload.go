package api

import (
	"encoding/json"
	"fmt"
	"time"
)

type AgentTaskType string

const (
	TaskTypeStamp   AgentTaskType = "stamp"
	TaskTypeTrace   AgentTaskType = "trace"
	TaskTypeTraceLb AgentTaskType = "tracelb"
	TaskTypePing    AgentTaskType = "ping"
)

type AgentTaskSchedule struct {
	IntervalSeconds int `json:"interval_seconds"`
	JitterPercent   int `json:"jitter_percent"`
}

type AgentTaskPingParams struct {
	Target   string `json:"target"`
	SourceIP string `json:"source_ip,omitempty"`
	Count    int    `json:"count"`
	Size     int    `json:"size"`
	Dscp     int    `json:"dscp,omitempty"`
	Timeout  int    `json:"timeout_seconds,omitempty"`
}

type TraceProbeMethod string

const (
	TraceMethodICMPParis TraceProbeMethod = "icmp-paris"
	TraceMethodUDPParis  TraceProbeMethod = "udp-paris"
	TraceMethodTCP       TraceProbeMethod = "tcp"
)

type AgentTaskTraceParams struct {
	Target      string           `json:"target"`
	SourceIP string           `json:"source_ip,omitempty"`
	Method   TraceProbeMethod `json:"method"`
	MaxTTL   int              `json:"max_ttl,omitempty"`
	FirstTTL    int              `json:"first_ttl,omitempty"`
	Dscp        int              `json:"dscp,omitempty"`
	WaitSeconds int              `json:"wait_seconds,omitempty"`
}

type AgentTaskTraceLbParams struct {
	Target     string  `json:"target"`
	SourceIP   string  `json:"source_ip,omitempty"`
	MaxTTL     int     `json:"max_ttl,omitempty"`
	FirstTTL   int     `json:"first_ttl,omitempty"`
	Dscp       int     `json:"dscp,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type StampMode string

const (
	StampModeSender    StampMode = "sender"
	StampModeReflector StampMode = "reflector"
)

type AgentTaskStampParams struct {
	Mode   StampMode `json:"mode"`
	Target string    `json:"target"`
	TargetPort       int       `json:"target_port"`
	SourceIP         string    `json:"source_ip,omitempty"`
	Dscp             int       `json:"dscp,omitempty"`
	RequireClockSync bool      `json:"require_clock_sync"`
}

type Task struct {
	TaskID   string            `json:"task_id"`
	Type     AgentTaskType     `json:"type"`
	Enabled  bool              `json:"enabled"`
	Generation int               `json:"generation"`
	Params   json.RawMessage   `json:"params"`
	Schedule AgentTaskSchedule `json:"schedule"`
}

func (t *Task) ParseParams() (interface{}, error) {
	switch t.Type {
	case TaskTypePing:
		var p AgentTaskPingParams
		return &p, json.Unmarshal(t.Params, &p)
	case TaskTypeTrace:
		var p AgentTaskTraceParams
		return &p, json.Unmarshal(t.Params, &p)
	case TaskTypeTraceLb:
		var p AgentTaskTraceLbParams
		return &p, json.Unmarshal(t.Params, &p)
	case TaskTypeStamp:
		var p AgentTaskStampParams
		return &p, json.Unmarshal(t.Params, &p)
	default:
		return nil, fmt.Errorf("unknown task type: %s", t.Type)
	}
}

type PendingActionType string

const (
	PendingActionRestart PendingActionType = "restart"
)

type PendingActionBaseParams struct {
	ScheduledOn time.Time `json:"scheduled_on"`
}

type PendingAction struct {
	ID        string                  `json:"id"`
	Type      PendingActionType       `json:"type"`
	CreatedOn int                     `json:"created_on"`
	Params    PendingActionBaseParams `json:"params"`
}

type AgentConfig struct {
	ID                    string          `json:"id"`
	Generation            int             `json:"generation"`
	ReportEndpoint        string          `json:"report_endpoint"`
	ReportBatchSize       int             `json:"report_batch_size"`
	ReportIntervalSeconds int             `json:"report_interval_seconds"`
	PendingActions        []PendingAction `json:"pending_actions"`
	Tasks                 []Task          `json:"tasks"`
	UpdatedAt             int             `json:"updated_at"`
	ScamperPPS            int             `json:"scamper_pps"`
}
