package agent

import (
	"cepheus/libs/common"
	scamper_client "cepheus/libs/scamper-client"
	"context"
	"fmt"
	"log/slog"
	"time"
)

type PingExecutor struct {
	scamper *scamper_client.ScamperClient
	logger  *slog.Logger
}

func NewPingExecutor(
	scamper *scamper_client.ScamperClient,
	logger *slog.Logger,
) *PingExecutor {
	return &PingExecutor{
		scamper: scamper,
		logger:  logger,
	}
}

func (p *PingExecutor) Execute(
	ctx context.Context,
	params TaskParams,
	spec *Task,
) (common.ProbeResult, error) {
	pr, ok := params.(*PingParams)
	if !ok {
		return common.ProbeResult{}, fmt.Errorf("params parsing error: expected PingParams, got %T", params)
	}

	resCh, err := p.scamper.Send(
		fmt.Sprintf("ping %s", pr.Target),
	)

	if err != nil {
		return common.ProbeResult{}, fmt.Errorf("failed to send ping probe, %v", err)
	}

	select {
	case <-ctx.Done():
		return common.ProbeResult{}, fmt.Errorf("context cancelled")
	case res := <-resCh:
		return common.ProbeResult{
			TaskID:    spec.TaskID,
			ProbeType: common.ProbeTypePing,
			Timestamp: time.Now(),
			Kind:      string(spec.Type),
			Data:      res.Data,
		}, nil
	}
}
