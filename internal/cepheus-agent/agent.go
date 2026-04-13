package cepheusagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// The control/management plane for cepheus-agent
// - Connects to the control plane (cepheus-server), and resolves configuration
// - Manages lifecycle of probes in goroutines, and restarts them if they fail
// - Reports batched probe results to the control plane at assigned intervals

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
	if a.generation == 0 {
		// Pull configuration for the first time, and start probes
		a.pullConfiguration()
	}
}

func (a *Agent) pullConfiguration() error {
	configUrl, err := url.JoinPath(a.controlPlaneConfig.ControlPlane.URL, a.controlPlaneConfig.ControlPlane.ConfigEndpoint, a.SerialId)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to join path for configUrl")
		return err
	}

	resp, err := http.Post(configUrl, "application/json", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to fetch configuration: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "failed to fetch configuration: %v\n", resp.Status)
		return fmt.Errorf("failed to fetch configuration: %v", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read configuration response body: %v\n", err)
		return err
	}

	var configResult AgentConfig
	if err = json.Unmarshal(body, &configResult); err != nil {
		fmt.Fprintf(os.Stderr, "failed to unmarshal agent configuration %v", err)
		return err
	}

	return nil
}
