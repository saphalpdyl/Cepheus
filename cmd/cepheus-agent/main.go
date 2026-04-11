package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ControlPlane struct {
		URL string `yaml:"url"`
	} `yaml:"control_plane"`
}

func main() {
	cfgPath := "cepheus-default.config.yaml"
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

	if cfg.ControlPlane.URL == "" {
		fmt.Fprintf(os.Stderr, "control_plane.url is required\n")
		os.Exit(1)
	}

	fmt.Printf("cepheus-agent starting, control plane: %s\n", cfg.ControlPlane.URL)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	fmt.Println("cepheus-agent shutting down")
}
