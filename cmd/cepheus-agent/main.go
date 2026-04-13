package main

import (
	cepheusagent "cepheus/internal/cepheus-agent"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gopkg.in/yaml.v3"
)

func main() {
	// The binary will be called as: cepheus-agent [serial-id] [config-file-path]
	serialID := ""
	cfgPath := "cepheus-agent.config.yaml"
	if len(os.Args) > 1 {
		serialID = os.Args[1]
	}
	if len(os.Args) > 2 {
		cfgPath = os.Args[2]
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read config: %v\n", err)
		os.Exit(1)
	}

	var cfg cepheusagent.Config
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

	agent := cepheusagent.NewAgent(cepheusagent.AgentConfig{
		SerialId:           serialID,
		LocalBufferSize:    100, // TODO: Make this configurable
		ControlPlaneConfig: cfg,
	})
	agent.Run(context.Background())

	fmt.Println("cepheus-agent shutting down")
}
