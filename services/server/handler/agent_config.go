package handler

import (
	agentv1 "cepheus/libs/api/gen/cepheus/agent/v1"
	"cepheus/libs/telemetry"
	logattr "cepheus/services/server/log"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
)

// errAgentNotFound is the internal sentinel for "no device matches this serial".
// It is translated to connect.CodeNotFound at the RPC boundary.
var errAgentNotFound = errors.New("agent not found")

// DB task.type values. The control plane owns its own mapping from the stored
// representation to the cepheus.agent.v1 wire contract (the only thing shared
// across services is the generated proto, not a domain-types package).
const (
	taskTypePing           = "ping"
	taskTypeTrace          = "trace"
	taskTypeTraceLb        = "tracelb"
	taskTypeStampSender    = "stamp-sender"
	taskTypeStampReflector = "stamp-reflector"
)

// GetAgentConfig implements agentv1connect.AgentConfigServiceHandler.
func (h *Handler) GetAgentConfig(
	ctx context.Context,
	req *connect.Request[agentv1.GetAgentConfigRequest],
) (resp *connect.Response[agentv1.GetAgentConfigResponse], err error) {
	ctx, end, _ := telemetry.SpanWithErr(ctx, "Handler.GetAgentConfig", &err)
	defer end()

	serialID := req.Msg.GetSerialId()
	if serialID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("missing serial_id"))
	}

	log().Info("fetching agent config", "serial_id", serialID)

	cfg, err := h.loadAgentConfig(ctx, serialID)
	if err != nil {
		if errors.Is(err, errAgentNotFound) {
			log().Warn("agent not found", "serial_id", serialID)
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("agent %q not found", serialID))
		}
		log().Error("failed to load agent config", "serial_id", serialID, logattr.Err(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&agentv1.GetAgentConfigResponse{Config: cfg}), nil
}

