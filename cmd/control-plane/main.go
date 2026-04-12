package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	controlplane "cepheus/internal/control-plane"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen string `yaml:"listen"`
}

func main() {
	cfgPath := "controlplane.config.yaml"
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

	srv := controlplane.NewServer(cfg.Listen, map[string]controlplane.AgentConfig{})

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
