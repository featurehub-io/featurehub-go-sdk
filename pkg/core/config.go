package core

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
	"github.com/sirupsen/logrus"
)

const defaultLogLevel = logrus.InfoLevel

// DefaultSSEReadTimeout is how long a streaming connection may go without data before reconnecting. FeatureHub
// holds an idle connection open (e.g. when the Last-Event-ID is current) for around 3 minutes before sending
// "bye", so the default is longer than that.
const DefaultSSEReadTimeout = 4 * time.Minute

// DefaultSSEMaxRetryDelay is the longest a streaming connection waits before reconnecting. The delay doubles on
// each consecutive failed reconnect until it reaches this.
const DefaultSSEMaxRetryDelay = time.Minute

// Config defines parameters for the repository:
type Config struct {
	LogLevel          logrus.Level // Logging level (default is "info")
	Logger            *logrus.Logger
	SDKKey            string                    // SDK key (copied from the UI), in the format "{namedCache}/environmentID/APIKey"
	additionalSDKKeys []string                  // extra SDK keys appended to polling requests
	ServerAddress     string                    // FeatureHub API endpoint
	WaitForData       *time.Duration            // if set, how long a repository will wait before aborting connection attempt
	repoMu            sync.Mutex                // makes setting, creating and discarding repoState take turns; taken after edgeMu, never before
	repoState         atomic.Pointer[repoState] // the repository and its usage adapter, set once (nil before first use and once closed)
	EdgeProvider      EdgeProviderFunc
	fatalErrorHandler atomic.Pointer[interfaces.ErrorFunc] // A user-provided handler func for fatal asynchronous errors, read by edge clients' goroutines
	requestedEdgeType atomic.Value                         // models.EdgeType, written under edgeMu but read by usage plugin goroutines
	timeout           time.Duration                        // timeout if using polling
	sseReadTimeout    time.Duration                        // how long a streaming connection may go without receiving data before reconnecting
	sseMaxRetryDelay  time.Duration                        // the longest delay between streaming reconnect attempts
	edgeMu            sync.Mutex                           // guards client, and makes Connect, Build, Close and the edge type changes take turns
	client            interfaces.EdgeClient
	closed            atomic.Bool
}

// NewConfig returns a configured Config:
func NewConfig(serverAddress, sdkKey string, edgeProvider EdgeProviderFunc) *Config {
	// Make a Logger:
	logger := logrus.New()
	logger.SetLevel(defaultLogLevel)

	// inspect environment variables to see if we are being signalled about what client to use
	var defaultEdge models.EdgeType = models.EdgeStreaming
	var timeout time.Duration = 0

	if os.Getenv("FEATUREHUB_POLLING_INTERVAL") != "" {
		defaultEdge = models.EdgeActiveRest
		timeout = EnvOrDefaultDuration("FEATUREHUB_POLLING_INTERVAL", 3*time.Minute)
	}

	if os.Getenv("FEATUREHUB_POLLING_PASSIVE") != "" {
		defaultEdge = models.EdgePassiveRest
	}

	config := &Config{
		LogLevel:         defaultLogLevel,
		Logger:           logger,
		SDKKey:           sdkKey,
		ServerAddress:    serverAddress,
		EdgeProvider:     edgeProvider,
		timeout:          timeout,
		sseReadTimeout:   EnvOrDefaultDuration("FEATUREHUB_SSE_READ_TIMEOUT", DefaultSSEReadTimeout),
		sseMaxRetryDelay: EnvOrDefaultDuration("FEATUREHUB_SSE_MAX_RETRY_DELAY", DefaultSSEMaxRetryDelay),
	}
	config.requestedEdgeType.Store(defaultEdge)
	return config
}

// repoState is a config's repository and its usage adapter. It is never changed once published:
type repoState struct {
	repository *ClientFeatureHubRepository
	adapter    *usage.Adapter
}

// passiveRestPollPlugin triggers a passive-REST poll whenever a usage event is emitted.
type passiveRestPollPlugin struct {
	config *Config
}

func (p *passiveRestPollPlugin) DefaultPluginAttributes() usage.ContextRecord { return nil }
func (p *passiveRestPollPlugin) CanSendAsync() bool                           { return true }
func (p *passiveRestPollPlugin) Close()                                       {}

