package main

import (
	stampprocessor "cepheus/processors/stamp-processor"
	"cepheus/telemetry"
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
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

	time.Sleep(30 * time.Second)

	slog.InfoContext(ctx, "shutting down")

}
