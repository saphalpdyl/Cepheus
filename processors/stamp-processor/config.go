package stampprocessor

import (
	"cepheus/common"
	"log/slog"
	"os"
)

type ProcessorConfig struct {
	OtelSink     string
	OtelEndpoint string
}

func handleConfigErrorWithExit(err error) {
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func GetConfig() ProcessorConfig {
	otelSink, err := common.TryGetFromEnv("OTEL_SINK")
	handleConfigErrorWithExit(err)

	otelEndpoint, err := common.TryGetFromEnv("OTEL_ENDPOINT")
	handleConfigErrorWithExit(err)

	return ProcessorConfig{
		OtelSink:     otelSink,
		OtelEndpoint: otelEndpoint,
	}
}
