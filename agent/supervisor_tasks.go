package agent

import (
	"cepheus/agent/log"
	"cepheus/api"
	"context"
	"encoding/json"
	"fmt"
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

func (s *Supervisor) runOnce(ctx context.Context, rt *RunningTask) {
	params, err := rt.Spec.ParseParams()
	if err != nil {
		s.logger.ErrorContext(ctx, "error parsing param for task", "task_id", rt.Spec.TaskID)
		return
	}

	executor, ok := s.executors[rt.Spec.Type]
	if !ok {
		s.logger.ErrorContext(ctx, fmt.Sprintf("executor for %s not found", rt.Spec.Type))
		return
	}

	res, err := executor.Execute(ctx, params, rt.Spec)
	if err != nil {
		s.logger.ErrorContext(ctx, "error executing probe task", log.Err(err), "task_id", rt.Spec.TaskID)
		return
	}

	// Send to probe channel
	// s.sendProbeToStream(ctx, res, rt)
	// TODO: dont ignore buffer full error in the future
	_ = s.probeDataStream.Insert(ctx, res)

	data, err := json.Marshal(res)
	if err != nil {
		s.logger.ErrorContext(ctx, "couldn't marshal probe data", "task", rt.Spec.TaskID)
		return
	}

	s.logger.InfoContext(ctx, fmt.Sprintf("probe result type %s", rt.Spec.Type), "result", string(data))
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
