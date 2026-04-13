package cepheusagent

type Config struct {
	ControlPlane struct {
		URL            string `yaml:"url"`
		ConfigEndpoint string `yaml:"config_endpoint"`
	} `yaml:"control_plane"`
}
