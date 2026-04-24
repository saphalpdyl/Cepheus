package cepheusagent

import (
	"cepheus/api"
	"cepheus/common"
	goscamper "cepheus/scamper"
	"context"
	"encoding/json"
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

func (e *TraceExecutor) Execute(ctx context.Context, params api.TaskParams, spec *api.Task) (common.ProbeResult, error) {
	p, ok := params.(*api.AgentTaskTraceParams)
	if !ok {
		return common.ProbeResult{}, fmt.Errorf("scamper-trace: expected AgentTaskTraceParams, got %T", params)
	}

	resCh, err := e.scamper.Send(fmt.Sprintf("trace -P %s %s", string(p.Method), p.Target))
	if err != nil {
		return common.ProbeResult{}, fmt.Errorf("scamper-trace: failed to send trace command: %w", err)
	}

	select {
	case <-ctx.Done():
		return common.ProbeResult{}, fmt.Errorf("context cancelled")
	case res := <-resCh:
		data := common.TraceData{
			Type:   common.TraceProbeTypeTrace, // TODO: This should be configurable
			Method: p.Method,
			Data:   res.Data,
			Format: string(e.scamper.Format),
		}

		traceData, err := json.Marshal(data)

		if err != nil {
			e.logger.ErrorContext(ctx, "failed to marshal tarce probe data")
			return common.ProbeResult{}, err
		}

		return common.ProbeResult{
			TaskID:    spec.TaskID,
			ProbeType: common.ProbeTypeTrace,
			Timestamp: time.Now(),
			Kind:      string(spec.Type),
			Data:      traceData,
		}, nil
	}
}
