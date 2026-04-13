package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	cepheusagent "cepheus/internal/cepheus-agent"
	"cepheus/internal/cepheus-agent/logattr"
	"cepheus/internal/common/telemetry"

	"gopkg.in/yaml.v3"
)

func log() *slog.Logger {
	return slog.Default().With(logattr.Domain(logattr.DomainAgentLifecycle))
}

func main() {
	ctx := context.Background()

	serialID := ""
	cfgPath := "cepheus-agent.config.yaml"
	if len(os.Args) > 1 {
		serialID = os.Args[1]
	}
	if len(os.Args) > 2 {
		cfgPath = os.Args[2]
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		log().Error("failed to read config", "path", cfgPath, "error", err)
		os.Exit(1)
	}

	var cfg cepheusagent.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log().Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	if cfg.ControlPlane.URL == "" {
		log().Error("control_plane.url is required")
		os.Exit(1)
	}

	if cfg.Telemetry.Sink == "otel" && cfg.Telemetry.OTelCollectorURL == "" {
		panic("sink == otel requires otel_collector_url to be non-empty")
	}

	logShutdown, err := telemetry.SetupLogging(ctx, cfg.Telemetry.Sink, cfg.Telemetry.OTelCollectorURL, "cepheus-agent", "", false)
	if err != nil {
		slog.Error("failed to setup logging", "error", err)
		os.Exit(1)
	}
	defer logShutdown(ctx)

	traceShutdown, err := telemetry.SetupTracing(ctx, cfg.Telemetry.Sink, cfg.Telemetry.OTelCollectorURL, "cepheus-agent", "", false)
	if err != nil {
		slog.Error("failed to setup tracing", "error", err)
		os.Exit(1)
	}
	defer traceShutdown(ctx)

	log().Info("starting", "control_plane", cfg.ControlPlane.URL, "serial_id", serialID)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		log().Info("shutting down")
	}()

	agent := cepheusagent.NewAgent(cepheusagent.AgentConfig{
		SerialId:           serialID,
		LocalBufferSize:    100,
		ControlPlaneConfig: cfg,
	})
	agent.Run(ctx)

	log().Info("shutting down")
}
