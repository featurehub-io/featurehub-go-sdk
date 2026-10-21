package core

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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
func (p *chanPlugin) Close()                                       {}
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

	var repo = NewClientFeatureHubRepository(newConfig.Logger)

	assert.NoError(t, newConfig.SetRepository(repo))

	// Try to connect (it will of course fail):
	_, err := newConfig.Connect()
	assert.Error(t, err)

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
	pollCalls         atomic.Int32 // Poll is called from the usage plugin's goroutine
	connectCalls      int
	contextChangeArgs []string
	stopped           bool
	closed            bool
	onConnect         func() // called from Connect, e.g. to run the fatal error handler as a rejected poll would
}

func (m *mockEdgeClient) Connect() {
	m.connectCalls++
	if m.onConnect != nil {
		m.onConnect()
	}
}
func (m *mockEdgeClient) Poll() error { m.pollCalls.Add(1); return nil }
func (m *mockEdgeClient) ContextChange(h string) {
	m.contextChangeArgs = append(m.contextChangeArgs, h)
}
func (m *mockEdgeClient) Close()        { m.closed = true }
func (m *mockEdgeClient) Stopped() bool { return m.stopped || m.closed }
func (m *mockEdgeClient) StopReason() error {
	if m.stopped {
		return errors.NewErrFromAPI("stopped by the test")
	}
	if m.closed {
		return errors.ErrClientClosed
	}
	return nil
}

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
	config.repo()

	mockClient := &mockEdgeClient{}
	config.client = mockClient

	config.repository.EmitUsageEvent(context.TODO(), simpleEvent("user-1"))

	// dispatch is async — wait briefly for the goroutine to complete
	assert.Eventually(t, func() bool { return mockClient.pollCalls.Load() == 1 }, time.Second, time.Millisecond)
}

func TestPassiveRestPluginDoesNotPollWhenClientIsNil(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	config.PassiveRest(time.Minute)
	config.repo()
	// client remains nil

	assert.NotPanics(t, func() {
		config.repository.EmitUsageEvent(context.TODO(), simpleEvent("user-1"))
	})
}

func TestPassiveRestPluginDoesNotPollForActiveRest(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	config.ActiveRest(time.Minute)
	config.repo()

	mockClient := &mockEdgeClient{}
	config.client = mockClient

	config.repository.EmitUsageEvent(context.TODO(), simpleEvent("user-1"))

	// Give the goroutine time to run if it were going to
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(0), mockClient.pollCalls.Load())
}

func TestPassiveRestPluginDoesNotPollForStreaming(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	config.Streaming()
	config.repo()

	mockClient := &mockEdgeClient{}
	config.client = mockClient

	config.repository.EmitUsageEvent(context.TODO(), simpleEvent("user-1"))

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(0), mockClient.pollCalls.Load())
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

// --- Closed config behaviour ---

func newClosedConfig(t *testing.T) *Config {
	t.Helper()
	ep := EdgeProviderFunc(func(_ *Config, _ interfaces.InternalRepository) (interfaces.EdgeClient, error) {
		return nil, errors.NewErrBadConfig("not needed")
	})
	cfg := NewConfig("http://server", "env/key*", ep)
	cfg.SetRepository(NewClientFeatureHubRepository(cfg.Logger))
	cfg.Close()
	return cfg
}

func TestClosedConfigIsClosedTrue(t *testing.T) {
	cfg := newClosedConfig(t)
	assert.True(t, cfg.IsClosed())
}

func TestClosedConfigCloseIsIdempotent(t *testing.T) {
	cfg := newClosedConfig(t)
	assert.NotPanics(t, func() { cfg.Close() })
	assert.True(t, cfg.IsClosed())
}

func TestClosedConfigNilsInternalReferences(t *testing.T) {
	cfg := newClosedConfig(t)
	assert.Nil(t, cfg.repository)
	assert.Nil(t, cfg.usageAdapter)
	assert.Nil(t, cfg.client)
}

func TestClosedConfigIsReadyFalse(t *testing.T) {
	cfg := newClosedConfig(t)
	assert.False(t, cfg.IsReady())
}

