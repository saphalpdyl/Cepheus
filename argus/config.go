package argus

import (
	"cepheus/common"
	"log/slog"
	"os"
	"strconv"
)

type DetectorConfig struct {
	OtelSink                 string
	OtelEndpoint             string
	DatabaseURL              string
	DetectionIntervalSeconds int
}

func handleConfigErrorWithExit(err error) {
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func GetConfig() DetectorConfig {
	otelSink, err := common.TryGetFromEnv("OTEL_SINK")
	handleConfigErrorWithExit(err)

	otelEndpoint, err := common.TryGetFromEnv("OTEL_ENDPOINT")
	handleConfigErrorWithExit(err)

	databaseUrl, err := common.TryGetFromEnv("CEPHEUS_DB_URL")
	handleConfigErrorWithExit(err)

	intervalStr, err := common.TryGetFromEnv("DETECTION_INTERVAL_SECONDS")
	handleConfigErrorWithExit(err)

	interval, err := strconv.Atoi(intervalStr)
	handleConfigErrorWithExit(err)

	return DetectorConfig{
		OtelSink:                 otelSink,
		OtelEndpoint:             otelEndpoint,
		DatabaseURL:              databaseUrl,
		DetectionIntervalSeconds: interval,
	}
}
