package cepheusagent

import (
	"cepheus/api"
	goscamper "cepheus/scamper"
	"context"
	"fmt"
	"log/slog"
	"time"
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
	p, ok := params.(*api.AgentTaskTraceParams)
	if !ok {
		return api.ProbeResult{}, fmt.Errorf("scamper-trace: expected AgentTaskTraceParams, got %T", params)
	}

	resCh, err := e.scamper.Send(fmt.Sprintf("trace -P %s %s", string(p.Method), p.Target))
	if err != nil {
		return api.ProbeResult{}, fmt.Errorf("scamper-trace: failed to send trace command: %w", err)
	}

	select {
	case <-ctx.Done():
		return api.ProbeResult{}, fmt.Errorf("context cancelled")
	case res := <-resCh:
		return api.ProbeResult{
			TaskID:    spec.TaskID,
			ProbeType: api.ProbeTypeTrace,
			Timestamp: time.Now(),
			Kind:      string(spec.Type),
			Data: map[string]any{
				"method": string(p.Method),
				"data":   res,
			},
		}, nil
	}
}
