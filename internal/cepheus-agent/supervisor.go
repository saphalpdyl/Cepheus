package cepheusagent

import (
	"cepheus/pkg/api"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type Supervisor struct {
	scamper *Scamper
	mu      sync.RWMutex

	tasks   map[string]api.Task
	running map[string]*RunningTask
	desired map[string]api.Task

	ctx context.Context

	logger *slog.Logger
}

type SupervisorConfig struct {
	Scamper *Scamper
	Ctx     context.Context
	Logger  *slog.Logger
}

func NewSupervisor(cfg SupervisorConfig) *Supervisor {
	return &Supervisor{
		scamper: cfg.Scamper,
		ctx:     cfg.Ctx,

		tasks:   make(map[string]api.Task),
		running: make(map[string]*RunningTask),
		desired: make(map[string]api.Task),
		logger:  cfg.Logger,
	}
}

func (s *Supervisor) startTask(spec *api.Task) *RunningTask {
	_, cancel := context.WithCancel(s.ctx)
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
	}()

	go s.startTaskLoop(s.ctx, rt)

	return rt
}

func (s *Supervisor) startTaskLoop(ctx context.Context, rt *RunningTask) {
	_, err := rt.Spec.ParseParams()
	if err != nil {
		s.logger.ErrorContext(ctx, "could not parse task params", "task_id", rt.Spec.TaskID)
		return
	}

	interval := time.Duration(rt.Spec.Schedule.IntervalSeconds) * time.Second
	if interval <= 0 {
		s.logger.ErrorContext(ctx, "interval less than zero", "interval", interval.Seconds(), "interval_raw", rt.Spec.Schedule.IntervalSeconds)
		return
	}

	jitter := computeJitter(interval, rt.Spec.Schedule.JitterPercent)

	select {
	case <-time.After(jitter):
	case <-ctx.Done():
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		// execute here
		s.logger.InfoContext(ctx, fmt.Sprintf("executed probe task: %s", rt.Spec.TaskID), "task_id", rt.Spec.TaskID)

		select {
		case <-ticker.C:
			// Next cycle
		case <-ctx.Done():
			return
		}
	}
}

func (s *Supervisor) SetDesiredTasks(tasks []api.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	desired := make(map[string]api.Task, len(tasks))
	for _, t := range tasks {
		desired[t.TaskID] = t
	}
	s.desired = desired

	s.logger.InfoContext(s.ctx, "Reconcilation starting...")
	s.reconcile(s.ctx)
}

func (s *Supervisor) reconcile(ctx context.Context) {
	// Stop removed tasks
	for id, _ := range s.running {
		if _, ok := s.desired[id]; !ok {
			// running.Stop()
			// delete(s.running, id)
		}
	}

	s.logger.InfoContext(ctx, "finished removing undesired tasks")

	for id, desired := range s.desired {
		running, exists := s.running[id]

		if !exists {
			// New task — start it
			rt := s.startTask(&desired)
			s.running[id] = rt
			continue
		}

		if running.Spec.Generation == desired.Generation {
			continue
		}

		// Generation changed — decide: hot update or restart
	}
}