func (p *passiveRestPollPlugin) Send(ctx context.Context, _ usage.UsageEvent) context.Context {
	if p.config.EdgeType() != models.EdgePassiveRest {
		return ctx
	}
	if client := p.config.edgeClient(); client != nil {
		client.Poll() //nolint:errcheck
	}
	return ctx
}

// Build - this is only relevant for Server Evaluated functionality. It pairs a single context with a single edge connection.
// you can have multiple connections ONLY if you have multiple instances of Config. All state is being evaluated on the server,
// no strategies are being sent back to the client.
func (c *Config) Build(context *models.Context) (interfaces.FeatureHubConfig, error) {
	if c.closed.Load() {
		return nil, errors.NewErrConfigClosed()
	}
	if c.ClientEvaluated() {
		return c, nil
	}

	// connect sends the header to an existing client, or makes and connects one, and reports a stopped client
	return c.connect(new(context.GenerateHeader()))
}

// SetRepository gives the config the repository to use, which must be done before anything else uses the config
// (which creates one). A config has one repository for its lifetime: setting a different one when it already has
// one returns ErrRepositoryAlreadySet, and setting one on a closed config returns ErrConfigClosed.
func (c *Config) SetRepository(repository *ClientFeatureHubRepository) error {
	if repository == nil {
		return errors.NewErrBadConfig("repository is nil")
	}
	c.repoMu.Lock()
	defer c.repoMu.Unlock()
	if c.closed.Load() {
		return errors.NewErrConfigClosed()
	}
	if state := c.repoState.Load(); state != nil {
		if state.repository == repository {
			return nil
		}
		return errors.NewErrRepositoryAlreadySet()
	}
	c.useRepository(repository)
	return nil
}

// useRepository publishes the repository with a new usage adapter. repoMu must be held:
func (c *Config) useRepository(repository *ClientFeatureHubRepository) *repoState {
	adapter := usage.NewAdapter(repository, c.Logger)
	adapter.RegisterPlugin(&passiveRestPollPlugin{config: c})
	state := &repoState{repository: repository, adapter: adapter}
	c.repoState.Store(state)
	return state
}

// repo returns the repository and its usage adapter, creating them on first use, or nils if the config is closed.
// Reading them takes no lock. Callers use the returned values, which stay usable even if Close runs meanwhile:
func (c *Config) repo() (*ClientFeatureHubRepository, *usage.Adapter) {
	if state := c.repoState.Load(); state != nil {
		return state.repository, state.adapter
	}
	if c.closed.Load() {
		return nil, nil
	}

	c.repoMu.Lock()
	defer c.repoMu.Unlock()
	// Close sets closed before it takes repoMu, so a repository made here is always discarded and closed by it
	if c.closed.Load() {
		return nil, nil
	}
	state := c.repoState.Load()
	if state == nil {
		state = c.useRepository(NewClientFeatureHubRepository(c.Logger))
	}
	return state.repository, state.adapter
}

// currentRepository returns the repository without creating one, or nil:
func (c *Config) currentRepository() *ClientFeatureHubRepository {
	if state := c.repoState.Load(); state != nil {
		return state.repository
	}
	return nil
}

// edgeClient returns the current edge client, or nil:
func (c *Config) edgeClient() interfaces.EdgeClient {
	c.edgeMu.Lock()
	defer c.edgeMu.Unlock()
	return c.client
}

// closeEdge closes and discards the edge client. edgeMu must be held:
func (c *Config) closeEdge() {
	if c.client != nil {
		c.client.Close()
		c.client = nil
	}
}

// Close shuts down the edge client, releases all interceptors and plugins, nils all
// internal references, and marks the config as closed. Idempotent. Plugins and interceptors are closed without
// any lock held, so their Close may call back into the config (which is then closed).
func (c *Config) Close() {
	c.edgeMu.Lock()
	if c.closed.Load() {
		c.edgeMu.Unlock()
		return
	}
	c.closed.Store(true)
	c.closeEdge()

	c.repoMu.Lock()
	state := c.repoState.Swap(nil)
	c.repoMu.Unlock()
	c.edgeMu.Unlock()

	if state != nil {
		// the adapter first, so no more usage events reach the plugins once the interceptors start closing
		state.adapter.Close()
		state.repository.Close()
	}
}

