package core

import "github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"

type EdgeProviderFunc func(config *Config, internalRepository interfaces.InternalRepository) (interfaces.EdgeClient, error)