func TestClosedConfigReadinessListenerIsNoop(t *testing.T) {
	cfg := newClosedConfig(t)
	called := false
	assert.NotPanics(t, func() {
		cfg.ReadinessListener(context.Background(), func(_ context.Context) { called = true })
	})
	assert.False(t, called)
}

func TestClosedConfigConnectReturnsError(t *testing.T) {
	cfg := newClosedConfig(t)
	result, err := cfg.Connect()
	assert.Nil(t, result)
	require.Error(t, err)
	assert.IsType(t, &errors.ErrConfigClosed{}, err)
}

func TestClosedConfigBuildReturnsError(t *testing.T) {
	cfg := newClosedConfig(t)
	result, err := cfg.Build(&models.Context{})
	assert.Nil(t, result)
	require.Error(t, err)
	assert.IsType(t, &errors.ErrConfigClosed{}, err)
}

func TestClosedConfigNewContextReturnsNil(t *testing.T) {
	cfg := newClosedConfig(t)
	assert.Nil(t, cfg.NewContext())
}

func TestClosedConfigWithContextReturnsNil(t *testing.T) {
	cfg := newClosedConfig(t)
	assert.Nil(t, cfg.WithContext(&models.Context{}))
}

func TestClosedConfigAddValueInterceptorIsNoop(t *testing.T) {
	cfg := newClosedConfig(t)
	assert.NotPanics(t, func() {
		cfg.AddValueInterceptor(interfaces.NewInterceptor(func(_ context.Context, _ string, _ interfaces.FeatureRepository, _ *models.FeatureState) (interface{}, bool) {
			return nil, false
		}))
	})
}

func TestClosedConfigRegisterUsagePluginReturnsNil(t *testing.T) {
	cfg := newClosedConfig(t)
	plugin := newChanPlugin()
	result := cfg.RegisterUsagePlugin(plugin)
	assert.Nil(t, result)
}

func TestOpenConfigIsClosedFalse(t *testing.T) {
	ep := EdgeProviderFunc(func(_ *Config, _ interfaces.InternalRepository) (interfaces.EdgeClient, error) {
		return nil, errors.NewErrBadConfig("not needed")
	})
	cfg := NewConfig("http://server", "env/key*", ep)
	assert.False(t, cfg.IsClosed())
}

func TestConnectReusesExistingEdgeClient(t *testing.T) {
	var clients []*mockEdgeClient
	providerCalls := 0
	config := NewConfig("http://localhost", "default/env/key", func(_ *Config, _ interfaces.InternalRepository) (interfaces.EdgeClient, error) {
		providerCalls++
		clients = append(clients, &mockEdgeClient{})
		return clients[len(clients)-1], nil
	})

	_, err := config.Connect()
	require.NoError(t, err)
	_, err = config.Connect()
	require.NoError(t, err)

	// the existing client is kept, and not connected a second time
	assert.Equal(t, 1, providerCalls)
	assert.Equal(t, 1, clients[0].connectCalls)

	// changing the edge type discards the client, so the next Connect makes a new one
	config.Streaming()
	_, err = config.Connect()
	require.NoError(t, err)
	assert.Equal(t, 2, providerCalls)
	assert.True(t, clients[0].closed, "the discarded client is closed")
}

func TestConnectWithStoppedEdgeClientReturnsError(t *testing.T) {
	mockClient := &mockEdgeClient{}
	providerCalls := 0
	config := NewConfig("http://localhost", "default/env/key", func(_ *Config, _ interfaces.InternalRepository) (interfaces.EdgeClient, error) {
		providerCalls++
		return mockClient, nil
	})

	_, err := config.Connect()
	require.NoError(t, err)

	// e.g. the server sent edge.stale
	mockClient.stopped = true

	_, err = config.Connect()
	var stoppedErr *errors.ErrEdgeStopped
	assert.ErrorAs(t, err, &stoppedErr)
	assert.Contains(t, err.Error(), "stopped by the test", "the error says why the client stopped")
	var apiErr *errors.ErrFromAPI
	assert.ErrorAs(t, err, &apiErr, "the cause can be unwrapped")
	assert.Equal(t, 1, providerCalls, "a stopped client is not replaced")
	assert.Equal(t, 1, mockClient.connectCalls, "a stopped client is not connected again")
}

