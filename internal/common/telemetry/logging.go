package telemetry

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// SetupLogging configures the global slog handler based on the sink value.
//   - "otel":   bridges slog to the OTel collector via OTLP HTTP → Loki
//   - "stdout": JSON handler to stderr (Cloud Run picks this up automatically)
//
// Both sinks filter out DEBUG-level logs (slog.LevelInfo minimum).
// Returns a shutdown function (no-op for stdout sink).
func SetupLogging(ctx context.Context, sink, endpoint, serviceName, instanceID string, insecure bool, attrs ...attribute.KeyValue) (shutdown func(context.Context) error, err error) {
	if sink != "otel" {
		handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
		slog.SetDefault(slog.New(handler).With(slog.String("service.instance.id", instanceID)))
		return func(context.Context) error { return nil }, nil
	}

	opts := []otlploghttp.Option{
		otlploghttp.WithEndpoint(endpoint),
		otlploghttp.WithInsecure(),
		otlploghttp.WithRetry(otlploghttp.RetryConfig{
			Enabled: true,
		}),
	}
	exporter, err := otlploghttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	res, err := ServiceResource(serviceName, instanceID, attrs...)
	if err != nil {
		return nil, err
	}

	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	)

	otelHandler := otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(provider))

	slog.SetDefault(slog.New(&levelFilter{
		level: slog.LevelInfo,
		inner: otelHandler,
	}))

	return provider.Shutdown, nil
}

// levelFilter wraps an slog.Handler and drops records below the configured level.
// The otelslog handler doesn't support slog.HandlerOptions, so this wrapper
// provides the minimum-level gate that filters out DEBUG from all sources
// including third-party SDKs.
type levelFilter struct {
	level slog.Level
	inner slog.Handler
}

func (f *levelFilter) Enabled(_ context.Context, l slog.Level) bool {
	return l >= f.level
}

func (f *levelFilter) Handle(ctx context.Context, r slog.Record) error {
	return f.inner.Handle(ctx, r)
}

func (f *levelFilter) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelFilter{level: f.level, inner: f.inner.WithAttrs(attrs)}
}

func (f *levelFilter) WithGroup(name string) slog.Handler {
	return &levelFilter{level: f.level, inner: f.inner.WithGroup(name)}
}
