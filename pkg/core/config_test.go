package core

import (
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chanPlugin is a test Plugin that sends received events to a channel.
type chanPlugin struct {
	ch chan usage.UsageEvent
}

func newChanPlugin() *chanPlugin {
	return &chanPlugin{ch: make(chan usage.UsageEvent, 1)}
}

func (p *chanPlugin) DefaultPluginAttributes() usage.ContextRecord { return nil }
func (p *chanPlugin) Send(event usage.UsageEvent)                  { p.ch <- event }

// waitForEvent blocks until the plugin receives an event or the timeout elapses.
func (p *chanPlugin) waitForEvent(t *testing.T, timeout time.Duration) usage.UsageEvent {
	t.Helper()
	select {
	case event := <-p.ch:
		return event
	case <-time.After(timeout):
		t.Fatal("timed out waiting for usage event")
		return nil
	}
}

func TestConfig(t *testing.T) {

	var edgeProvider EdgeProviderFunc

	edgeProvider = func(config *Config, internalRepository interfaces.InternalRepository) (interfaces.EdgeClient, error) {
		return nil, errors.NewErrBadConfig("no edge required in this test")
	}
	// Make sure that our fluent API for NewConfig works as expected:
	newConfig := NewConfig("myserver", "mySDKKey", edgeProvider).WithLogLevel(logrus.WarnLevel).WithWaitForData(time.Second)
	assert.Equal(t, "myserver", newConfig.ServerAddress)
	assert.Equal(t, "mySDKKey", newConfig.SDKKey)
	assert.Equal(t, logrus.WarnLevel, newConfig.LogLevel)
	assert.Equal(t, time.Second, *newConfig.WaitForData)

	// Try to connect (it will of course fail):
	newConfig, err := newConfig.Connect()
	assert.Error(t, err)

	var repo = NewClientFeatureHubRepository(newConfig.Logger)

	newConfig.SetRepository(repo)

	// Get a context, check that it inherited the correct attributes:
	newContext := newConfig.NewContext()
	assert.Equal(t, newConfig.repository, newContext.repository)
	assert.NotNil(t, newContext.Custom)

	// Now try WithContext:
	customContext := &models.Context{
		Userkey: "customContextKey",
	}

	withContext := newConfig.WithContext(customContext)
	assert.Equal(t, newConfig.repository, withContext.repository)
	assert.Equal(t, "customContextKey", withContext.Userkey)
}

func TestClientEvaluated(t *testing.T) {
	config := &Config{}

	config.SDKKey = "default/environment-id/my-secret-api-key"
	assert.False(t, config.ClientEvaluated())

	config.SDKKey = "default/environment-id/my-secret-api-key*"
	assert.True(t, config.ClientEvaluated())

	config.SDKKey = "default/environment-id/*my-secret-api-key"
	assert.True(t, config.ClientEvaluated())
}

func TestRegisterUsagePluginReceivesEvents(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	plugin := newChanPlugin()

	config.RegisterUsagePlugin(plugin)

	event := simpleEvent("user-1")
	config.repository.EmitUsageEvent(event)

	received := plugin.waitForEvent(t, time.Second)
	require.NotNil(t, received)
	assert.Equal(t, "user-1", received.UserKey())
}

func TestRegisterMultipleUsagePluginsAllReceiveEvents(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	p1, p2 := newChanPlugin(), newChanPlugin()

	config.RegisterUsagePlugin(p1)
	config.RegisterUsagePlugin(p2)

	config.repository.EmitUsageEvent(simpleEvent("user-2"))

	assert.Equal(t, "user-2", p1.waitForEvent(t, time.Second).UserKey())
	assert.Equal(t, "user-2", p2.waitForEvent(t, time.Second).UserKey())
}

// mockEdgeClient records Poll calls.
type mockEdgeClient struct {
	pollCalls int
}

func (m *mockEdgeClient) Connect()               {}
func (m *mockEdgeClient) Poll() error            { m.pollCalls++; return nil }
func (m *mockEdgeClient) ContextChange(_ string) {}
func (m *mockEdgeClient) Close()                 {}

func TestPassiveRestPluginCallsPollOnUsageEvent(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	config.PassiveRest(time.Minute)
	config.checkRepository()

	mockClient := &mockEdgeClient{}
	config.client = mockClient

	config.repository.EmitUsageEvent(simpleEvent("user-1"))

	// dispatch is async — wait briefly for the goroutine to complete
	assert.Eventually(t, func() bool { return mockClient.pollCalls == 1 }, time.Second, time.Millisecond)
}

func TestPassiveRestPluginDoesNotPollWhenClientIsNil(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	config.PassiveRest(time.Minute)
	config.checkRepository()
	// client remains nil

	assert.NotPanics(t, func() {
		config.repository.EmitUsageEvent(simpleEvent("user-1"))
	})
}

func TestPassiveRestPluginDoesNotPollForActiveRest(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	config.ActiveRest(time.Minute)
	config.checkRepository()

	mockClient := &mockEdgeClient{}
	config.client = mockClient

	config.repository.EmitUsageEvent(simpleEvent("user-1"))

	// Give the goroutine time to run if it were going to
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, mockClient.pollCalls)
}

func TestPassiveRestPluginDoesNotPollForStreaming(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	config.Streaming()
	config.checkRepository()

	mockClient := &mockEdgeClient{}
	config.client = mockClient

	config.repository.EmitUsageEvent(simpleEvent("user-1"))

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, mockClient.pollCalls)
}

func TestConfigValidation(t *testing.T) {

	// Make a new config with nothing set:
	config := &Config{}

	// First thing tested is SDKKey:
	assert.Error(t, config.Validate())
	assert.Contains(t, config.Validate().Error(), "SDKKey is required")
	config.SDKKey = "some SDKKey"

	// Next is the format of the SDKKey:
	assert.Error(t, config.Validate())
	assert.Contains(t, config.Validate().Error(), "Invalid SDKKey format")
	config.SDKKey = "default/environment-id/my-secret-api-key"

	// Next is ServerAddress:
	assert.Error(t, config.Validate())
	assert.Contains(t, config.Validate().Error(), "ServerAddress is required")
	config.ServerAddress = "http://streams.test:8086"

	// Check that we can build the correct Features URL:
	featuresURL := config.FeaturesURL()
	assert.Equal(t, "http://streams.test:8086/features/default/environment-id/my-secret-api-key", featuresURL)

	// Now try a valid config:
	assert.NoError(t, config.Validate())
}