func TestConnectReturnsErrorWhenClientStopsStraightAway(t *testing.T) {
	// e.g. the server rejected the SDK key on the first request
	mockClient := &mockEdgeClient{stopped: true}
	config := NewConfig("http://localhost", "default/env/key", newMockEdgeProvider(mockClient))

	_, err := config.Connect()
	var stoppedErr *errors.ErrEdgeStopped
	assert.ErrorAs(t, err, &stoppedErr)
	assert.Equal(t, 0, mockClient.connectCalls, "a client that is already stopped is not connected")
}

func TestBuildWithStoppedEdgeClientReturnsError(t *testing.T) {
	mockClient := &mockEdgeClient{}
	config := NewConfig("http://localhost", "default/env/key", newMockEdgeProvider(mockClient))

	_, err := config.Build(&models.Context{Userkey: "fred"})
	require.NoError(t, err)
	_, err = config.Build(&models.Context{Userkey: "mary"})
	require.NoError(t, err)
	assert.Equal(t, 1, mockClient.connectCalls)
	assert.Len(t, mockClient.contextChangeArgs, 2)

	mockClient.stopped = true
	_, err = config.Build(&models.Context{Userkey: "sue"})
	var stoppedErr *errors.ErrEdgeStopped
	assert.ErrorAs(t, err, &stoppedErr)
	assert.Len(t, mockClient.contextChangeArgs, 2, "a stopped client is not sent the new context")
}

func TestReportFatalErrorCallsHandlerOrLogs(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	logs := new(bytes.Buffer)
	config.Logger.SetOutput(logs)
	exited := false
	config.Logger.ExitFunc = func(int) { exited = true }

	config.ReportFatalError(errors.NewErrFromAPI("bad"), "stopping", nil)
	assert.Contains(t, logs.String(), "stopping", "without a handler the error is logged")
	assert.False(t, exited, "the SDK never exits the process")

	messages := make(chan string, 1)
	config.WithFatalErrorHandler(func(err error, message string, details map[string]interface{}) {
		messages <- message
	})
	logs.Reset()
	config.ReportFatalError(errors.NewErrFromAPI("bad"), "stopping again", nil)
	select {
	case message := <-messages:
		assert.Equal(t, "stopping again", message)
	case <-time.After(2 * time.Second):
		t.Fatal("the handler was not called")
	}
	assert.Empty(t, logs.String(), "the handler decides what to log")
}

func TestConfigClosedDuringConnectReturnsConfigClosed(t *testing.T) {
	// e.g. a fatal error handler (on its own goroutine) closes the config while Connect waits for data
	mockClient := &mockEdgeClient{}
	config := NewConfig("http://localhost", "default/env/key", newMockEdgeProvider(mockClient))
	mockClient.onConnect = func() { config.Close() }

	cfg, err := config.Connect()

	assert.Nil(t, cfg)
	var closedErr *errors.ErrConfigClosed
	assert.ErrorAs(t, err, &closedErr)
	assert.True(t, mockClient.closed)
}

func TestFatalHandlerRunsOffTheConnectGoroutine(t *testing.T) {
	// e.g. streaming with a rejected key: the client reports while the edge provider makes it, and the handler
	// connects again. It must not deadlock or recurse, and gets the stopped client's error.
	var config *Config
	providerCalls := 0
	innerErr := make(chan error, 1)
	config = NewConfig("http://localhost", "default/env/key", func(c *Config, _ interfaces.InternalRepository) (interfaces.EdgeClient, error) {
		providerCalls++
		c.ReportFatalError(errors.NewStopReason(errors.ErrKeyRejected, "status 401"), "rejected", nil)
		return &mockEdgeClient{stopped: true}, nil
	})
	config.WithFatalErrorHandler(func(error, string, map[string]interface{}) {
		_, err := config.Connect()
		innerErr <- err
	})

	_, err := config.Connect()
	var stoppedErr *errors.ErrEdgeStopped
	assert.ErrorAs(t, err, &stoppedErr)

	select {
	case err := <-innerErr:
		assert.ErrorAs(t, err, &stoppedErr, "the handler's connect sees the stopped client")
	case <-time.After(2 * time.Second):
		t.Fatal("the handler did not run, or deadlocked")
	}
	assert.Equal(t, 1, providerCalls, "no second client is made")
}

