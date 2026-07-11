package handler

import (
	agentv1 "cepheus/libs/api/gen/cepheus/agent/v1"
	"testing"
	"time"
)

// TestSetTaskParamsStampSender covers the trickiest jsonb -> proto decode: the
// stored packet_interval is time.Duration nanoseconds, which must become a
// protobuf Duration.
func TestSetTaskParamsStampSender(t *testing.T) {
	task := &agentv1.Task{}
	raw := []byte(`{"target":"2.2.2.2","target_port":862,"require_clock_sync":true,"packet_count":10,"packet_interval":500000000}`)

	if err := setTaskParams(task, taskTypeStampSender, raw); err != nil {
		t.Fatalf("setTaskParams: %v", err)
	}

	p := task.GetStampSender()
	if p == nil {
		t.Fatalf("stamp_sender oneof not set: %+v", task.GetParams())
	}
	if p.GetTarget() != "2.2.2.2" || p.GetTargetPort() != 862 || p.GetPacketCount() != 10 || !p.GetRequireClockSync() {
		t.Errorf("scalar mismatch: %+v", p)
	}
	if got := p.GetPacketInterval().AsDuration(); got != 500*time.Millisecond {
		t.Errorf("packet_interval: got %s want 500ms", got)
	}
}

func TestSetTaskParamsPing(t *testing.T) {
	task := &agentv1.Task{}
	raw := []byte(`{"target":"1.1.1.1","source_ip":"10.0.0.1","count":3,"size":64,"timeout_seconds":2}`)

	if err := setTaskParams(task, taskTypePing, raw); err != nil {
		t.Fatalf("setTaskParams: %v", err)
	}
	p := task.GetPing()
	if p == nil || p.GetTarget() != "1.1.1.1" || p.GetSourceIp() != "10.0.0.1" || p.GetCount() != 3 || p.GetTimeoutSeconds() != 2 {
		t.Errorf("ping decode mismatch: %+v", p)
	}
}

func TestTaskTypeToProto(t *testing.T) {
	cases := map[string]agentv1.TaskType{
		taskTypePing:           agentv1.TaskType_TASK_TYPE_PING,
		taskTypeTrace:          agentv1.TaskType_TASK_TYPE_TRACE,
		taskTypeTraceLb:        agentv1.TaskType_TASK_TYPE_TRACE_LB,
		taskTypeStampSender:    agentv1.TaskType_TASK_TYPE_STAMP_SENDER,
		taskTypeStampReflector: agentv1.TaskType_TASK_TYPE_STAMP_REFLECTOR,
		"bogus":                agentv1.TaskType_TASK_TYPE_UNSPECIFIED,
	}
	for in, want := range cases {
		if got := taskTypeToProto(in); got != want {
			t.Errorf("taskTypeToProto(%q): got %v want %v", in, got, want)
		}
	}
}
