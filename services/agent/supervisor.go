package agent

import (
	goscamper "cepheus/libs/scamper-client"
	"context"
	"log/slog"
	"sync"
)

type Supervisor struct {
	scamper *goscamper.ScamperClient
	mu      sync.RWMutex

	tasks   map[string]Task
	running map[string]*RunningTask
	desired map[string]Task

	ctx context.Context

	logger *slog.Logger

	executors map[TaskType]Executor

	probeDataStream *ProbeDataStream
}

type SupervisorConfig struct {
	Scamper   *goscamper.ScamperClient
	Ctx       context.Context
	Logger    *slog.Logger
	Executors map[TaskType]Executor

	ProbeDataStream *ProbeDataStream
}

func NewSupervisor(cfg SupervisorConfig) *Supervisor {
	return &Supervisor{
		scamper: cfg.Scamper,
		ctx:     cfg.Ctx,

		tasks:   make(map[string]Task),
		running: make(map[string]*RunningTask),
		desired: make(map[string]Task),
		logger:  cfg.Logger,

		executors:       cfg.Executors,
		probeDataStream: cfg.ProbeDataStream,
	}
}
