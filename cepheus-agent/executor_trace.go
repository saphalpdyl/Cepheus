package cepheusagent

import (
	"cepheus/api"
	goscamper "cepheus/scamper"
	"context"
	"fmt"
	"log/slog"
)

type TraceExecutor struct {
	scamper *goscamper.ScamperClient
	logger  *slog.Logger
}

func NewTraceExecutor(
	scamper *goscamper.ScamperClient,
	logger *slog.Logger,
) *TraceExecutor {
	return &TraceExecutor{
		scamper: scamper,
		logger:  logger,
	}
}

func (e *TraceExecutor) Execute(ctx context.Context, params api.TaskParams, spec *api.Task) (api.ProbeResult, error) {
	_, ok := params.(*api.AgentTaskTraceParams)
	if !ok {
		return api.ProbeResult{}, fmt.Errorf("scamper-trace: expected AgentTaskTraceParams, got %T", params)
	}

	return api.ProbeResult{}, nil

	// result, err := e.scamper.Traceroute(ctx, p.Target)
	// if err != nil {
	// 	e.logger.ErrorContext(ctx, "error with tracing", log.Err(err))
	// 	return api.ProbeResult{}, err
	// }

	// return api.ProbeResult{
	// 	TaskID:    spec.TaskID,
	// 	Kind:      string(spec.Type),
	// 	Timestamp: time.Now(),
	// 	Data:      result.ToMap(),
	// }, nil
}
