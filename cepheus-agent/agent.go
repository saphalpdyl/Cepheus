package cepheusagent

import (
	"cepheus/api"
	"cepheus/cepheus-agent/log"
	"cepheus/scamper"
	goscamper "cepheus/scamper"
	"cepheus/stamp"
	"cepheus/telemetry"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/avast/retry-go"
)

// The control/management plane for cepheus-agent
// - Connects to the control plane (cepheus-server), and resolves configuration
// - Manages lifecycle of probes in goroutines, and restarts them if they fail
// - Reports batched probe results to the control plane at assigned intervals

type Agent struct {
	SerialId           string
	generation         int
	controlPlaneConfig ControlPlaneConfig

	probeDataStream chan api.ProbeResult

	lastConfigurationPulled time.Time

	mu          sync.RWMutex
	agentConfig *api.AgentConfig

	// initial configuration
	scamperBinPath string

	logger *slog.Logger

	httpClient *http.Client
}

type AgentInitConfig struct {
	SerialId           string
	LocalBufferSize    int
	ControlPlaneConfig ControlPlaneConfig
	ScamperBinPath     string

	Logger *slog.Logger
}

func NewAgent(cfg AgentInitConfig) *Agent {
	return &Agent{
		SerialId:           cfg.SerialId,
		generation:         0,
		probeDataStream:    make(chan api.ProbeResult, cfg.LocalBufferSize), // TODO: Change to a defined type
		controlPlaneConfig: cfg.ControlPlaneConfig,
		logger:             cfg.Logger,
		scamperBinPath:     cfg.ScamperBinPath,
	}
}

func (a *Agent) Run(ctx context.Context) (err error) {
	ctx, end, _ := telemetry.SpanWithErr(ctx, "Agent.Run", &err)
	defer end()

	err = a.pullAgentConfiguration(ctx)
	if err != nil {
		a.logger.ErrorContext(ctx, "failed to pull agent configuration", log.Err(err))
		return err
	}

	scamper, err := goscamper.NewClient(
		scamper.ScamperClientConfig{
			BinPath: a.scamperBinPath,
			PPS:     uint32(a.agentConfig.ScamperPPS),
		},
	)
	if err != nil {
		var scamperError goscamper.ConfigError
		if errors.Is(err, &scamperError) {
			a.logger.ErrorContext(ctx, err.Error())
		} else {
			a.logger.ErrorContext(ctx, "error initializing scamper client", log.Err(err))
		}

		return err
	}

	err = scamper.Start(ctx)
	if err != nil {
		a.logger.ErrorContext(ctx, "cannot start scamper", log.Err(err))
		return err
	}

	// TODO: TEMP stuff
	stampConfig := stamp.Config{
		ErrorEstimate: stamp.ErrorEstimateConfig{
			ClockFormat:  stamp.ClockFormatNTP,
			Multiplier:   1,
			Scale:        22,
			Synchronized: true,
		},
	}

	supervisor := NewSupervisor(SupervisorConfig{
		Ctx:             ctx,
		Scamper:         scamper,
		Logger:          a.logger,
		ProbeDataStream: a.probeDataStream,
		Executors: map[api.AgentTaskType]Executor{
			api.TaskTypeStampSender: NewStampSenderExecutor(
				stampConfig,
				a.logger.With(log.Domain(log.DomainProbeExecutor), log.Executor(api.TaskTypeStampSender)),
			),
			api.TaskTypeStampReflector: NewStampReflectorExecutor(
				stampConfig,
				a.logger.With(log.Domain(log.DomainProbeExecutor), log.Executor(api.TaskTypeStampReflector)),
			),
			api.TaskTypeTrace: NewTraceExecutor(
				scamper,
				a.logger.With(log.Domain(log.DomainProbeExecutor), log.Executor(api.TaskTypeTrace)),
			),
		},
	})

	a.logger.InfoContext(ctx, "started supervisor")
	supervisor.SetDesiredTasks(a.agentConfig.Tasks)

	go func() {
		ticker := time.NewTicker(time.Duration(a.controlPlaneConfig.ControlPlane.ConfigPullIntervalSeconds) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := a.pullAgentConfiguration(ctx)
				if err != nil {
					continue
				}

				a.lastConfigurationPulled = time.Now()

				supervisor.SetDesiredTasks(a.agentConfig.Tasks)
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
	err = scamper.Stop()
	if err != nil {
		a.logger.ErrorContext(ctx, "failed to stop scamper", log.Err(err))
	}

	return nil
}

func (a *Agent) pullAgentConfiguration(ctx context.Context) error {
	ctx, end, span := telemetry.SpanWithErr(ctx, "Agent.pullAgentConfiguration", nil)
	defer end()

	var agentConfig *api.AgentConfig

	err := retry.Do(
		func() error {
			var err error

			agentConfig, err = a.pullConfiguration(ctx)

			if err != nil {
				span.AddEvent("agent.config.pull failed")
			}

			return err
		},
		retry.Attempts(3),
		retry.Delay(5*time.Second),
	)

	if err != nil {
		a.logger.ErrorContext(ctx, "error with pulling agent configuration")
		return err
	}

	if a.generation == 0 {
		a.agentConfig = agentConfig
	}

	return nil
}

func (a *Agent) pullConfiguration(ctx context.Context) (config *api.AgentConfig, err error) {
	ctx, end, _ := telemetry.SpanWithErr(ctx, "Agent.PullConfiguration", &err)
	defer end()

	configUrl, err := url.JoinPath(a.controlPlaneConfig.ControlPlane.URL, a.controlPlaneConfig.ControlPlane.ConfigEndpoint, a.SerialId)
	if err != nil {
		a.logger.Error("failed to join path for config URL", log.Err(err))
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, configUrl, nil)
	if err != nil {
		a.logger.Error("failed to create request", log.Err(err))
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if a.httpClient == nil {
		a.httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.logger.Error("failed to fetch configuration", log.Err(err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		a.logger.Error("failed to fetch configuration", "status", resp.Status)
		return nil, fmt.Errorf("failed to fetch configuration: %v", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		a.logger.Error("failed to read configuration response body", log.Err(err))
		return nil, err
	}

	var configResult api.AgentConfig
	if err = json.Unmarshal(body, &configResult); err != nil {
		a.logger.Error("failed to unmarshal agent configuration", log.Err(err))
		return nil, err
	}

	a.logger.Info("configuration pulled", "serial_id", a.SerialId, "configuration", string(body))
	return &configResult, nil
}
