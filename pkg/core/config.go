package core

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
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
	LogLevel           logrus.Level // Logging level (default is "info")
	Logger             *logrus.Logger
	SDKKey             string                      // SDK key (copied from the UI), in the format "{namedCache}/environmentID/APIKey"
	ServerAddress      string                      // FeatureHub API endpoint
	WaitForData        *time.Duration              // if set, how long a repository will wait before aborting connection attempt
	_repository        *ClientFeatureHubRepository // A FeatureHub repository implementation
	internalRepository interfaces.InternalRepository
	repository         interfaces.Repository
	EdgeProvider       EdgeProviderFunc
	fatalErrorHandler  *interfaces.ErrorFunc // A user-provided handler func for fatal asynchronous errors
	RequestedEdgeType  EdgeType              // used to determine which edge repository we should use
	timeout            time.Duration         // timeout if using polling
	client             interfaces.EdgeClient
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
		RequestedEdgeType: defaultEdge,
		ServerAddress:     serverAddress,
		EdgeProvider:      edgeProvider,
		timeout:           timeout,
	}
}

func (c *Config) SetRepository(repository interfaces.Repository) {
	c.repository = repository
}

func (c *Config) SetInternalRepository(repository interfaces.InternalRepository) {
	c.internalRepository = repository
}

func (c *Config) PassiveRest(interval time.Duration) {
	c.RequestedEdgeType = EdgePassiveRest
	c.timeout = interval
}

func (c *Config) ActiveRest(timeout time.Duration) {
	c.RequestedEdgeType = EdgeActiveRest
	c.timeout = timeout
}

func (c *Config) Streaming() {
	c.RequestedEdgeType = EdgeStreaming
	c.timeout = time.Millisecond * 0
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
		repository: c.Repository(),
	}
}

func (c *Config) Repository() interfaces.Repository {
	if c.repository != nil && c.internalRepository != nil {
		return c.repository
	}
	if c._repository == nil {
		c._repository = NewClientFeatureHubRepository(c.Logger)
	}

	return c._repository
}

// ensures the repositories are all set correctly and returns the internal one. for use by edge clients to
// push data into the repository
func (c *Config) checkRepository() interfaces.InternalRepository {
	if c.repository != nil && c.internalRepository != nil {
		return c.internalRepository
	}

	if c._repository == nil {
		c._repository = NewClientFeatureHubRepository(c.Logger)
	}

	return c._repository
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
		Context:    context,
		repository: c.Repository(),
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