// IsClosed reports whether Close has been called on this Config.
func (c *Config) IsClosed() bool {
	return c.closed.Load()
}

func (c *Config) PassiveRest(interval time.Duration) *Config {
	c.edgeMu.Lock()
	defer c.edgeMu.Unlock()
	c.closeEdge()
	c.requestedEdgeType.Store(models.EdgePassiveRest)
	c.timeout = interval
	return c
}

func (c *Config) ActiveRest(timeout time.Duration) *Config {
	c.edgeMu.Lock()
	defer c.edgeMu.Unlock()
	c.closeEdge()
	c.requestedEdgeType.Store(models.EdgeActiveRest)
	c.timeout = timeout
	return c
}

func (c *Config) Streaming() *Config {
	c.edgeMu.Lock()
	defer c.edgeMu.Unlock()
	c.closeEdge()
	c.requestedEdgeType.Store(models.EdgeStreaming)
	c.timeout = time.Millisecond * 0
	return c
}

func (c *Config) EdgeType() models.EdgeType {
	edgeType, _ := c.requestedEdgeType.Load().(models.EdgeType)
	return edgeType
}

// IsReady - Is the repository ready, does it have its initial state?
// delegates to the internal repository. Returns false if the config has been closed.
func (c *Config) IsReady() bool {
	repository, _ := c.repo()
	if repository == nil {
		return false
	}
	return repository.IsReady()
}

// ReadinessListener - Configure the SDK with a function to call when we're ready (up and running with some data)
// delegates to the internal repository. No-op if the config has been closed.
func (c *Config) ReadinessListener(context context.Context, callbackFunc func(context context.Context)) {
	repository, _ := c.repo()
	if repository == nil {
		return
	}
	repository.ReadinessListener(context, callbackFunc)
}

func (c *Config) connect(header *string) (interfaces.FeatureHubConfig, error) {
	client, err := c.edgeClientFor(header)
	if client == nil || err != nil {
		return c, err
	}

	// Connect runs without edgeMu, as it can wait for data. The fatal error handler always runs on its own
	// goroutine, so it never runs inside connect, but it (or another goroutine) may Close the config meanwhile:
	client.Connect()

	if c.closed.Load() {
		return nil, errors.NewErrConfigClosed()
	}
	// the client may have stopped while connecting (e.g. a rejected first poll, or edge.stale)
	if client.Stopped() {
		return c, errors.NewErrEdgeStopped(client.StopReason())
	}

	return c, nil
}

// edgeClientFor returns a new edge client that needs connecting, or nil if there is already a client (which is
// sent the header). It returns ErrEdgeStopped if the client has stopped. Streaming(), ActiveRest() and
// PassiveRest() discard the client, so the next connect makes a new one:
func (c *Config) edgeClientFor(header *string) (interfaces.EdgeClient, error) {
	c.edgeMu.Lock()
	defer c.edgeMu.Unlock()

	if c.closed.Load() {
		return nil, errors.NewErrConfigClosed()
	}

	// An existing edge client is already connected, so it is not connected again. One that has stopped for good
	// (e.g. edge.stale) stays stopped and is reported as an error:
	if c.client != nil {
		if c.client.Stopped() {
			return nil, errors.NewErrEdgeStopped(c.client.StopReason())
		}
		if header != nil {
			c.client.ContextChange(*header)
		}
		return nil, nil
	}

	// closed was checked under edgeMu, which Close needs, so the repository is never nil here
	repository, _ := c.repo()
	client, err := c.EdgeProvider(c, repository)
	if err != nil {
		return nil, err
	}
	c.client = client

	// the client may be stopped before it connects (e.g. the server rejected the SDK key on the first request)
	if client.Stopped() {
		return nil, errors.NewErrEdgeStopped(client.StopReason())
	}

	if header != nil {
		client.ContextChange(*header)
	}
	return client, nil
}

// Connect prepares a repository and connects to the configured FH server:
func (c *Config) Connect() (interfaces.FeatureHubConfig, error) {
	if c.closed.Load() {
		return nil, errors.NewErrConfigClosed()
	}
	return c.connect(nil)
}

