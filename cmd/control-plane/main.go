// Control-plane = cepheus-server

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cepheus/internal/common"
	"cepheus/internal/common/telemetry"
	controlplane "cepheus/internal/control-plane"
	"cepheus/internal/control-plane/logattr"

	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"
)

func log() *slog.Logger {
	return slog.Default().With(logattr.Domain(logattr.DomainServerLifecycle))
}

type Config struct {
	Listen string `yaml:"listen"`
}

func main() {
	ctx := context.Background()

	logShutdown, err := telemetry.SetupLogging(ctx, "stdout", "", "cepheus-server", "", false)
	if err != nil {
		slog.Error("failed to setup logging", "error", err)
		os.Exit(1)
	}
	defer logShutdown(ctx)

	traceShutdown, err := telemetry.SetupTracing(ctx, "stdout", "", "cepheus-server", "", false)
	if err != nil {
		slog.Error("failed to setup tracing", "error", err)
		os.Exit(1)
	}
	defer traceShutdown(ctx)

	cfgPath := "cepheus-server.config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		log().Error("failed to read config", "path", cfgPath, "error", err)
		os.Exit(1)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log().Error("failed to parse config", "error", err)
		os.Exit(1)
	}

	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}

	databaseUrl, err := common.TryGetFromEnv("CEPHEUS_DB_URL")
	if err != nil {
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, databaseUrl)
	if err != nil {
		log().Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	srv := controlplane.NewServer(cfg.Listen, pool)

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
