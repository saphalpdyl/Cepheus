package main

import (
	cepheusagent "cepheus/agent"
	"cepheus/agent/log"
	telemetry "cepheus/telemetry"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/kardianos/osext"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"
)

func main() {
	ctx := context.Background()

	serialID := ""
	cfgPath := "agent.config.yaml"
	scamperBinPath := ""

	if len(os.Args) > 1 {
		serialID = os.Args[1]
	}

	if serialID == "" {
		slog.Error("serial_id cannot be empty")
		os.Exit(1)
	}

	if len(os.Args) > 2 {
		cfgPath = os.Args[2]
	}

	if len(os.Args) > 3 {
		scamperBinPath = os.Args[3]
	} else {
		dirPath, err := osext.ExecutableFolder()
		if err != nil {
			slog.ErrorContext(ctx, "cannot get Executable directory")
		}

		scamperBinPath = path.Join(dirPath, "scamper-client")
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		slog.Error("failed to read config", "path", cfgPath, "error", err)
		os.Exit(1)
	}

	var cfg cepheusagent.ControlPlaneConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		slog.Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	if cfg.ControlPlane.URL == "" {
		slog.Error("control_plane.url is required")
		os.Exit(1)
	}

	if cfg.Telemetry.Sink == "otel" && cfg.Telemetry.OTelCollectorURL == "" {
		panic("sink == otel requires otel_collector_url to be non-empty")
	}

	logShutdown, err := telemetry.SetupLogging(ctx, cfg.Telemetry.Sink, cfg.Telemetry.OTelCollectorURL, "agent", "", false, attribute.String("serial_id", serialID))
	if err != nil {
		slog.Error("failed to setup logging", "error", err)
		os.Exit(1)
	}
	defer logShutdown(ctx)

	traceShutdown, err := telemetry.SetupTracing(ctx, cfg.Telemetry.Sink, cfg.Telemetry.OTelCollectorURL, "agent", "", false, attribute.String("serial_id", serialID))
	if err != nil {
		slog.Error("failed to setup tracing", "error", err)
		os.Exit(1)
	}
	defer traceShutdown(ctx)

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	slog.Info("starting", "control_plane", cfg.ControlPlane.URL, "serial_id", serialID)

	agent := cepheusagent.NewAgent(cepheusagent.AgentInitConfig{
		SerialId:           serialID,
		ControlPlaneConfig: cfg,
		ScamperBinPath:     scamperBinPath,
		Logger:             slog.Default().With(log.Domain(log.DomainAgentSupervisor), log.SerialID(serialID)),
	})

	err = agent.Run(ctx)
	if err != nil {
		slog.Error("error running agent", "error", err)
		os.Exit(1)
	}

	slog.Info("shutting down")
}