// loadAgentConfig reads a device's desired configuration from Postgres and
// assembles it directly into the wire message. Returns errAgentNotFound if no
// device matches the serial id.
func (h *Handler) loadAgentConfig(ctx context.Context, serialID string) (*agentv1.AgentConfig, error) {
	rows, err := h.Pool.Query(ctx,
		`SELECT c.id, c.generation,
		        c.report_endpoint, c.report_batch_size, c.report_interval_seconds, c.report_timeout_seconds,
		        EXTRACT(EPOCH FROM c.updated_at)::bigint,
		        t.task_id, t.type, t.enabled, t.generation, t.params,
		        t.schedule_interval_seconds, t.schedule_jitter_percent, schedule_enabled
		 FROM device d
		 JOIN agent_config c ON c.id = d.agent_config_id
		 LEFT JOIN agent_task t ON t.agent_config_id = c.id
		 WHERE d.serial_id = $1`,
		serialID,
	)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	cfg := &agentv1.AgentConfig{}
	found := false
	for rows.Next() {
		var (
			generation, reportBatch, reportInterval, reportTimeout int
			updatedAt                                              int64

			taskID           *string
			taskType         *string
			taskEnabled      *bool
			taskGeneration   *int
			taskParams       *json.RawMessage
			scheduleInterval *int
			scheduleJitter   *int
			scheduleEnabled  *bool
		)

		if err = rows.Scan(
			&cfg.Id, &generation,
			&cfg.ReportEndpoint, &reportBatch, &reportInterval, &reportTimeout,
			&updatedAt,
			&taskID, &taskType, &taskEnabled, &taskGeneration, &taskParams,
			&scheduleInterval, &scheduleJitter, &scheduleEnabled,
		); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		found = true

		cfg.Generation = int32(generation)
		cfg.ReportBatchSize = int32(reportBatch)
		cfg.ReportIntervalSeconds = int32(reportInterval)
		cfg.ReportTimeoutSeconds = int32(reportTimeout)
		cfg.UpdatedAt = updatedAt

		if taskID == nil {
			continue
		}

		var typeStr string
		if taskType != nil {
			typeStr = *taskType
		}

		task := &agentv1.Task{
			TaskId:   *taskID,
			Type:     taskTypeToProto(typeStr),
			Schedule: &agentv1.TaskSchedule{},
		}
		if taskEnabled != nil {
			task.Enabled = *taskEnabled
		}
		if taskGeneration != nil {
			task.Generation = int32(*taskGeneration)
		}
		if scheduleInterval != nil {
			task.Schedule.IntervalSeconds = int32(*scheduleInterval)
		}
		if scheduleJitter != nil {
			task.Schedule.JitterPercent = int32(*scheduleJitter)
		}
		if scheduleEnabled != nil {
			task.Schedule.Enabled = *scheduleEnabled
		}

		if taskParams != nil && len(*taskParams) > 0 {
			if err := setTaskParams(task, typeStr, *taskParams); err != nil {
				return nil, fmt.Errorf("task %s params: %w", *taskID, err)
			}
		}

		cfg.Tasks = append(cfg.Tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}

	if !found {
		return nil, errAgentNotFound
	}

	return cfg, nil
}

func taskTypeToProto(taskType string) agentv1.TaskType {
	switch taskType {
	case taskTypePing:
		return agentv1.TaskType_TASK_TYPE_PING
	case taskTypeTrace:
		return agentv1.TaskType_TASK_TYPE_TRACE
	case taskTypeTraceLb:
		return agentv1.TaskType_TASK_TYPE_TRACE_LB
	case taskTypeStampSender:
		return agentv1.TaskType_TASK_TYPE_STAMP_SENDER
	case taskTypeStampReflector:
		return agentv1.TaskType_TASK_TYPE_STAMP_REFLECTOR
	default:
		return agentv1.TaskType_TASK_TYPE_UNSPECIFIED
	}
}

// setTaskParams decodes the stored params jsonb into the proto oneof for the
// task's type. The local structs mirror the jsonb shape written by the schema.
func setTaskParams(task *agentv1.Task, taskType string, raw json.RawMessage) error {
	switch taskType {
	case taskTypePing:
		var p struct {
			Target   string `json:"target"`
			SourceIP string `json:"source_ip"`
			Count    int    `json:"count"`
			Size     int    `json:"size"`
			Dscp     int    `json:"dscp"`
			Timeout  int    `json:"timeout_seconds"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		task.Params = &agentv1.Task_Ping{Ping: &agentv1.PingParams{
			Target:         p.Target,
			SourceIp:       p.SourceIP,
			Count:          int32(p.Count),
			Size:           int32(p.Size),
			Dscp:           int32(p.Dscp),
			TimeoutSeconds: int32(p.Timeout),
		}}
	case taskTypeTrace:
		var p struct {
			Target      string `json:"target"`
			SourceIP    string `json:"source_ip"`
			Method      string `json:"method"`
			MaxTTL      int    `json:"max_ttl"`
			FirstTTL    int    `json:"first_ttl"`
			Dscp        int    `json:"dscp"`
			WaitSeconds int    `json:"wait_seconds"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		task.Params = &agentv1.Task_Trace{Trace: &agentv1.TraceParams{
			Target:      p.Target,
			SourceIp:    p.SourceIP,
			Method:      p.Method,
			MaxTtl:      int32(p.MaxTTL),
			FirstTtl:    int32(p.FirstTTL),
			Dscp:        int32(p.Dscp),
			WaitSeconds: int32(p.WaitSeconds),
		}}
	case taskTypeTraceLb:
		var p struct {
			Target     string  `json:"target"`
			SourceIP   string  `json:"source_ip"`
			MaxTTL     int     `json:"max_ttl"`
			FirstTTL   int     `json:"first_ttl"`
			Dscp       int     `json:"dscp"`
			Confidence float64 `json:"confidence"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		task.Params = &agentv1.Task_TraceLb{TraceLb: &agentv1.TraceLbParams{
			Target:     p.Target,
			SourceIp:   p.SourceIP,
			MaxTtl:     int32(p.MaxTTL),
			FirstTtl:   int32(p.FirstTTL),
			Dscp:       int32(p.Dscp),
			Confidence: p.Confidence,
		}}
	case taskTypeStampSender:
		var p struct {
			Target           string        `json:"target"`
			TargetPort       int           `json:"target_port"`
			SourceIP         string        `json:"source_ip"`
			Dscp             int           `json:"dscp"`
			RequireClockSync bool          `json:"require_clock_sync"`
			PacketCount      int           `json:"packet_count"`
			PacketInterval   time.Duration `json:"packet_interval"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		task.Params = &agentv1.Task_StampSender{StampSender: &agentv1.StampSenderParams{
			Target:           p.Target,
			TargetPort:       int32(p.TargetPort),
			SourceIp:         p.SourceIP,
			Dscp:             int32(p.Dscp),
			RequireClockSync: p.RequireClockSync,
			PacketCount:      int32(p.PacketCount),
			PacketInterval:   durationpb.New(p.PacketInterval),
		}}
	case taskTypeStampReflector:
		var p struct {
			ListenPort       int    `json:"listen_port"`
			SourceIP         string `json:"source_ip"`
			Dscp             int    `json:"dscp"`
			RequireClockSync bool   `json:"require_clock_sync"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		task.Params = &agentv1.Task_StampReflector{StampReflector: &agentv1.StampReflectorParams{
			ListenPort:       int32(p.ListenPort),
			SourceIp:         p.SourceIP,
			Dscp:             int32(p.Dscp),
			RequireClockSync: p.RequireClockSync,
		}}
	default:
		return fmt.Errorf("unknown task type %q", taskType)
	}
	return nil
}
