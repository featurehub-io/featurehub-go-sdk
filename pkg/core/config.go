package core

import (
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

const (
	defaultLogLevel = logrus.InfoLevel

	EdgeActiveRest  = "active-rest"
	EdgePassiveRest = "passive-rest"
	EdgeStreaming   = "streaming"
)

type EdgeType string

// Config defines parameters for the repository:
type Config struct {
	LogLevel          logrus.Level // Logging level (default is "info")
	Logger            *logrus.Logger
	SDKKey            string                      // SDK key (copied from the UI), in the format "{namedCache}/environmentID/APIKey"
	ServerAddress     string                      // FeatureHub API endpoint
	WaitForData       *time.Duration              // if set, how long a repository will wait before aborting connection attempt
	repository        *ClientFeatureHubRepository // A FeatureHub repository implementation
	usageAdapter      *usage.Adapter
	EdgeProvider      EdgeProviderFunc
	fatalErrorHandler *interfaces.ErrorFunc // A user-provided handler func for fatal asynchronous errors
	requestedEdgeType EdgeType
	timeout           time.Duration // timeout if using polling
	client            interfaces.EdgeClient
}

// NewConfig returns a configured Config:
func NewConfig(serverAddress, sdkKey string, edgeProvider EdgeProviderFunc) *Config {
	// Make a Logger:
	logger := logrus.New()
	logger.SetLevel(defaultLogLevel)

	// inspect environment variables to see if we are being signalled about what client to use
	var defaultEdge EdgeType = EdgeStreaming
	var timeout time.Duration = 0

	if os.Getenv("FEATUREHUB_POLLING_INTERVAL") != "" {
		defaultEdge = EdgeActiveRest
		timeout = EnvOrDefaultDuration("FEATUREHUB_POLLING_INTERVAL", 3*time.Minute)
	}

	if os.Getenv("FEATUREHUB_POLLING_PASSIVE") != "" {
		defaultEdge = EdgePassiveRest
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

func (c *Config) SetRepository(repository *ClientFeatureHubRepository) {
	if repository != c.repository {
		if c.usageAdapter != nil {
			c.usageAdapter.Close()
		}
		c.repository = repository
		c.usageAdapter = usage.NewAdapter(repository, c.Logger)
	}
}

func (c *Config) closeEdge() {
	if c.client != nil {
		c.client.Close()
		c.client = nil
	}
}

func (c *Config) PassiveRest(interval time.Duration) *Config {
	c.closeEdge()
	c.requestedEdgeType = EdgePassiveRest
	c.timeout = interval
	return c
}

func (c *Config) ActiveRest(timeout time.Duration) *Config {
	c.closeEdge()
	c.requestedEdgeType = EdgeActiveRest
	c.timeout = timeout
	return c
}

func (c *Config) Streaming() *Config {
	c.closeEdge()
	c.requestedEdgeType = EdgeStreaming
	c.timeout = time.Millisecond * 0
	return c
}

func (c *Config) EdgeType() EdgeType {
	return c.requestedEdgeType
}

// IsReady - Is the repository ready, does it have its initial state?
// delegates to the internal repository
func (c *Config) IsReady() bool {
	return c.checkRepository().IsReady()
}

// ReadinessListener - Configure the SDK with a function to call when we're ready (up and running with some data)
// delegates to the internal repository
func (c *Config) ReadinessListener(callbackFunc func()) {
	c.checkRepository().ReadinessListener(callbackFunc)
}

// Connect prepares a repository and connects to the configured FH server:
func (c *Config) Connect() (*Config, error) {
	provider, err := c.EdgeProvider(c, c.checkRepository())

	if err != nil {
		return c, err
	}

	c.client = provider

	c.client.Connect()

	return c, nil
}

// NewContext returns a ClientWithContext, with default context values:
func (c *Config) NewContext() *ClientWithContext {
	return &ClientWithContext{
		Context: &models.Context{
			Custom: make(map[string]interface{}),
		},
		repository:        c.repository,
		featureRepository: c.repository,
	}
}

func (c *Config) RegisterUsagePlugin(plugin usage.Plugin) *Config {
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
func (c *Config) WithContext(context *models.Context) *ClientWithContext {
	return &ClientWithContext{
		Context:           context,
		repository:        c.repository,
		featureRepository: c.repository,
	}
}

// WithFatalErrorHandler configures an error handler which will be called for asynchronous fatal errors:
func (c *Config) WithFatalErrorHandler(fatalErrorFunc interfaces.ErrorFunc) *Config {
	c.fatalErrorHandler = &fatalErrorFunc
	return c
}

// WithLogLevel adds a logLevel to the config:
func (c *Config) WithLogLevel(logLevel logrus.Level) *Config {
	c.LogLevel = logLevel
	return c
}

// WithWaitForData adds a WaitForData config:
func (c *Config) WithWaitForData(value time.Duration) *Config {
	c.WaitForData = &value
	return c
}

func (c *Config) AddValueInterceptor(valueInterceptor interfaces.FeatureValueInterceptor) {
	c.checkRepository()
	c.repository.AddValueInterceptor(valueInterceptor)
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

// PollingFeaturesURL returns the URL for the REST polling endpoint:
func (c *Config) PollingFeaturesURL() string {
	return fmt.Sprintf("%s/features?apiKey=%s", c.ServerAddress, c.SDKKey)
}

// Timeout returns the polling interval configured via ActiveRest or PassiveRest:
func (c *Config) Timeout() time.Duration {
	return c.timeout
}
