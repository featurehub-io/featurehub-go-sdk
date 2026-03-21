package core

import (
	"os"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
)

type EdgeProviderFunc func(config *Config, internalRepository interfaces.InternalRepository) (interfaces.EdgeClient, error)

func EnvOrDefaultStr(env, defaultValue string) string {
	e := os.Getenv(env)

	if e == "" {
		return defaultValue
	}

	return e
}

func EnvOrDefaultDuration(env string, defaultValue time.Duration) time.Duration {
	e := os.Getenv(env)

	if e == "" {
		return defaultValue
	}

	conv, err := time.ParseDuration(e)

	if err != nil {
		return defaultValue
	}

	return conv
}
