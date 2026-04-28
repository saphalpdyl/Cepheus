package scamper_client

import "fmt"

type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("invalid config: %s: %s", e.Field, e.Message)
}
