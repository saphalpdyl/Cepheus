package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path"
	"syscall"

	cepheusagent "cepheus/internal/cepheus-agent"
	"cepheus/internal/cepheus-agent/log"
	"cepheus/internal/telemetry"

	"github.com/kardianos/osext"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"
)

func main() {
	ctx := context.Background()

	serialID := ""
	cfgPath := "cepheus-agent.config.yaml"
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

		scamperBinPath = path.Join(dirPath, "scamper")
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

	logShutdown, err := telemetry.SetupLogging(ctx, cfg.Telemetry.Sink, cfg.Telemetry.OTelCollectorURL, "cepheus-agent", "", false, attribute.String("serial_id", serialID))
	if err != nil {
		slog.Error("failed to setup logging", "error", err)
		os.Exit(1)
	}
	defer logShutdown(ctx)

	traceShutdown, err := telemetry.SetupTracing(ctx, cfg.Telemetry.Sink, cfg.Telemetry.OTelCollectorURL, "cepheus-agent", "", false, attribute.String("serial_id", serialID))
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
		LocalBufferSize:    100,
		ControlPlaneConfig: cfg,
		ScamperBinPath:     scamperBinPath,
		Logger:             slog.Default().With(log.Domain(log.DomainAgentSupervisor)),
	})
	agent.Run(ctx)

	slog.Info("shutting down")
}
