package client

import (
	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	streaming "github.com/featurehub-io/featurehub-go-sdk/pkg/streaming-client"
)

// New returns a streaming client config:
// As golang packages are not allowed to be cyclic, we need an orchestration method that
// allows us to pick the right Edge type and create the appropriate instances of it.
func New(serverAddress, sdkKey string) *core.Config {
	return core.NewConfig(serverAddress, sdkKey, func(config *core.Config, internalRepository interfaces.InternalRepository) (interfaces.EdgeClient, error) {
		var edgeClient interfaces.EdgeClient = nil
		var err error = nil

		if config.RequestedEdgeType == core.EdgeStreaming {
			edgeClient, err = streaming.NewStreamingClient(config, internalRepository)
		}

		if edgeClient == nil && err == nil {
			return nil, errors.NewErrBadConfig("Failed to create streaming client")
		}

		return edgeClient, nil
	})
}