// NewContext returns a ClientWithContext, with default context values.
// Returns nil if the config has been closed.
func (c *Config) NewContext() interfaces.Context {
	repository, _ := c.repo()
	if repository == nil {
		return nil
	}
	return &ClientWithContext{
		Context: &models.Context{
			Custom: make(map[string]interface{}),
		},
		repository:        repository,
		featureRepository: repository,
	}
}

func (c *Config) RegisterUsagePlugin(plugin usage.Plugin) interfaces.FeatureHubConfig {
	_, adapter := c.repo()
	// a closed adapter (Close ran meanwhile) closes the plugin rather than keeping it
	if adapter == nil || !adapter.RegisterPlugin(plugin) {
		return nil
	}

	return c
}

// StartRequest is called once at the start of each request (by ContextMiddleware or NewRequestContext). Usage
// plugins that keep state for each request (usage.RequestScopedPlugin) add it to the context, which is returned.
// A closed config returns the context unchanged.
func (c *Config) StartRequest(ctx context.Context) context.Context {
	_, adapter := c.repo()
	if adapter == nil {
		return ctx
	}
	return adapter.StartRequest(ctx)
}

// Validate can be called to check various config options:
func (c *Config) Validate() error {

	// LogLevel shouldn't be empty:
	if c.LogLevel == 0 {
		c.LogLevel = logrus.InfoLevel
	}

	// SDKKey shouldn't be empty:
	if len(c.SDKKey) == 0 {
		return errors.NewErrBadConfig("SDKKey is required")
	}

	// SDKKey should be 3 strings delimited with a slash:
	if len(strings.Split(c.SDKKey, "/")) < 2 {
		return errors.NewErrBadConfig("Invalid SDKKey format")
	}

	// ServerAddress shouldn't be empty:
	if len(c.ServerAddress) == 0 {
		return errors.NewErrBadConfig("ServerAddress is required")
	}

	return nil
}

// WithContext creates a new context with the passed models.Context.
// Returns nil if the config has been closed.
func (c *Config) WithContext(context *models.Context) interfaces.Context {
	repository, _ := c.repo()
	if repository == nil {
		return nil
	}
	return &ClientWithContext{
		Context:           context,
		repository:        repository,
		featureRepository: repository,
	}
}

// WithFatalErrorHandler configures a handler called when an edge client stops for good because of an error
// (a rejected SDK key or server address, or an error or failure event from the server). The client stops, but
// the SDK never stops the process: without a handler the error is only logged. The handler runs on its own
// goroutine, so it may call back into the config (e.g. Close). If the error happens during Connect or Build (e.g.
// the first request is rejected), they also return it as ErrEdgeStopped. It can be set at any time:
func (c *Config) WithFatalErrorHandler(fatalErrorFunc interfaces.ErrorFunc) interfaces.FeatureHubConfig {
	c.fatalErrorHandler.Store(&fatalErrorFunc)
	return c
}

// ReportFatalError is used by edge clients when they stop because of an error. It passes the error to the
// handler set with WithFatalErrorHandler, on its own goroutine, or logs it if none is set. It never stops the
// process:
func (c *Config) ReportFatalError(err error, message string, details map[string]interface{}) {
	if handler := c.FatalErrorHandler(); handler != nil {
		// never on the caller's goroutine, which may be inside Connect holding the config's lock
		go handler(err, message, details)
		return
	}
	logger := c.Logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	logger.WithError(err).WithFields(details).Error(message)
}

// FatalErrorHandler returns the handler set with WithFatalErrorHandler, or nil if none is set:
func (c *Config) FatalErrorHandler() interfaces.ErrorFunc {
	handler := c.fatalErrorHandler.Load()
	if handler == nil {
		return nil
	}
	return *handler
}

// WithLogLevel adds a logLevel to the config:
func (c *Config) WithLogLevel(logLevel logrus.Level) interfaces.FeatureHubConfig {
	c.LogLevel = logLevel
	return c
}

