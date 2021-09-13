package client

import (
	streamingclient "github.com/featurehub-io/featurehub-go-sdk/pkg/streaming-client"
)

// New returns a streaming client config:
func New(serverAddress, sdkKey string) *streamingclient.Config {
	return streamingclient.NewConfig(serverAddress, sdkKey)
}
