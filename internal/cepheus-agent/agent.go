package cepheusagent

import (
	"cepheus/internal/cepheus-agent/logattr"
	"cepheus/internal/common"
	"cepheus/internal/telemetry"
	"context"
	"encoding/json"
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

func log() *slog.Logger {
	return slog.Default().With(logattr.Domain(logattr.DomainAgentLifecycle))
}

type Agent struct {
	SerialId           string
	generation         int
	controlPlaneConfig ControlPlaneConfig

	probeDataBuffer chan any // TODO: Change to a defined type

	lastConfigurationPulled time.Time

	mu          sync.RWMutex
	agentConfig *common.AgentConfig
}

type AgentInitConfig struct {
	SerialId           string
	LocalBufferSize    int
	ControlPlaneConfig ControlPlaneConfig
}

func NewAgent(cfg AgentInitConfig) *Agent {
	return &Agent{
		SerialId:           cfg.SerialId,
		generation:         0,
		probeDataBuffer:    make(chan any, cfg.LocalBufferSize), // TODO: Change to a defined type
		controlPlaneConfig: cfg.ControlPlaneConfig,
	}
}

func (a *Agent) Run(ctx context.Context) {
	var err error
	ctx, end, _ := telemetry.SpanWithErr(ctx, "Agent.Run", &err)
	defer end()

	<-ctx.Done()
}

func (a *Agent) pullAgentConfiguration(ctx context.Context) {
	ctx, end, span := telemetry.SpanWithErr(ctx, "Agent.pullAgentConfiguration", nil)
	defer end()

	var agentConfig *common.AgentConfig

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
		log().ErrorContext(ctx, "error with pulling agent configuration")
		return
	}

	if a.generation == 0 {
		a.agentConfig = agentConfig
	}

	<-ctx.Done()
}

func (a *Agent) pullConfiguration(ctx context.Context) (config *common.AgentConfig, err error) {
	ctx, end, _ := telemetry.SpanWithErr(ctx, "Agent.PullConfiguration", &err)
	defer end()

	configUrl, err := url.JoinPath(a.controlPlaneConfig.ControlPlane.URL, a.controlPlaneConfig.ControlPlane.ConfigEndpoint, a.SerialId)
	if err != nil {
		log().Error("failed to join path for config URL", logattr.Err(err))
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, configUrl, nil)
	if err != nil {
		log().Error("failed to create request", logattr.Err(err))
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log().Error("failed to fetch configuration", logattr.Err(err))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log().Error("failed to fetch configuration", "status", resp.Status)
		return nil, fmt.Errorf("failed to fetch configuration: %v", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log().Error("failed to read configuration response body", logattr.Err(err))
		return nil, err
	}

	var configResult common.AgentConfig
	if err = json.Unmarshal(body, &configResult); err != nil {
		log().Error("failed to unmarshal agent configuration", logattr.Err(err))
		return nil, err
	}

	log().Info("configuration pulled", "serial_id", a.SerialId)
	return &configResult, nil
}
