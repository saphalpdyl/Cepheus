package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cepheus/internal/probe"
)

func main() {
	mode := os.Getenv("MODE")
	if mode == "" {
		mode = "active"
	}

	cfg := probe.Config{
		Mode: probe.Mode(mode),
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("probe-agent starting in %s mode\n", cfg.Mode)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("probe-agent shutting down")
}
