package main

import (
	"cepheus/argus"
	log "cepheus/argus/log"
	"cepheus/telemetry"
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx := context.Background()

	b := make([]byte, 4)
	rand.Read(b)
	serviceInstanceId := fmt.Sprintf("%x", b)

	config := argus.GetConfig()

	logShutdown, err := telemetry.SetupLogging(ctx, config.OtelSink, config.OtelEndpoint, "argus", serviceInstanceId, false)
	if err != nil {
		slog.Error("failed to setup logging", "error", err)
		os.Exit(1)
	}
	defer logShutdown(ctx)

	traceShutdown, err := telemetry.SetupTracing(ctx, config.OtelSink, config.OtelEndpoint, "argus", serviceInstanceId, false)
	if err != nil {
		slog.Error("failed to setup tracing", "error", err)
		os.Exit(1)
	}
	defer traceShutdown(ctx)

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	slog.InfoContext(ctx, "starting argus")

	d := argus.NewDetector(
		serviceInstanceId,
		config,
		slog.Default().With(log.Domain(log.DomainDetectorLifecycle)),
	)

	err = d.Start(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to start argus", log.Err(err))
		os.Exit(1)
	}

	slog.InfoContext(ctx, "shutting down")
}
