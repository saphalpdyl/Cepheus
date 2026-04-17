package cepheusagent

import (
	"cepheus/api"
	"cepheus/stamp"
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
)

type StampReflectorExecutor struct {
	stampConfig stamp.Config
	logger      *slog.Logger
}

func NewStampReflectorExecutor(stampCfg stamp.Config, logger *slog.Logger) *StampReflectorExecutor {
	return &StampReflectorExecutor{
		stampConfig: stampCfg,
		logger:      logger,
	}
}

func (e *StampReflectorExecutor) Execute(ctx context.Context, params api.TaskParams) (api.ProbeResult, error) {

	p, ok := params.(*api.AgentTaskStampReflectorParams)
	if !ok {
		return api.ProbeResult{}, fmt.Errorf("stamp-sender: expected AgentTaskStampSenderParams, got %T", params)
	}

	reflectorConfig := stamp.ReflectorConfig{
		LocalAddr: net.JoinHostPort(p.SourceIP, strconv.Itoa(int(p.ListenPort))),
		HMACKey:   nil,
		OnError: func(err error) {
			if ctx.Err() == nil {
				e.logger.ErrorContext(ctx, "error with reflector", "err", err)
			}
		},
		Config: e.stampConfig,
	}
	reflector, err := stamp.NewReflector(reflectorConfig)
	if err != nil {
		e.logger.ErrorContext(ctx, "couldn't create reflector")
		return api.ProbeResult{}, err
	}

	errChan := make(chan error, 1)

	go func() {
		errChan <- reflector.Serve(ctx)
	}()

	select {
	case <-ctx.Done():
		err := reflector.Close()
		if err != nil {
			e.logger.ErrorContext(ctx, "couldn't close reflector")
		}

		<-errChan

		return api.ProbeResult{}, ctx.Err()
	case err := <-errChan:
		if err != nil {
			e.logger.ErrorContext(ctx, "reflector errored out", "err", err)
		}
		return api.ProbeResult{}, err
	}
}
