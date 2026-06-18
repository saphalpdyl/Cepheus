package agent

// Domain types the agent operates on. These are intentionally owned by the agent
// (not shared across services): the wire contract is the generated protobuf in
// libs/api/gen, and each service maps that contract to/from its own model. See
// config_from_proto.go for the agentv1 -> domain mapping.

import (
	"cepheus/libs/common"
	"time"
)

type TaskType string

const (
	TaskTypeStampSender    TaskType = "stamp-sender"
	TaskTypeStampReflector TaskType = "stamp-reflector"
	TaskTypeTrace          TaskType = "trace"
	TaskTypeTraceLb        TaskType = "tracelb"
	TaskTypePing           TaskType = "ping"
)

// TaskParams is the sealed set of per-task-type parameter payloads. Executors
// type-assert to the concrete variant they expect.
type TaskParams interface {
	isTaskParams()
}

type PingParams struct {
	Target   string
	SourceIP string
	Count    int
	Size     int
	Dscp     int
	Timeout  int
}

type TraceParams struct {
	Target      string
	SourceIP    string
	Method      common.TraceProbeMethod
	MaxTTL      int
	FirstTTL    int
	Dscp        int
	WaitSeconds int
}

type TraceLbParams struct {
	Target     string
	SourceIP   string
	MaxTTL     int
	FirstTTL   int
	Dscp       int
	Confidence float64
}

type StampSenderParams struct {
	Target           string
	TargetPort       int
	SourceIP         string
	Dscp             int
	RequireClockSync bool
	PacketCount      int
	PacketInterval   time.Duration
}

type StampReflectorParams struct {
	ListenPort       int
	SourceIP         string
	Dscp             int
	RequireClockSync bool
}

func (*PingParams) isTaskParams()           {}
func (*TraceParams) isTaskParams()          {}
func (*TraceLbParams) isTaskParams()        {}
func (*StampSenderParams) isTaskParams()    {}
func (*StampReflectorParams) isTaskParams() {}

type TaskSchedule struct {
	Enabled         bool
	IntervalSeconds int
	JitterPercent   int
}

// Task is a fully-resolved unit of work for an executor. Params is already the
// typed variant for Type (mapped from the protobuf oneof), so there is no
// runtime re-parse.
type Task struct {
	TaskID     string
	Type       TaskType
	Enabled    bool
	Generation int
	Params     TaskParams
	Schedule   TaskSchedule
}

type PendingActionType string

const (
	PendingActionRestart PendingActionType = "restart"
)

type PendingAction struct {
	ID          string
	Type        PendingActionType
	CreatedOn   int
	ScheduledOn time.Time
}

type AgentConfig struct {
	ID                    string
	Generation            int
	ReportEndpoint        string
	ReportBatchSize       int
	ReportIntervalSeconds int
	ReportTimeoutSeconds  int
	PendingActions        []PendingAction
	Tasks                 []Task
	UpdatedAt             int
	ScamperPPS            int
}
