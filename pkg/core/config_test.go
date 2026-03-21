package core

import (
	"context"
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
func (p *chanPlugin) CanSendAsync() bool                           { return true }
func (p *chanPlugin) Send(ctx context.Context, event usage.UsageEvent) context.Context {
	p.ch <- event
	return ctx
}

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
	newConfig := NewConfig("myserver", "mySDKKey", edgeProvider)
	newConfig.WithLogLevel(logrus.WarnLevel)
	newConfig.WithWaitForData(time.Second)
	assert.Equal(t, "myserver", newConfig.ServerAddress)
	assert.Equal(t, "mySDKKey", newConfig.SDKKey)
	assert.Equal(t, logrus.WarnLevel, newConfig.LogLevel)
	assert.Equal(t, time.Second, *newConfig.WaitForData)

	// Try to connect (it will of course fail):
	_, err := newConfig.Connect()
	assert.Error(t, err)

	var repo = NewClientFeatureHubRepository(newConfig.Logger)

	newConfig.SetRepository(repo)

	// Get a context, check that it inherited the correct attributes:
	newContext := newConfig.NewContext().(*ClientWithContext)
	assert.Equal(t, newConfig.repository, newContext.repository)
	assert.NotNil(t, newContext.Custom)

	// Now try WithContext:
	customContext := &models.Context{
		Userkey: "customContextKey",
	}

	withContext := newConfig.WithContext(customContext).(*ClientWithContext)
	assert.Equal(t, newConfig.repository, withContext.repository)
	assert.Equal(t, "customContextKey", withContext.Userkey)
}

func TestEnvironmentIDThreePart(t *testing.T) {
	config := &Config{SDKKey: "default/env-id-123/my-api-key"}
	assert.Equal(t, "env-id-123", config.EnvironmentID())
}

func TestEnvironmentIDTwoPart(t *testing.T) {
	config := &Config{SDKKey: "env-id-456/my-api-key"}
	assert.Equal(t, "env-id-456", config.EnvironmentID())
}

func TestEnvironmentIDEmptyWhenKeyInvalid(t *testing.T) {
	config := &Config{SDKKey: "notvalid"}
	assert.Equal(t, "", config.EnvironmentID())
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
	config.repository.EmitUsageEvent(context.TODO(), event)

	received := plugin.waitForEvent(t, time.Second)
	require.NotNil(t, received)
	assert.Equal(t, "user-1", received.UserKey())
}

func TestRegisterMultipleUsagePluginsAllReceiveEvents(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	p1, p2 := newChanPlugin(), newChanPlugin()

	config.RegisterUsagePlugin(p1)
	config.RegisterUsagePlugin(p2)

	config.repository.EmitUsageEvent(context.TODO(), simpleEvent("user-2"))

	assert.Equal(t, "user-2", p1.waitForEvent(t, time.Second).UserKey())
	assert.Equal(t, "user-2", p2.waitForEvent(t, time.Second).UserKey())
}

// mockEdgeClient records Poll and ContextChange calls.
type mockEdgeClient struct {
	pollCalls         int
	connectCalls      int
	contextChangeArgs []string
}

func (m *mockEdgeClient) Connect()    { m.connectCalls++ }
func (m *mockEdgeClient) Poll() error { m.pollCalls++; return nil }
func (m *mockEdgeClient) ContextChange(h string) {
	m.contextChangeArgs = append(m.contextChangeArgs, h)
}
func (m *mockEdgeClient) Close() {}

func newMockEdgeProvider(client *mockEdgeClient) EdgeProviderFunc {
	return func(_ *Config, _ interfaces.InternalRepository) (interfaces.EdgeClient, error) {
		return client, nil
	}
}

func newErrorEdgeProvider(err error) EdgeProviderFunc {
	return func(_ *Config, _ interfaces.InternalRepository) (interfaces.EdgeClient, error) {
		return nil, err
	}
}

func TestPassiveRestPluginCallsPollOnUsageEvent(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	config.PassiveRest(time.Minute)
	config.checkRepository()

	mockClient := &mockEdgeClient{}
	config.client = mockClient

	config.repository.EmitUsageEvent(context.TODO(), simpleEvent("user-1"))

	// dispatch is async — wait briefly for the goroutine to complete
	assert.Eventually(t, func() bool { return mockClient.pollCalls == 1 }, time.Second, time.Millisecond)
}

func TestPassiveRestPluginDoesNotPollWhenClientIsNil(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	config.PassiveRest(time.Minute)
	config.checkRepository()
	// client remains nil

	assert.NotPanics(t, func() {
		config.repository.EmitUsageEvent(context.TODO(), simpleEvent("user-1"))
	})
}

func TestPassiveRestPluginDoesNotPollForActiveRest(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	config.ActiveRest(time.Minute)
	config.checkRepository()

	mockClient := &mockEdgeClient{}
	config.client = mockClient

	config.repository.EmitUsageEvent(context.TODO(), simpleEvent("user-1"))

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

	config.repository.EmitUsageEvent(context.TODO(), simpleEvent("user-1"))

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, mockClient.pollCalls)
}

// --- Close ---

func TestCloseCallsClientCloseAndNilsReference(t *testing.T) {
	mockClient := &mockEdgeClient{}
	config := &Config{client: mockClient}

	config.Close()

	assert.Nil(t, config.client)
}

func TestCloseIsNoopWhenClientIsNil(t *testing.T) {
	config := &Config{}
	assert.NotPanics(t, func() { config.Close() })
	assert.Nil(t, config.client)
}

