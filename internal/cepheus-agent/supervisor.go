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

	return rt
}

func (s *Supervisor) startTaskLoop(ctx context.Context, rt *RunningTask) {
	_, err := rt.Spec.ParseParams()
	if err != nil {
		s.logger.ErrorContext(ctx, "could not parse task params", "task_id", rt.Spec.TaskID)
		return
	}

	interval := time.Duration(rt.Spec.Schedule.IntervalSeconds)
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

	// Signal the reconcile loop, don't act here
	// s.reconcileNotify()
	s.reconcile()
}

func (s *Supervisor) reconcile() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop removed tasks
	for id, _ := range s.running {
		if _, ok := s.desired[id]; !ok {
			// running.Stop()
			// delete(s.running, id)
		}
	}

	for id, desired := range s.desired {
		running, exists := s.running[id]

		if !exists {
			// New task — start it
			s.running[id] = s.startTask(&desired)
			continue
		}

		if running.Spec.Generation == desired.Generation {
			continue
		}

		// Generation changed — decide: hot update or restart
	}
}
