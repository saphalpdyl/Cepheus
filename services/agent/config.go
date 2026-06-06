package agent

type ControlPlaneConfig struct {
	ControlPlane struct {
		URL                       string `yaml:"url"`
		ConfigEndpoint            string `yaml:"config_endpoint"`
		ConfigPullIntervalSeconds int    `yaml:"config_pull_interval_secs"`
	} `yaml:"control_plane"`
	Telemetry struct {
		Sink             string `yaml:"sink"`
		OTelCollectorURL string `yaml:"otel_collector_url"`
	} `yaml:"telemetry"`
}
