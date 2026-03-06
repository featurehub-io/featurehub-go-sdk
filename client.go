package client

import (
	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	polling "github.com/featurehub-io/featurehub-go-sdk/pkg/polling-client"
	streaming "github.com/featurehub-io/featurehub-go-sdk/pkg/streaming-client"
)

// New returns a config builder wired with all supported edge client implementations.
// Call one of config.Streaming(), config.ActiveRest(interval), or config.PassiveRest(interval)
// before calling config.Connect() to select the edge type.
//
// Go packages cannot be cyclic, so this root package acts as the orchestration point
// that wires core, streaming-client, and polling-client together.
func New(serverAddress, sdkKey string) *core.Config {
	return core.NewConfig(serverAddress, sdkKey, func(config *core.Config, internalRepository interfaces.InternalRepository) (interfaces.EdgeClient, error) {
		switch config.RequestedEdgeType {
		case core.EdgeStreaming:
			return streaming.NewStreamingClient(config, internalRepository)
		case core.EdgeActiveRest, core.EdgePassiveRest:
			return polling.NewPollingClient(config, internalRepository)
		default:
			return nil, errors.NewErrBadConfig("no edge type configured: call config.Streaming(), config.ActiveRest(), or config.PassiveRest() before Connect()")
		}
	})
}
