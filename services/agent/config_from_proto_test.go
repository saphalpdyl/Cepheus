package agent

import (
	agentv1 "cepheus/libs/api/gen/cepheus/agent/v1"
	"cepheus/libs/common"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestAgentConfigFromProto(t *testing.T) {
	resp := &agentv1.GetAgentConfigResponse{
		Config: &agentv1.AgentConfig{
			Id:                    "cfg-1",
			Generation:            7,
			ReportEndpoint:        "nats://broker:4222",
			ReportBatchSize:       10,
			ReportIntervalSeconds: 30,
			ReportTimeoutSeconds:  5,
			UpdatedAt:             1700000000,
			ScamperPps:            100,
			Tasks: []*agentv1.Task{
				{
					TaskId:     "t-ping",
					Type:       agentv1.TaskType_TASK_TYPE_PING,
					Enabled:    true,
					Generation: 1,
					Schedule:   &agentv1.TaskSchedule{Enabled: true, IntervalSeconds: 60, JitterPercent: 10},
					Params:     &agentv1.Task_Ping{Ping: &agentv1.PingParams{Target: "1.1.1.1", SourceIp: "10.0.0.1", Count: 3, Size: 64, TimeoutSeconds: 2}},
				},
				{
					TaskId: "t-trace",
					Type:   agentv1.TaskType_TASK_TYPE_TRACE,
					Params: &agentv1.Task_Trace{Trace: &agentv1.TraceParams{Target: "8.8.8.8", Method: "udp-paris", MaxTtl: 30, FirstTtl: 1}},
				},
				{
					TaskId: "t-stamp-sender",
					Type:   agentv1.TaskType_TASK_TYPE_STAMP_SENDER,
					Params: &agentv1.Task_StampSender{StampSender: &agentv1.StampSenderParams{Target: "2.2.2.2", TargetPort: 862, RequireClockSync: true, PacketCount: 10, PacketInterval: durationpb.New(500 * time.Millisecond)}},
				},
			},
			PendingActions: []*agentv1.PendingAction{
				{Id: "pa-1", Type: agentv1.PendingActionType_PENDING_ACTION_TYPE_RESTART, CreatedOn: 1700000001, Params: &agentv1.PendingActionParams{ScheduledOn: timestamppb.New(time.Unix(1700000100, 0).UTC())}},
			},
		},
	}

	cfg, err := agentConfigFromProto(resp)
	if err != nil {
		t.Fatalf("agentConfigFromProto: %v", err)
	}

	if cfg.ID != "cfg-1" || cfg.Generation != 7 || cfg.ReportEndpoint != "nats://broker:4222" ||
		cfg.ReportBatchSize != 10 || cfg.ReportIntervalSeconds != 30 || cfg.ReportTimeoutSeconds != 5 ||
		cfg.UpdatedAt != 1700000000 || cfg.ScamperPPS != 100 {
		t.Fatalf("config scalar mismatch: %+v", cfg)
	}

	if len(cfg.Tasks) != 3 {
		t.Fatalf("tasks: got %d want 3", len(cfg.Tasks))
	}

	ping := cfg.Tasks[0]
	if ping.Type != TaskTypePing || !ping.Enabled || ping.Schedule != (TaskSchedule{Enabled: true, IntervalSeconds: 60, JitterPercent: 10}) {
		t.Errorf("ping task header mismatch: %+v", ping)
	}
	pp, ok := ping.Params.(*PingParams)
	if !ok {
		t.Fatalf("ping params: got %T", ping.Params)
	}
	if *pp != (PingParams{Target: "1.1.1.1", SourceIP: "10.0.0.1", Count: 3, Size: 64, Timeout: 2}) {
		t.Errorf("ping params mismatch: %+v", pp)
	}

	tp, ok := cfg.Tasks[1].Params.(*TraceParams)
	if !ok {
		t.Fatalf("trace params: got %T", cfg.Tasks[1].Params)
	}
	if tp.Method != common.TraceMethodUDPParis || tp.Target != "8.8.8.8" || tp.MaxTTL != 30 {
		t.Errorf("trace params mismatch: %+v", tp)
	}

	sp, ok := cfg.Tasks[2].Params.(*StampSenderParams)
	if !ok {
		t.Fatalf("stamp-sender params: got %T", cfg.Tasks[2].Params)
	}
	if sp.PacketInterval != 500*time.Millisecond || sp.TargetPort != 862 || sp.PacketCount != 10 || !sp.RequireClockSync {
		t.Errorf("stamp-sender params mismatch: %+v", sp)
	}

	if len(cfg.PendingActions) != 1 {
		t.Fatalf("pending actions: got %d want 1", len(cfg.PendingActions))
	}
	pa := cfg.PendingActions[0]
	if pa.Type != PendingActionRestart || !pa.ScheduledOn.Equal(time.Unix(1700000100, 0).UTC()) {
		t.Errorf("pending action mismatch: %+v", pa)
	}
}