func TestCloseIsIdempotent(t *testing.T) {
	mockClient := &mockEdgeClient{}
	config := &Config{client: mockClient}

	config.Close()
	assert.NotPanics(t, func() { config.Close() })
	assert.Nil(t, config.client)
}

// --- Build ---

func TestBuildClientEvaluatedReturnsWithoutConnecting(t *testing.T) {
	mockClient := &mockEdgeClient{}
	config := NewConfig("http://fh.example.com", "default/env/key*", newMockEdgeProvider(mockClient))

	ctx := &models.Context{Userkey: "alice"}
	result, err := config.Build(ctx)

	require.NoError(t, err)
	assert.Same(t, config, result.(*Config))
	assert.Nil(t, config.client, "client should not be created for client-evaluated keys")
	assert.Empty(t, mockClient.contextChangeArgs)
}

func TestBuildServerEvaluatedNoClientConnectsAndSendsHeader(t *testing.T) {
	mockClient := &mockEdgeClient{}
	config := NewConfig("http://fh.example.com", "default/env/key", newMockEdgeProvider(mockClient))

	ctx := &models.Context{Userkey: "bob", Country: models.ContextCountryAustralia}
	result, err := config.Build(ctx)

	require.NoError(t, err)
	assert.Same(t, config, result.(*Config))
	assert.Equal(t, 1, mockClient.connectCalls)
	require.Len(t, mockClient.contextChangeArgs, 1)
	assert.Equal(t, ctx.GenerateHeader(), mockClient.contextChangeArgs[0])
}

func TestBuildServerEvaluatedExistingClientCallsContextChange(t *testing.T) {
	mockClient := &mockEdgeClient{}
	config := NewConfig("http://fh.example.com", "default/env/key", newMockEdgeProvider(mockClient))
	config.client = mockClient // already connected

	ctx := &models.Context{Userkey: "carol", Device: models.ContextDeviceDesktop}
	result, err := config.Build(ctx)

	require.NoError(t, err)
	assert.Same(t, config, result.(*Config))
	assert.Equal(t, 0, mockClient.connectCalls, "Connect should not be called again when client already exists")
	require.Len(t, mockClient.contextChangeArgs, 1)
	assert.Equal(t, ctx.GenerateHeader(), mockClient.contextChangeArgs[0])
}

func TestBuildServerEvaluatedConnectErrorPropagates(t *testing.T) {
	providerErr := errors.NewErrBadConfig("edge provider failed")
	config := NewConfig("http://fh.example.com", "default/env/key", newErrorEdgeProvider(providerErr))

	ctx := &models.Context{Userkey: "dave"}
	_, err := config.Build(ctx)

	assert.Error(t, err)
	assert.Nil(t, config.client)
}

func TestBuildHeaderMatchesGenerateHeader(t *testing.T) {
	mockClient := &mockEdgeClient{}
	config := NewConfig("http://fh.example.com", "default/env/key", newMockEdgeProvider(mockClient))

	ctx := &models.Context{
		Userkey:  "eve",
		Session:  "s42",
		Device:   models.ContextDeviceMobile,
		Platform: models.ContextPlatformIos,
		Country:  models.ContextCountryThailand,
		Version:  "3.0.0",
		Custom:   map[string]interface{}{"plan": "enterprise"},
	}
	_, err := config.Build(ctx)

	require.NoError(t, err)
	require.Len(t, mockClient.contextChangeArgs, 1)
	assert.Equal(t, ctx.GenerateHeader(), mockClient.contextChangeArgs[0])
}

func TestBuildCalledTwiceWithExistingClientSendsEachHeader(t *testing.T) {
	mockClient := &mockEdgeClient{}
	config := NewConfig("http://fh.example.com", "default/env/key", newMockEdgeProvider(mockClient))
	config.client = mockClient

	ctx1 := &models.Context{Userkey: "frank"}
	ctx2 := &models.Context{Userkey: "grace"}
	_, err := config.Build(ctx1)
	require.NoError(t, err)
	_, err = config.Build(ctx2)
	require.NoError(t, err)

	require.Len(t, mockClient.contextChangeArgs, 2)
	assert.Equal(t, ctx1.GenerateHeader(), mockClient.contextChangeArgs[0])
	assert.Equal(t, ctx2.GenerateHeader(), mockClient.contextChangeArgs[1])
}

func TestPollingFeaturesURLSingleKey(t *testing.T) {
	config := &Config{ServerAddress: "http://fh.example.com", SDKKey: "default/env-1/key-1"}
	assert.Equal(t, "http://fh.example.com/features?apiKey=default/env-1/key-1", config.PollingFeaturesURL())
}

func TestPollingFeaturesURLWithAdditionalKeys(t *testing.T) {
	config := &Config{ServerAddress: "http://fh.example.com", SDKKey: "default/env-1/key-1"}
	config.WithSDKKey("default/env-2/key-2").WithSDKKey("default/env-3/key-3")
	assert.Equal(t,
		"http://fh.example.com/features?apiKey=default/env-1/key-1&apiKey=default/env-2/key-2&apiKey=default/env-3/key-3",
		config.PollingFeaturesURL())
}

func TestWithSDKKeyIsFluentAndAccumulates(t *testing.T) {
	config := &Config{SDKKey: "primary/env/key"}
	result := config.WithSDKKey("extra-1").WithSDKKey("extra-2")
	assert.Same(t, config, result)
	assert.Equal(t, []string{"extra-1", "extra-2"}, config.additionalSDKKeys)
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
