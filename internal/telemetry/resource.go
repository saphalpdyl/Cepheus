package telemetry

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

// ServiceResource returns the shared OTel resource identifying this service.
// Both the logging and tracing providers use the same resource so that
// Grafana can correlate logs ↔ traces by service.name and service.instance.id.
func ServiceResource(serviceName, instanceID string, attrs ...attribute.KeyValue) (*resource.Resource, error) {
	all := append([]attribute.KeyValue{
		attribute.String("service.name", serviceName),
		attribute.String("service.instance.id", instanceID),
	}, attrs...)

	return resource.Merge(
		resource.Default(),
		resource.NewSchemaless(all...),
	)
}
