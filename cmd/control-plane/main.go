// Control-plane = cepheus-server

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cepheus/internal/common"
	controlplane "cepheus/internal/control-plane"

	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen string `yaml:"listen"`
}

func main() {
	cfgPath := "cepheus-server.config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read config: %v\n", err)
		os.Exit(1)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}

	// Environment variables for database url
	databaseUrl, err := common.TryGetFromEnv("CEPHEUS_DB_URL")
	if err != nil {
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), databaseUrl)
	if err != nil {
		// TODO: Should retry instead of kill
		log.Fatalf("failed to connect to the database %v", err)
	}

	srv := controlplane.NewServer(cfg.Listen, pool)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		fmt.Println("control-plane shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	fmt.Printf("control-plane listening on %s\n", cfg.Listen)
	if err := srv.Start(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
