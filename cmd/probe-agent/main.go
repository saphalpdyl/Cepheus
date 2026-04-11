package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cepheus/internal/probe"
	"cepheus/pkg/cepheustamp"
)

func main() {
	cfg := probe.ParseFlags()

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(1)
	}

	stampCfg := cepheustamp.Config{
		ErrorEstimate: cepheustamp.ErrorEstimateConfig{
			Scale:        uint8(cfg.ErrorEstimateScale),
			Multiplier:   uint8(cfg.ErrorEstimateMultiplier),
			ClockFormat:  cepheustamp.TimestampClockFormat(cfg.ErrorEstimateClockFormat),
			Synchronized: cfg.ErrorEstimateSynchronized,
		},
	}

	onError := func(err error) {
		fmt.Fprintf(os.Stderr, "stamp error: %v\n", err)
	}

	fmt.Printf("probe-agent starting in %s mode\n", cfg.Mode)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	switch cfg.Mode {
	case probe.ModeReflector:
		reflector, err := cepheustamp.NewReflector(cepheustamp.ReflectorConfig{
			LocalAddr: cfg.LocalAddr,
			OnError:   onError,
			Config:    stampCfg,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create reflector: %v\n", err)
			os.Exit(1)
		}

		go func() {
			if err := reflector.Serve(); err != nil {
				fmt.Fprintf(os.Stderr, "reflector error: %v\n", err)
				os.Exit(1)
			}
		}()

		<-sig
		reflector.Close()

	case probe.ModeActive:
		sender, err := cepheustamp.NewSender(cepheustamp.SenderConfig{
			LocalAddr:  cfg.LocalAddr,
			RemoteAddr: cfg.RemoteAddr,
			OnError:    onError,
			Config:     stampCfg,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create sender: %v\n", err)
			os.Exit(1)
		}

		go func() {
			seq, err := sender.Send()
			if err != nil {
				fmt.Fprintf(os.Stderr, "send error: %v\n", err)
				return
			}
			fmt.Printf("sent packet seq=%d\n", seq)
		}()

		<-sig
		sender.Close()
	}

	fmt.Println("probe-agent shutting down")
}
