package common

// Payloads communicated between agent and control plane
type ProbeConfig struct {
	Type   string         `json:"type" yaml:"type"`
	Mode   string         `json:"mode" yaml:"mode"`
	Params map[string]any `json:"params" yaml:"params"`
}

type AgentConfig struct {
	Probes []ProbeConfig `json:"probes" yaml:"probes"`
}
