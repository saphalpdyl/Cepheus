package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// SetupTracing configures the global TracerProvider based on the sink value.
//   - "otel":   OTLP HTTP exporter → OTel collector → Tempo
//   - anything else: no-op (no tracing overhead in stdout/local mode)
//
// Returns a shutdown function that flushes pending spans.
func SetupTracing(ctx context.Context, sink, endpoint, serviceName, instanceID string, insecure bool, attrs ...attribute.KeyValue) (shutdown func(context.Context) error, err error) {
	if sink != "otel" {
		return func(context.Context) error { return nil }, nil
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithRetry(otlptracehttp.RetryConfig{
			Enabled: true,
		}),
	}
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	res, err := ServiceResource(serviceName, instanceID, attrs...)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter,
			// Large queue so spans buffer locally during network partitions
			// instead of blocking or being dropped.
			sdktrace.WithMaxQueueSize(6144),
			sdktrace.WithMaxExportBatchSize(512),
			sdktrace.WithBatchTimeout(5*time.Second),
		),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return provider.Shutdown, nil
}

// Span starts a new span and returns the updated context.
// Usage:
//
//	ctx, end, span := telemetry.SpanWithErr(ctx, "CepheusAgent.StreamProbeData", &err)
//	defer end()
func SpanWithErr(ctx context.Context, name string, err *error) (context.Context, func(), trace.Span) {
	ctx, span := otel.Tracer("cepheus").Start(ctx, name)
	return ctx, func() {
		if err != nil && *err != nil {
			span.RecordError(*err)
			span.SetStatus(codes.Error, (*err).Error())
		}
		span.End()
	}, span
}
