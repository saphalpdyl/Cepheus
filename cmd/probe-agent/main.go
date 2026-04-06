package main

import (
	"fmt"
	"os"

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
}
