package common

import (
	"fmt"
	"log/slog"
	"os"
)

func TryGetFromEnv(k string) (string, error) {
	val := os.Getenv(k)
	if val == "" {
		slog.Error("missing environment variable", "key", k)
		return "", fmt.Errorf("missing environment variable: %s", k)
	}

	return val, nil
}
