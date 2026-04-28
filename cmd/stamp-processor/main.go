package main

import (
	stampprocessor "cepheus/stamp-processor"
	log "cepheus/stamp-processor/log"
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

	// Service instance id
	b := make([]byte, 4)
	rand.Read(b)
	serviceInstanceId := fmt.Sprintf("%x", b)

	// Get config
	config := stampprocessor.GetConfig()

	logShutdown, err := telemetry.SetupLogging(ctx, config.OtelSink, config.OtelEndpoint, "stamp-processor", serviceInstanceId, false)
	if err != nil {
		slog.Error("failed to setup logging", "error", err)
		os.Exit(1)
	}
	defer logShutdown(ctx)

	traceShutdown, err := telemetry.SetupTracing(ctx, config.OtelSink, config.OtelEndpoint, "stamp-processor", serviceInstanceId, false)
	if err != nil {
		slog.Error("failed to setup tracing", "error", err)
		os.Exit(1)
	}
	defer traceShutdown(ctx)

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	slog.InfoContext(ctx, "starting the processor")

	processor := stampprocessor.NewStampProcessor(
		serviceInstanceId,
		config,
		slog.Default().With(log.Domain(log.DomainProcessorLifecycle)),
	)

	err = processor.Start(ctx)

	if err != nil {
		slog.ErrorContext(ctx, "error while starting server", log.Err(err))
	}

	slog.InfoContext(ctx, "shutting down")

}
