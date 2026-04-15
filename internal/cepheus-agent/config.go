package cepheusagent

type ControlPlaneConfig struct {
	ControlPlane struct {
		URL            string `yaml:"url"`
		ConfigEndpoint string `yaml:"config_endpoint"`
	} `yaml:"control_plane"`
	Telemetry struct {
		Sink             string `yaml:"sink"`
		OTelCollectorURL string `yaml:"otel_collector_url"`
	} `yaml:"telemetry"`
}
