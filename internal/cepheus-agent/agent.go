package cepheusagent

import (
	"cepheus/internal/cepheus-agent/logattr"
	"cepheus/internal/common/telemetry"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
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
	controlPlaneConfig Config

	probeDataBuffer chan any // TODO: Change to a defined type

	lastConfigurationPulled time.Time
}

type AgentConfig struct {
	SerialId           string
	LocalBufferSize    int
	ControlPlaneConfig Config
}

func NewAgent(cfg AgentConfig) *Agent {
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

	if a.generation == 0 {
		err = a.pullConfiguration(ctx)
	}
}

func (a *Agent) pullConfiguration(ctx context.Context) (err error) {
	ctx, end, _ := telemetry.SpanWithErr(ctx, "Agent.PullConfiguration", &err)
	defer end()

	configUrl, err := url.JoinPath(a.controlPlaneConfig.ControlPlane.URL, a.controlPlaneConfig.ControlPlane.ConfigEndpoint, a.SerialId)
	if err != nil {
		log().Error("failed to join path for config URL", logattr.Err(err))
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, configUrl, nil)
	if err != nil {
		log().Error("failed to create request", logattr.Err(err))
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log().Error("failed to fetch configuration", logattr.Err(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log().Error("failed to fetch configuration", "status", resp.Status)
		return fmt.Errorf("failed to fetch configuration: %v", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log().Error("failed to read configuration response body", logattr.Err(err))
		return err
	}

	var configResult AgentConfig
	if err = json.Unmarshal(body, &configResult); err != nil {
		log().Error("failed to unmarshal agent configuration", logattr.Err(err))
		return err
	}

	log().Info("configuration pulled", "serial_id", a.SerialId)
	return nil
}
