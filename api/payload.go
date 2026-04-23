package api

import (
	"encoding/json"
	"fmt"
	"time"
)

type TaskParams interface {
	GetTaskName() string
}

type AgentTaskType string

const (
	TaskTypeStampSender    AgentTaskType = "stamp-sender"
	TaskTypeStampReflector AgentTaskType = "stamp-reflector"
	TaskTypeTrace          AgentTaskType = "trace"
	TaskTypeTraceLb        AgentTaskType = "tracelb"
	TaskTypePing           AgentTaskType = "ping"
)

// -------------------------------

type AgentTaskSchedule struct {
	Enabled         bool `json:"enabled"`
	IntervalSeconds int  `json:"interval_seconds"`
	JitterPercent   int  `json:"jitter_percent"`
}

type AgentTaskPingParams struct {
	Target   string `json:"target"`
	SourceIP string `json:"source_ip,omitempty"`
	Count    int    `json:"count"`
	Size     int    `json:"size"`
	Dscp     int    `json:"dscp,omitempty"`
	Timeout  int    `json:"timeout_seconds,omitempty"`
}

// -------------------------------

type TraceProbeMethod string

const (
	TraceMethodICMPParis TraceProbeMethod = "icmp-paris"
	TraceMethodUDPParis  TraceProbeMethod = "udp-paris"
	TraceMethodTCP       TraceProbeMethod = "tcp"
)

type AgentTaskTraceParams struct {
	Target      string           `json:"target"`
	SourceIP    string           `json:"source_ip,omitempty"`
	Method      TraceProbeMethod `json:"method"`
	MaxTTL      int              `json:"max_ttl,omitempty"`
	FirstTTL    int              `json:"first_ttl,omitempty"`
	Dscp        int              `json:"dscp,omitempty"`
	WaitSeconds int              `json:"wait_seconds,omitempty"`
}

// -------------------------------

type AgentTaskTraceLbParams struct {
	Target     string  `json:"target"`
	SourceIP   string  `json:"source_ip,omitempty"`
	MaxTTL     int     `json:"max_ttl,omitempty"`
	FirstTTL   int     `json:"first_ttl,omitempty"`
	Dscp       int     `json:"dscp,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// -------------------------------

type AgentTaskStampSenderParams struct {
	Target           string        `json:"target"`
	TargetPort       int           `json:"target_port"`
	SourceIP         string        `json:"source_ip,omitempty"`
	Dscp             int           `json:"dscp,omitempty"`
	RequireClockSync bool          `json:"require_clock_sync"`
	PacketCount      int           `json:"packet_count,omitempty"`
	PacketInterval   time.Duration `json:"packet_interval,omitempty"`
}

type AgentTaskStampReflectorParams struct {
	ListenPort       int    `json:"listen_port"`
	SourceIP         string `json:"source_ip,omitempty"`
	Dscp             int    `json:"dscp,omitempty"`
	RequireClockSync bool   `json:"require_clock_sync"`
}

type Task struct {
	TaskID     string            `json:"task_id"`
	Type       AgentTaskType     `json:"type"`
	Enabled    bool              `json:"enabled"`
	Generation int               `json:"generation"`
	Params     json.RawMessage   `json:"params"`
	Schedule   AgentTaskSchedule `json:"schedule"`
}

func (p *AgentTaskPingParams) GetTaskName() string           { return string(TaskTypePing) }
func (p *AgentTaskTraceParams) GetTaskName() string          { return string(TaskTypeTrace) }
func (p *AgentTaskTraceLbParams) GetTaskName() string        { return string(TaskTypeTraceLb) }
func (p *AgentTaskStampSenderParams) GetTaskName() string    { return string(TaskTypeStampSender) }
func (p *AgentTaskStampReflectorParams) GetTaskName() string { return string(TaskTypeStampReflector) }

func (t *Task) ParseParams() (TaskParams, error) {
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
	case TaskTypeStampSender:
		var p AgentTaskStampSenderParams
		return &p, json.Unmarshal(t.Params, &p)
	case TaskTypeStampReflector:
		var p AgentTaskStampReflectorParams
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
	ReportTimeoutSeconds  int             `json:"report_timeout_seconds"`
	PendingActions        []PendingAction `json:"pending_actions"`
	Tasks                 []Task          `json:"tasks"`
	UpdatedAt             int             `json:"updated_at"`
	ScamperPPS            int             `json:"scamper_pps"`
}

type ProbeResult struct {
	TaskID    string         `json:"task_id"`
	Kind      string         `json:"kind"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
}
