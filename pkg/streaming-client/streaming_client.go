package streamingclient

import (
	"net/http"
	"sync"
	"time"

	"github.com/donovanhide/eventsource"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/sirupsen/logrus"
)

// ErrorFunc is called when asynchronous errors are encountered:
type ErrorFunc func(error, string, map[string]interface{})

// StreamingClient implements the client interface by by subscribing to server-side events:
type StreamingClient struct {
	analyticsCollectors []interfaces.AnalyticsCollector
	analyticsMutex      sync.Mutex
	apiClient           *eventsource.Stream
	config              *Config
	fatalErrorHandler   ErrorFunc
	features            map[string]*models.FeatureState
	featuresMutex       sync.Mutex
	featuresURL         string
	hasData             bool
	isRunning           bool
	logger              *logrus.Logger
	notifiers           notifiers
	notifiersMutex      sync.Mutex
	readinessListener   func()
}

// New wraps NewStreamingClient (as the default / only implementation):
func New(config *Config) (*StreamingClient, error) {
	return NewStreamingClient(config)
}

// NewStreamingClient prepares a new StreamingClient with given config:
func NewStreamingClient(config *Config) (*StreamingClient, error) {

	// Check for nil config:
	if config == nil {
		return nil, errors.NewErrBadConfig("Nil config provided")
	}

	// Get the config to self-validate:
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Make a logger:
	logger := logrus.New()
	logger.SetLevel(config.LogLevel)

	// Set this logger in the models package (they use a global to keep the API simple):
	SetLogger(logger)

	// Put this into a new StreamingClient:
	client := &StreamingClient{
		config:    config,
		logger:    logger,
		notifiers: make(notifiers),
	}

	// Use the default fatalErrorFunc to handle fatal errors:
	client.WithFatalErrorHandler(client.fatalErrorFunc)

	// Report that we're starting:
	logger.WithField("server_address", client.config.ServerAddress).Info("Subscribing to FeatureHub server")

	// Prepare a custom HTTP request:
	req, err := http.NewRequest("GET", config.featuresURL(), nil)
	if err != nil {
		client.logger.WithError(err).Error("Error preparing request")
		return nil, err
	}

	// Prepare an API client:
	apiClient, err := eventsource.SubscribeWithRequest("", req)
	if err != nil {
		client.logger.WithError(err).Error("Error subscribing to server")
		return nil, err
	}
	client.apiClient = apiClient

	return client, nil
}

// FatalErrorFunc is called when an unrecoverable asynchronous error is encountered:
func (c *StreamingClient) fatalErrorFunc(err error, message string, details map[string]interface{}) {
	c.logger.WithError(err).WithFields(details).Fatal(message)
}

// ReadinessListener defines a callback function which will be triggered once the client has received data for the first time:
func (c *StreamingClient) ReadinessListener(callbackFunc func()) {
	c.readinessListener = callbackFunc
}

// Start begins handling events from the streamer:
func (c *StreamingClient) Start() {

	// Set the isRunning flag:
	c.isRunning = true

	// Handle incoming events:
	go c.handleEvents()
	go c.handleErrors()

	// Block until we have some data:
	if c.config.WaitForData {
		for !c.hasData {
			time.Sleep(time.Second)
		}
	}
}

// WithContext returns a ClientWithContext:
func (c *StreamingClient) WithContext(context *models.Context) *ClientWithContext {
	return &ClientWithContext{
		Context: context,
		client:  c,
		config:  c.config,
	}
}

// WithFatalErrorHandler configures an error handler which will be called for asynchronous fatal errors:
func (c *StreamingClient) WithFatalErrorHandler(fatalErrorFunc ErrorFunc) *StreamingClient {
	c.fatalErrorHandler = fatalErrorFunc
	return c
}

// isReady triggers various notifications that the client is ready to serve data:
func (c *StreamingClient) isReady() {

	// If we're not already flagged as ready:
	if !c.hasData {

		// Flag us as ready:
		c.hasData = true

		// Trigger the registered readinessListener:
		if c.readinessListener != nil {
			c.logger.Trace("Calling readinessListener()")
			c.readinessListener()
		} else {
			c.logger.Trace("No registered readinessListener() to call")
		}
	}
}
