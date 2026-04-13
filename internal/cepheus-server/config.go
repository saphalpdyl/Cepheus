package cepheusserver

type Config struct {
	Listen    string `yaml:"listen"`
	Telemetry struct {
		Sink             string `yaml:"sink"`
		OTelCollectorURL string `yaml:"otel_collector_url"`
	} `yaml:"telemetry"`
}
