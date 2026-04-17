package cepheusagent

import (
	"cepheus/api"
	"cepheus/cepheus-agent/log"
	"context"
	"encoding/json"
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

	executors map[api.AgentTaskType]Executor
}

type SupervisorConfig struct {
	Scamper   *Scamper
	Ctx       context.Context
	Logger    *slog.Logger
	Executors map[api.AgentTaskType]Executor
}

func NewSupervisor(cfg SupervisorConfig) *Supervisor {
	return &Supervisor{
		scamper: cfg.Scamper,
		ctx:     cfg.Ctx,

		tasks:   make(map[string]api.Task),
		running: make(map[string]*RunningTask),
		desired: make(map[string]api.Task),
		logger:  cfg.Logger,

		executors: cfg.Executors,
	}
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

	// TODO: Extract this to a manageable place
	executeTask := func() {
		params, err := rt.Spec.ParseParams()
		if err != nil {
			s.logger.ErrorContext(ctx, "error parsing param for task", "task_id", rt.Spec.TaskID)
		}

		res, err := s.executors[rt.Spec.Type].Execute(ctx, params)
		if err != nil {
			s.logger.ErrorContext(ctx, "error executing probe task", log.Err(err), "task_id", rt.Spec.TaskID)
		}

		// TODO: ProbeResult should be fed into the probeDataBuffer chan
		data, err := json.Marshal(res)
		s.logger.InfoContext(ctx, "probe result", "result", string(data))
	}

	interval := time.Duration(rt.Spec.Schedule.IntervalSeconds) * time.Second
	if interval < 0 {
		s.logger.ErrorContext(ctx, "interval less than zero", "interval", interval.Seconds(), "interval_raw", rt.Spec.Schedule.IntervalSeconds)
		return
	} else if rt.Spec.Schedule.IntervalSeconds == 0 {
		// TODO: Very ugly compare here, interval=0 && jitterperc=0 meaning non-interval task
		// TODO: Change to Daemon mode, or non-interval mode flag
		if rt.Spec.Schedule.JitterPercent != 0 {
			s.logger.ErrorContext(ctx, "interval less than zero", "interval", interval.Seconds(), "interval_raw", rt.Spec.Schedule.IntervalSeconds)
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
			executeTask()
		}
		return
	} else {
		// Interval-based task
		jitter := computeJitter(interval, rt.Spec.Schedule.JitterPercent)

		select {
		case <-time.After(jitter):
		case <-ctx.Done():
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			executeTask()

			select {
			case <-ticker.C:
				// Next cycle
			case <-ctx.Done():
				s.logger.InfoContext(ctx, "closing task loop", "task_id", rt.Spec.TaskID)
				return
			}
		}
	}
}

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

func (s *Supervisor) reconcile(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop removed tasks
	toStop := make([]*RunningTask, 0, len(s.running))

	for id, running := range s.running {
		if _, ok := s.desired[id]; !ok {
			toStop = append(toStop, running)
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(toStop))
	for _, rt := range toStop {
		go func() {
			defer wg.Done()
			rt.Stop()
		}()
	}
	wg.Wait()

	for _, rt := range toStop {
		delete(s.running, rt.Spec.TaskID)
	}

	s.logger.InfoContext(ctx, "finished removing undesired tasks", "task_count", len(toStop))

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
		// just restart for now
		s.logger.InfoContext(ctx, "restarting task", "task_id", running.Spec.TaskID)
		running.Stop()
		newRt := s.startTask(&desired)
		s.running[id] = newRt
	}
}
