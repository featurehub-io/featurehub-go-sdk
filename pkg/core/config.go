package core

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
	"github.com/sirupsen/logrus"
)

const defaultLogLevel = logrus.InfoLevel

// Config defines parameters for the repository:
type Config struct {
	LogLevel          logrus.Level // Logging level (default is "info")
	Logger            *logrus.Logger
	SDKKey            string                      // SDK key (copied from the UI), in the format "{namedCache}/environmentID/APIKey"
	additionalSDKKeys []string                    // extra SDK keys appended to polling requests
	ServerAddress     string                      // FeatureHub API endpoint
	WaitForData       *time.Duration              // if set, how long a repository will wait before aborting connection attempt
	repository        *ClientFeatureHubRepository // A FeatureHub repository implementation
	usageAdapter      *usage.Adapter
	EdgeProvider      EdgeProviderFunc
	fatalErrorHandler *interfaces.ErrorFunc // A user-provided handler func for fatal asynchronous errors
	requestedEdgeType models.EdgeType
	timeout           time.Duration // timeout if using polling
	client            interfaces.EdgeClient
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

	return &Config{
		LogLevel:          defaultLogLevel,
		Logger:            logger,
		SDKKey:            sdkKey,
		requestedEdgeType: defaultEdge,
		ServerAddress:     serverAddress,
		EdgeProvider:      edgeProvider,
		timeout:           timeout,
	}
}

// passiveRestPollPlugin triggers a passive-REST poll whenever a usage event is emitted.
type passiveRestPollPlugin struct {
	config *Config
}

func (p *passiveRestPollPlugin) DefaultPluginAttributes() usage.ContextRecord { return nil }

func (p *passiveRestPollPlugin) Send(ctx context.Context, _ usage.UsageEvent) context.Context {
	if p.config.client != nil && p.config.requestedEdgeType == models.EdgePassiveRest {
		p.config.client.Poll() //nolint:errcheck
	}
	return ctx
}

// Build - this is only relevant for Server Evaluated functionality. It pairs a single context with a single edge connection.
// you can have multiple connections ONLY if you have multiple instances of Config. All state is being evaluated on the server,
// no strategies are being sent back to the client.
func (c *Config) Build(context *models.Context) (interfaces.FeatureHubConfig, error) {
	if c.ClientEvaluated() {
		return c, nil
	}

	if c.client == nil {
		return c.connect(new(context.GenerateHeader()))
	} else {
		c.client.ContextChange(context.GenerateHeader())
		return c, nil
	}
}

func (c *Config) SetRepository(repository *ClientFeatureHubRepository) {
	if repository != c.repository {
		if c.usageAdapter != nil {
			c.usageAdapter.Close()
		}
		c.repository = repository
		c.usageAdapter = usage.NewAdapter(repository, c.Logger)
		c.usageAdapter.RegisterPlugin(&passiveRestPollPlugin{config: c})
	}
}

func (c *Config) closeEdge() {
	c.Close()
}

// Close shuts down the active edge client, if any, and clears the reference.
func (c *Config) Close() {
	if c.client != nil {
		c.client.Close()
		c.client = nil
	}
}

func (c *Config) PassiveRest(interval time.Duration) *Config {
	c.closeEdge()
	c.requestedEdgeType = models.EdgePassiveRest
	c.timeout = interval
	return c
}

func (c *Config) ActiveRest(timeout time.Duration) *Config {
	c.closeEdge()
	c.requestedEdgeType = models.EdgeActiveRest
	c.timeout = timeout
	return c
}

func (c *Config) Streaming() *Config {
	c.closeEdge()
	c.requestedEdgeType = models.EdgeStreaming
	c.timeout = time.Millisecond * 0
	return c
}

func (c *Config) EdgeType() models.EdgeType {
	return c.requestedEdgeType
}

// IsReady - Is the repository ready, does it have its initial state?
// delegates to the internal repository
func (c *Config) IsReady() bool {
	return c.checkRepository().IsReady()
}

// ReadinessListener - Configure the SDK with a function to call when we're ready (up and running with some data)
// delegates to the internal repository
func (c *Config) ReadinessListener(context context.Context, callbackFunc func(context context.Context)) {
	c.checkRepository().ReadinessListener(context, callbackFunc)
}

func (c *Config) connect(header *string) (interfaces.FeatureHubConfig, error) {
	provider, err := c.EdgeProvider(c, c.checkRepository())

	if err != nil {
		return c, err
	}

	c.client = provider

	if header != nil {
		c.client.ContextChange(*header)
	}

	c.client.Connect()

	return c, nil
}

// Connect prepares a repository and connects to the configured FH server:
func (c *Config) Connect() (interfaces.FeatureHubConfig, error) {
	return c.connect(nil)
}

// NewContext returns a ClientWithContext, with default context values:
func (c *Config) NewContext() interfaces.Context {
	return &ClientWithContext{
		Context: &models.Context{
			Custom: make(map[string]interface{}),
		},
		repository:        c.repository,
		featureRepository: c.repository,
	}
}

func (c *Config) RegisterUsagePlugin(plugin usage.Plugin) interfaces.FeatureHubConfig {
	c.checkRepository()

	c.usageAdapter.RegisterPlugin(plugin)

	return c
}

// ensures the repositories are all set correctly and returns the internal one. for use by edge clients to
// push data into the repository
func (c *Config) checkRepository() interfaces.InternalRepository {
	if c.repository == nil {
		c.SetRepository(NewClientFeatureHubRepository(c.Logger))
	}

	return c.repository
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

// WithContext Create a new context with passed context
func (c *Config) WithContext(context *models.Context) interfaces.Context {
	return &ClientWithContext{
		Context:           context,
		repository:        c.repository,
		featureRepository: c.repository,
	}
}

// WithFatalErrorHandler configures an error handler which will be called for asynchronous fatal errors:
func (c *Config) WithFatalErrorHandler(fatalErrorFunc interfaces.ErrorFunc) interfaces.FeatureHubConfig {
	c.fatalErrorHandler = &fatalErrorFunc
	return c
}

// WithLogLevel adds a logLevel to the config:
func (c *Config) WithLogLevel(logLevel logrus.Level) interfaces.FeatureHubConfig {
	c.LogLevel = logLevel
	return c
}

// WithWaitForData adds a WaitForData config:
func (c *Config) WithWaitForData(value time.Duration) interfaces.FeatureHubConfig {
	c.WaitForData = &value
	return c
}

func (c *Config) AddValueInterceptor(valueInterceptor interfaces.FeatureValueInterceptor) {
	c.checkRepository()
	c.repository.AddValueInterceptor(valueInterceptor)
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
