package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	newConfig := New("http://some.server", "some/api-key")
	assert.Equal(t, "http://some.server", newConfig.ServerAddress)
	assert.Equal(t, "some/api-key", newConfig.SDKKey)
}
