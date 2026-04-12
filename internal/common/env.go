package common

import (
	"fmt"
	"os"
)

func TryGetFromEnv(k string) (string, error) {
	val := os.Getenv(k)
	if val == "" {
		fmt.Fprintf(os.Stderr, "missing %s environment variable", k)
		return "", fmt.Errorf("missing environment variable")
	}

	return val, nil
}
