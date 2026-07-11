package agent

import (
	agentv1 "cepheus/libs/api/gen/cepheus/agent/v1"
	"cepheus/libs/common"
	"fmt"
)

// agentConfigFromProto maps the wire contract (cepheus.agent.v1) into the agent's
// own domain model. This is the agent's half of the contract mapping; the control
// plane owns the inverse (DB -> proto) on its side.
func agentConfigFromProto(resp *agentv1.GetAgentConfigResponse) (*AgentConfig, error) {
	if resp == nil || resp.GetConfig() == nil {
		return nil, fmt.Errorf("response carries no agent config")
	}
	pb := resp.GetConfig()

	cfg := &AgentConfig{
		ID:                    pb.GetId(),
		Generation:            int(pb.GetGeneration()),
		ReportEndpoint:        pb.GetReportEndpoint(),
		ReportBatchSize:       int(pb.GetReportBatchSize()),
		ReportIntervalSeconds: int(pb.GetReportIntervalSeconds()),
		ReportTimeoutSeconds:  int(pb.GetReportTimeoutSeconds()),
		UpdatedAt:             int(pb.GetUpdatedAt()),
		ScamperPPS:            int(pb.GetScamperPps()),
		Tasks:                 []Task{},
		PendingActions:        []PendingAction{},
	}

	for _, t := range pb.GetTasks() {
		task, err := taskFromProto(t)
		if err != nil {
			return nil, fmt.Errorf("task %s: %w", t.GetTaskId(), err)
		}
		cfg.Tasks = append(cfg.Tasks, task)
	}

	for _, a := range pb.GetPendingActions() {
		action, err := pendingActionFromProto(a)
		if err != nil {
			return nil, fmt.Errorf("pending action %s: %w", a.GetId(), err)
		}
		cfg.PendingActions = append(cfg.PendingActions, action)
	}

	return cfg, nil
}

func taskFromProto(t *agentv1.Task) (Task, error) {
	taskType, err := taskTypeFromProto(t.GetType())
	if err != nil {
		return Task{}, err
	}

	params, err := taskParamsFromProto(t)
	if err != nil {
		return Task{}, err
	}

	return Task{
		TaskID:     t.GetTaskId(),
		Type:       taskType,
		Enabled:    t.GetEnabled(),
		Generation: int(t.GetGeneration()),
		Params:     params,
		Schedule: TaskSchedule{
			Enabled:         t.GetSchedule().GetEnabled(),
			IntervalSeconds: int(t.GetSchedule().GetIntervalSeconds()),
			JitterPercent:   int(t.GetSchedule().GetJitterPercent()),
		},
	}, nil
}

func taskParamsFromProto(t *agentv1.Task) (TaskParams, error) {
	switch p := t.GetParams().(type) {
	case *agentv1.Task_Ping:
		v := p.Ping
		return &PingParams{
			Target:   v.GetTarget(),
			SourceIP: v.GetSourceIp(),
			Count:    int(v.GetCount()),
			Size:     int(v.GetSize()),
			Dscp:     int(v.GetDscp()),
			Timeout:  int(v.GetTimeoutSeconds()),
		}, nil
	case *agentv1.Task_Trace:
		v := p.Trace
		return &TraceParams{
			Target:      v.GetTarget(),
			SourceIP:    v.GetSourceIp(),
			Method:      common.TraceProbeMethod(v.GetMethod()),
			MaxTTL:      int(v.GetMaxTtl()),
			FirstTTL:    int(v.GetFirstTtl()),
			Dscp:        int(v.GetDscp()),
			WaitSeconds: int(v.GetWaitSeconds()),
		}, nil
	case *agentv1.Task_TraceLb:
		v := p.TraceLb
		return &TraceLbParams{
			Target:     v.GetTarget(),
			SourceIP:   v.GetSourceIp(),
			MaxTTL:     int(v.GetMaxTtl()),
			FirstTTL:   int(v.GetFirstTtl()),
			Dscp:       int(v.GetDscp()),
			Confidence: v.GetConfidence(),
		}, nil
	case *agentv1.Task_StampSender:
		v := p.StampSender
		return &StampSenderParams{
			Target:           v.GetTarget(),
			TargetPort:       int(v.GetTargetPort()),
			SourceIP:         v.GetSourceIp(),
			Dscp:             int(v.GetDscp()),
			RequireClockSync: v.GetRequireClockSync(),
			PacketCount:      int(v.GetPacketCount()),
			PacketInterval:   v.GetPacketInterval().AsDuration(),
		}, nil
	case *agentv1.Task_StampReflector:
		v := p.StampReflector
		return &StampReflectorParams{
			ListenPort:       int(v.GetListenPort()),
			SourceIP:         v.GetSourceIp(),
			Dscp:             int(v.GetDscp()),
			RequireClockSync: v.GetRequireClockSync(),
		}, nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("unhandled task params %T", t.GetParams())
	}
}

func pendingActionFromProto(a *agentv1.PendingAction) (PendingAction, error) {
	actionType, err := pendingActionTypeFromProto(a.GetType())
	if err != nil {
		return PendingAction{}, err
	}
	return PendingAction{
		ID:          a.GetId(),
		Type:        actionType,
		CreatedOn:   int(a.GetCreatedOn()),
		ScheduledOn: a.GetParams().GetScheduledOn().AsTime(),
	}, nil
}

func taskTypeFromProto(t agentv1.TaskType) (TaskType, error) {
	switch t {
	case agentv1.TaskType_TASK_TYPE_STAMP_SENDER:
		return TaskTypeStampSender, nil
	case agentv1.TaskType_TASK_TYPE_STAMP_REFLECTOR:
		return TaskTypeStampReflector, nil
	case agentv1.TaskType_TASK_TYPE_TRACE:
		return TaskTypeTrace, nil
	case agentv1.TaskType_TASK_TYPE_TRACE_LB:
		return TaskTypeTraceLb, nil
	case agentv1.TaskType_TASK_TYPE_PING:
		return TaskTypePing, nil
	default:
		return "", fmt.Errorf("unknown task type: %v", t)
	}
}

func pendingActionTypeFromProto(t agentv1.PendingActionType) (PendingActionType, error) {
	switch t {
	case agentv1.PendingActionType_PENDING_ACTION_TYPE_RESTART:
		return PendingActionRestart, nil
	default:
		return "", fmt.Errorf("unknown pending action type: %v", t)
	}
}
