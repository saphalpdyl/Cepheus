package cepheusagent

import (
	"cepheus/api"
	"cepheus/cepheus-agent/log"
	"context"
	"encoding/json"
	"time"
)

func (s *Supervisor) SetDesiredTasks(tasks []api.Task) {

	desired := make(map[string]api.Task, len(tasks))
	for _, t := range tasks {
		desired[t.TaskID] = t
	}

	s.mu.Lock()
	s.desired = desired
	s.mu.Unlock()

	s.logger.InfoContext(s.ctx, "Reconcilation starting...")
	s.reconcile(s.ctx)
}

func (s *Supervisor) sendProbeToStream(ctx context.Context, data api.ProbeResult, task *RunningTask) {
	select {
	case s.probeDataStream <- data:
	case <-ctx.Done():
		return
	default:
		s.logger.WarnContext(ctx, "probe buffer full, dropping result", "task_id", task.Spec.TaskID)
	}
}

func (s *Supervisor) runOnce(ctx context.Context, rt *RunningTask) {
	params, err := rt.Spec.ParseParams()
	if err != nil {
		s.logger.ErrorContext(ctx, "error parsing param for task", "task_id", rt.Spec.TaskID)
		return
	}

	res, err := s.executors[rt.Spec.Type].Execute(ctx, params, rt.Spec)
	if err != nil {
		s.logger.ErrorContext(ctx, "error executing probe task", log.Err(err), "task_id", rt.Spec.TaskID)
		return
	}

	// Send to probe channel
	s.sendProbeToStream(ctx, res, rt)

	data, err := json.Marshal(res)
	s.logger.InfoContext(ctx, "probe result", "result", string(data))
}

func (s *Supervisor) startTask(spec *api.Task) *RunningTask {
	ctx, cancel := context.WithCancel(s.ctx)
	done := make(chan struct{})

	rt := &RunningTask{
		Spec:      spec,
		cancel:    cancel,
		done:      done,
		startedAt: time.Now(),
		errors:    0,
	}

	go func() {
		defer close(done)
		s.startTaskLoop(ctx, rt)
	}()

	return rt
}

func (s *Supervisor) startTaskLoop(ctx context.Context, rt *RunningTask) {
	_, err := rt.Spec.ParseParams()
	if err != nil {
		s.logger.ErrorContext(ctx, "could not parse task params", "task_id", rt.Spec.TaskID)
		return
	}

	interval := time.Duration(rt.Spec.Schedule.IntervalSeconds) * time.Second
	if interval < 0 {
		s.logger.ErrorContext(ctx, "interval less than zero", "interval", interval.Seconds(), "interval_raw", rt.Spec.Schedule.IntervalSeconds)
		return
	}

	// RunOnce without intervals
	if rt.Spec.Schedule.Enabled == false {
		s.runOnce(ctx, rt)
		return
	}

	// Interval-based task
	jitter := computeJitter(interval, rt.Spec.Schedule.JitterPercent)

	select {
	case <-time.After(jitter):
		s.runOnce(ctx, rt)
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Next cycle
			s.runOnce(ctx, rt)
		case <-ctx.Done():
			s.logger.InfoContext(ctx, "closing task loop", "task_id", rt.Spec.TaskID)
			return
		}

	}
}
