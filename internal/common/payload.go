package common

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
	AgentTaskSchedule AgentTaskSchedule `json:"schedule"`
}

type StampMode string

const (
	StampModeSender    StampMode = "sender"
	StampModeReflector StampMode = "reflector"
)

type AgentTaskStampParams struct {
	AgentTaskParams

	Mode       StampMode `json:"mode"`
	Target     string    `json:"target"`
	TargetPort int       `json:"target_port"`

	// the source and source port are managed by the agent's own control plane
}

type AgentTask struct {
	TaskID  string        `json:"task_id"`
	Type    AgentTaskType `json:"type"`
	Enabled bool          `json:"enabled"`

	Params json.RawMessage `json:"params"` // Will be unmarshalled into specific params at runtime
}

type AgentConfig struct {
	Version    int   `json:"version"`
	Generation int64 `json:"generation"`

	ReportEndpoint        string `json:"report_endpoint"`
	ReportBatchSize       int    `json:"report_batch_size"`
	ReportIntervalSeconds int    `json:"report_interval_seconds"`

	Tasks []AgentTask `json:"tasks"`

	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}
