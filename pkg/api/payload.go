package api

import "encoding/json"

type AgentTaskType string

const (
	TaskTypeStamp   AgentTaskType = "stamp"
	TaskTypeTrace   AgentTaskType = "trace"
	TaskTypeRestart AgentTaskType = "restart" // Highest precedence, should be executed immediately
)

type AgentTaskSchedule struct {
	IntervalSeconds int `json:"interval_seconds"`
	JitterPercent   int `json:"jitter_percent"`
}

type AgentTaskParams struct {
	Schedule AgentTaskSchedule `json:"schedule"`
}

type StampMode string

const (
	StampModeSender    StampMode = "sender"
	StampModeReflector StampMode = "reflector"
)

type AgentTaskStampParams struct {
	AgentTaskParams

	Mode             StampMode `json:"mode"`
	Target           string    `json:"target"`
	TargetPort       int       `json:"target_port"`
	SourceIP         string    `json:"source_ip"`
	Dscp             int       `json:"dscp"`
	RequireClockSync bool      `json:"require_clock_sync"`
}

type Task struct {
	TaskID     string          `json:"task_id"`
	Type       AgentTaskType   `json:"type"`
	Enabled    bool            `json:"enabled"`
	Generation int             `json:"generation"`
	Params     json.RawMessage `json:"params"`
}

type PendingAction struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	CreatedOn int    `json:"created_on"`
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
}
