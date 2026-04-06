package probe

import "fmt"

type Mode string

const (
	ModeActive    Mode = "active"
	ModeReflector Mode = "reflector"
)

type Config struct {
	Mode       Mode
	BackendURL string
}

func (c *Config) Validate() error {
	switch c.Mode {
	case ModeActive, ModeReflector:
		return nil
	default:
		return fmt.Errorf("unknown mode %q, expected %q or %q", c.Mode, ModeActive, ModeReflector)
	}
}
