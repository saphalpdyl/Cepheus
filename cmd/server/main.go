// Control-plane = server

package main

import (
	"cepheus/common"
	cepheusserver "cepheus/server"
	logattr "cepheus/server/log"
	"cepheus/telemetry"
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"
)

func log() *slog.Logger {
	return slog.Default().With(logattr.Domain(logattr.DomainServerLifecycle))
}

func main() {
	ctx := context.Background()

	cfgPath := "server.config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		log().Error("failed to read config", "path", cfgPath, "error", err)
		os.Exit(1)
	}

	var cfg cepheusserver.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log().Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}

	if cfg.Telemetry.Sink == "otel" && cfg.Telemetry.OTelCollectorURL == "" {
		panic("sink == otel requires otel_collector_url to be non-empty")
	}

	logShutdown, err := telemetry.SetupLogging(ctx, cfg.Telemetry.Sink, cfg.Telemetry.OTelCollectorURL, "server", "", false)
	if err != nil {
		slog.Error("failed to setup logging", "error", err)
		os.Exit(1)
	}
	defer logShutdown(ctx)

	traceShutdown, err := telemetry.SetupTracing(ctx, cfg.Telemetry.Sink, cfg.Telemetry.OTelCollectorURL, "server", "", false)
	if err != nil {
		slog.Error("failed to setup tracing", "error", err)
		os.Exit(1)
	}
	defer traceShutdown(ctx)

	databaseUrl, err := common.TryGetFromEnv("CEPHEUS_DB_URL")
	if err != nil {
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, databaseUrl)
	if err != nil {
		log().Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	srv := cepheusserver.NewServer(cfg.Listen, pool)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		log().Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log().Info("listening", "addr", cfg.Listen)
	if err := srv.Start(); err != nil && err != http.ErrServerClosed {
		log().Error("server error", "error", err)
		os.Exit(1)
	}
}
