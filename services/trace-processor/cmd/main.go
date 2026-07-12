package main

import (
	"cepheus/libs/telemetry"
	traceprocessor "cepheus/services/trace-processor"
	log "cepheus/services/trace-processor/log"
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
	config := traceprocessor.GetConfig()

	logShutdown, err := telemetry.SetupLogging(ctx, config.OtelSink, config.OtelEndpoint, "trace-processor", serviceInstanceId, false)
	if err != nil {
		slog.Error("failed to setup logging", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := logShutdown(ctx); err != nil {
			slog.Error("failed to shut down logging", "error", err)
		}
	}()

	traceShutdown, err := telemetry.SetupTracing(ctx, config.OtelSink, config.OtelEndpoint, "trace-processor", serviceInstanceId, false)
	if err != nil {
		slog.Error("failed to setup tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := traceShutdown(ctx); err != nil {
			slog.Error("failed to shut down tracing", "error", err)
		}
	}()

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	slog.InfoContext(ctx, "starting trace processor")

	processor := traceprocessor.NewTraceProcessor(
		serviceInstanceId,
		config,
		slog.Default().With(log.Domain(log.DomainProcessorLifecycle)),
	)

	err = processor.Start(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to start trace processor", log.Err(err))
		os.Exit(1)
	}

	slog.InfoContext(ctx, "shutting down")

}
