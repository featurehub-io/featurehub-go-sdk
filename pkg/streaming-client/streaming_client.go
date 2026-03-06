package streamingclient

import (
	"net/http"
	"time"

	"github.com/donovanhide/eventsource"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/sirupsen/logrus"
)

// StreamingClient implements the client interface by subscribing to server-side events:
type StreamingClient struct {
	apiClient         *eventsource.Stream
	config            *core.Config
	fatalErrorHandler interfaces.ErrorFunc
	isRunning         bool
	logger            *logrus.Logger
	repository        interfaces.InternalRepository
}

// New wraps NewStreamingClient (as the default / only implementation):
func New(config *core.Config, repository interfaces.InternalRepository) (*StreamingClient, error) {
	return NewStreamingClient(config, repository)
}

// NewStreamingClient prepares a new StreamingClient with given config:
func NewStreamingClient(config *core.Config, repository interfaces.InternalRepository) (*StreamingClient, error) {

	// Check for nil config:
	if config == nil {
		return nil, errors.NewErrBadConfig("Nil config provided")
	}

	// Get the config to self-validate:
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Set this logger in the models package (they use a global to keep the API simple):
	SetLogger(config.Logger)

	// Put this into a new StreamingClient:
	client := &StreamingClient{
		config:     config,
		logger:     config.Logger,
		repository: repository,
	}

	// Use the default fatalErrorFunc to handle fatal errors:
	client.WithFatalErrorHandler(client.fatalErrorFunc)

	// Report that we're starting:
	logger.WithField("server_address", client.config.ServerAddress).Info("Subscribing to FeatureHub server")

	// Prepare a custom HTTP request:
	req, err := http.NewRequest("GET", config.FeaturesURL(), nil)
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

// streaming client does not support server evaluated SSE
func (c *StreamingClient) ContextChange(header string) {
	// empty
}

// FatalErrorFunc is called when an unrecoverable asynchronous error is encountered:
func (c *StreamingClient) fatalErrorFunc(err error, message string, details map[string]interface{}) {
	c.logger.WithError(err).WithFields(details).Fatal(message)
}

// Start begins handling events from the streamer:
func (c *StreamingClient) Connect() {

	// Set the isRunning flag:
	c.isRunning = true

	// Handle incoming events:
	go c.handleEvents()
	go c.handleErrors()

	// Block until we have some data:
	if c.config.WaitForData != nil {
		for !c.repository.IsReady() {
			time.Sleep(time.Second)
		}
	}
}

// WithFatalErrorHandler configures an error handler which will be called for asynchronous fatal errors:
func (c *StreamingClient) WithFatalErrorHandler(fatalErrorFunc interfaces.ErrorFunc) *StreamingClient {
	c.fatalErrorHandler = fatalErrorFunc
	return c
}