// WithSSEReadTimeout sets how long a streaming (SSE) connection may go without receiving any data before
// the client drops it and reconnects (sending the Last-Event-ID). The server keeps an idle connection open
// and sends "bye" before closing it, so this should be longer than the server's connection time.
// It can also be set with the FEATUREHUB_SSE_READ_TIMEOUT environment variable (e.g. "5m").
// A value of zero (or less) means DefaultSSEReadTimeout is used; the timeout cannot be turned off.
// It must be set before Connect: a connected streaming client keeps the value it started with.
func (c *Config) WithSSEReadTimeout(value time.Duration) interfaces.FeatureHubConfig {
	c.sseReadTimeout = value
	return c
}

// SSEReadTimeout returns the streaming read timeout, or DefaultSSEReadTimeout if it is zero or less:
func (c *Config) SSEReadTimeout() time.Duration {
	if c.sseReadTimeout <= 0 {
		return DefaultSSEReadTimeout
	}
	return c.sseReadTimeout
}

// WithSSEMaxRetryDelay sets the longest a streaming (SSE) connection waits before reconnecting. The delay
// starts at the server's retry interval (3 seconds by default) and doubles on each consecutive failed reconnect
// until it reaches this. Each delay is also reduced by a random amount of up to half, so that clients do not
// all reconnect at once after an outage.
// It can also be set with the FEATUREHUB_SSE_MAX_RETRY_DELAY environment variable (e.g. "2m").
// A value of zero (or less) means DefaultSSEMaxRetryDelay is used.
// It must be set before Connect: a connected streaming client keeps the value it started with.
func (c *Config) WithSSEMaxRetryDelay(value time.Duration) interfaces.FeatureHubConfig {
	c.sseMaxRetryDelay = value
	return c
}

// SSEMaxRetryDelay returns the longest streaming reconnect delay, or DefaultSSEMaxRetryDelay if it is zero or less:
func (c *Config) SSEMaxRetryDelay() time.Duration {
	if c.sseMaxRetryDelay <= 0 {
		return DefaultSSEMaxRetryDelay
	}
	return c.sseMaxRetryDelay
}

// WithWaitForData adds a WaitForData config:
func (c *Config) WithWaitForData(value time.Duration) interfaces.FeatureHubConfig {
	c.WaitForData = &value
	return c
}

func (c *Config) AddValueInterceptor(valueInterceptor interfaces.FeatureValueInterceptor) {
	repository, _ := c.repo()
	if repository == nil {
		return
	}
	repository.AddValueInterceptor(valueInterceptor)
}

// EnvironmentID extracts the environment ID from the SDKKey.
// The key is either "namedCache/environmentID/apiKey" or "environmentID/apiKey".
func (c *Config) EnvironmentID() string {
	parts := strings.Split(c.SDKKey, "/")
	if len(parts) >= 3 {
		return parts[1]
	}
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

// ClientEvaluated reports whether this SDK key is a client-evaluated key.
// Client-evaluated keys contain a "*" and cause the SDK to evaluate rollout
// strategies locally. Server-evaluated keys do not contain "*" and rely on
// the server to evaluate strategies per-request.
func (c *Config) ClientEvaluated() bool {
	return strings.Contains(c.SDKKey, "*")
}

// FeaturesURL give us the full URL for receiving features (SSE endpoint):
func (c *Config) FeaturesURL() string {
	return fmt.Sprintf("%s/features/%s", c.ServerAddress, c.SDKKey)
}

// WithSDKKey adds an additional SDK key to the config. Additional keys are appended
// as extra apiKey= query parameters in polling requests, enabling multi-environment polling.
func (c *Config) WithSDKKey(key string) interfaces.FeatureHubConfig {
	c.additionalSDKKeys = append(c.additionalSDKKeys, key)
	return c
}

// PollingFeaturesURL returns the URL for the REST polling endpoint.
// All SDK keys (primary and additional) are included as separate apiKey= parameters.
func (c *Config) PollingFeaturesURL() string {
	url := fmt.Sprintf("%s/features?apiKey=%s", c.ServerAddress, c.SDKKey)
	for _, key := range c.additionalSDKKeys {
		url += "&apiKey=" + key
	}
	return url
}

// Timeout returns the polling interval configured via ActiveRest or PassiveRest:
func (c *Config) Timeout() time.Duration {
	return c.timeout
}