func TestFatalHandlerClosingConfigWhileClientIsMade(t *testing.T) {
	var config *Config
	made := &mockEdgeClient{stopped: true}
	config = NewConfig("http://localhost", "default/env/key", func(c *Config, _ interfaces.InternalRepository) (interfaces.EdgeClient, error) {
		c.ReportFatalError(errors.NewStopReason(errors.ErrKeyRejected, "status 401"), "rejected", nil)
		return made, nil
	})
	config.WithFatalErrorHandler(func(error, string, map[string]interface{}) { config.Close() })

	_, err := config.Connect()

	var stoppedErr *errors.ErrEdgeStopped
	assert.ErrorAs(t, err, &stoppedErr)
	assert.Eventually(t, config.IsClosed, 2*time.Second, 10*time.Millisecond)
	assert.Eventually(t, func() bool { return config.edgeClient() == nil }, 2*time.Second, 10*time.Millisecond)
	assert.Zero(t, made.connectCalls)
}

func TestConcurrentBuildsAllSendTheirHeader(t *testing.T) {
	mockClient := &mockEdgeClient{}
	config := NewConfig("http://localhost", "default/env/key", newMockEdgeProvider(mockClient))

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := config.Build(&models.Context{Userkey: fmt.Sprintf("user-%d", i)})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}
	assert.Equal(t, 1, mockClient.connectCalls)
	config.edgeMu.Lock()
	defer config.edgeMu.Unlock()
	assert.Len(t, mockClient.contextChangeArgs, 20)
}

// --- One repository per config ---

func TestSetRepositoryCannotReplaceTheRepository(t *testing.T) {
	cfg := NewConfig("http://server", "env/key*", nil)
	first := NewClientFeatureHubRepository(cfg.Logger)
	require.NoError(t, cfg.SetRepository(first))
	assert.NoError(t, cfg.SetRepository(first), "setting the same repository again is allowed")

	var alreadySet *errors.ErrRepositoryAlreadySet
	assert.ErrorAs(t, cfg.SetRepository(NewClientFeatureHubRepository(cfg.Logger)), &alreadySet)
	assert.Same(t, first, cfg.repository)
}

func TestSetRepositoryAfterTheConfigMadeOneIsRefused(t *testing.T) {
	cfg := NewConfig("http://server", "env/key*", nil)
	ctx := cfg.NewContext().(*ClientWithContext)

	var alreadySet *errors.ErrRepositoryAlreadySet
	assert.ErrorAs(t, cfg.SetRepository(NewClientFeatureHubRepository(cfg.Logger)), &alreadySet)
	assert.Same(t, ctx.repository, cfg.repository, "contexts already made keep using the config's repository")
}

func TestSetRepositoryOnClosedConfigIsRefused(t *testing.T) {
	cfg := newClosedConfig(t)
	var closed *errors.ErrConfigClosed
	assert.ErrorAs(t, cfg.SetRepository(NewClientFeatureHubRepository(cfg.Logger)), &closed)
	assert.Nil(t, cfg.repository)
}

func TestConcurrentFirstUseMakesOneRepository(t *testing.T) {
	cfg := NewConfig("http://server", "env/key*", nil)
	cfg.Logger.SetOutput(new(bytes.Buffer))

	const n = 20
	repos := make(chan interfaces.FeatureRepository, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg.StartRequest(context.Background())
			repos <- cfg.NewContext().(*ClientWithContext).featureRepository
		}()
	}
	wg.Wait()
	close(repos)

	for repo := range repos {
		assert.Same(t, cfg.repository, repo)
	}
}

func TestRequestsRacingCloseDoNotPanic(t *testing.T) {
	for i := 0; i < 50; i++ {
		cfg := NewConfig("http://server", "env/key*", nil)
		cfg.Logger.SetOutput(new(bytes.Buffer))
		cfg.RegisterUsagePlugin(&evaluationPlugin{})

		var wg sync.WaitGroup
		for j := 0; j < 4; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				assert.NotPanics(t, func() {
					ctx, fhCtx := NewRequestContext(context.Background(), cfg)
					if fhCtx != nil {
						fhCtx.GetBoolean(ctx, "flag") //nolint:errcheck
					}
				})
			}()
		}
		cfg.Close()
		wg.Wait()
	}
}
