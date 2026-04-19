package cepheusagent

import (
	"cepheus/api"
	goscamper "cepheus/scamper"
	"context"
	"log/slog"
	"sync"
)

type Supervisor struct {
	scamper *goscamper.ScamperClient
	mu      sync.RWMutex

	tasks   map[string]api.Task
	running map[string]*RunningTask
	desired map[string]api.Task

	ctx context.Context

	logger *slog.Logger

	executors map[api.AgentTaskType]Executor

	probeDataStream chan api.ProbeResult
}

type SupervisorConfig struct {
	Scamper   *goscamper.ScamperClient
	Ctx       context.Context
	Logger    *slog.Logger
	Executors map[api.AgentTaskType]Executor

	ProbeDataStream chan api.ProbeResult
}

func NewSupervisor(cfg SupervisorConfig) *Supervisor {
	return &Supervisor{
		scamper: cfg.Scamper,
		ctx:     cfg.Ctx,

		tasks:   make(map[string]api.Task),
		running: make(map[string]*RunningTask),
		desired: make(map[string]api.Task),
		logger:  cfg.Logger,

		executors:       cfg.Executors,
		probeDataStream: cfg.ProbeDataStream,
	}
}
