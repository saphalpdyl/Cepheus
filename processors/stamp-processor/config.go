package stampprocessor

import (
	"cepheus/common"
	"log/slog"
	"os"
)

type StampProcessorConfig struct {
	OtelSink          string
	OtelEndpoint      string
	NatsListenSubject string
	NatsConnectURL    string
	DatabaseURL       string
}

func handleConfigErrorWithExit(err error) {
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func GetConfig() StampProcessorConfig {
	otelSink, err := common.TryGetFromEnv("OTEL_SINK")
	handleConfigErrorWithExit(err)

	otelEndpoint, err := common.TryGetFromEnv("OTEL_ENDPOINT")
	handleConfigErrorWithExit(err)

	natsListenSubject, err := common.TryGetFromEnv("NATS_LISTEN_SUBJECT")
	handleConfigErrorWithExit(err)

	natsConnectUrl, err := common.TryGetFromEnv("NATS_LISTEN_URL")
	handleConfigErrorWithExit(err)

	databaseUrl, err := common.TryGetFromEnv("CEPHEUS_DB_URL")
	handleConfigErrorWithExit(err)

	return StampProcessorConfig{
		OtelSink:          otelSink,
		OtelEndpoint:      otelEndpoint,
		NatsListenSubject: natsListenSubject,
		NatsConnectURL:    natsConnectUrl,
		DatabaseURL:       databaseUrl,
	}
}
