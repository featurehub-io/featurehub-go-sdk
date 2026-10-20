package streamingclient

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/launchdarkly/eventsource"
	"github.com/sirupsen/logrus"
)

// StreamingClient implements the client interface by subscribing to server-side events:
type StreamingClient struct {
	events            chan eventsource.Event // events received from the SSE stream, closed when the stream ends
	config            *core.Config
	fatalErrorHandler atomic.Pointer[interfaces.ErrorFunc] // overrides the config's handler if set with WithFatalErrorHandler
	isRunning         atomic.Bool
	stopped           atomic.Bool
	// cancelRequest cancels the context of the SSE request. This is the only way the stream is shut down: the
	// eventsource library watches the request context and, when it is cancelled, closes the stream (so it will
	// not reconnect), aborts any connection or reconnect in progress, and closes the events channel.
	cancelRequest context.CancelFunc
	logger        *logrus.Logger
	repository    interfaces.InternalRepository
}

// sseJitterRatio randomly reduces each reconnect delay by up to this proportion, so that many clients do not
// all reconnect at the same moment after an outage:
const sseJitterRatio = 0.5

// sseResponseHeaderTimeout is how long to wait for the server to answer a connection attempt. The read timeout
// only applies once the response has started, so without this a server that accepts the connection but never
// answers would block Connect forever:
const sseResponseHeaderTimeout = 30 * time.Second

// newSSEHTTPClient returns an HTTP client for the stream. It has no overall timeout (the stream is long lived),
// only a limit on waiting for the response headers:
func newSSEHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = sseResponseHeaderTimeout
	return &http.Client{Transport: transport}
}

// eventsourceLogger sends the eventsource library's connection messages (e.g. reconnects) to the debug log:
type eventsourceLogger struct {
	logger *logrus.Logger
}

func (l eventsourceLogger) Println(v ...interface{})               { l.logger.Debugln(v...) }
func (l eventsourceLogger) Printf(format string, v ...interface{}) { l.logger.Debugf(format, v...) }

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

	// Report that we're starting:
	logger.WithField("server_address", client.config.ServerAddress).Info("Subscribing to FeatureHub server")

	// Prepare a custom HTTP request. Cancelling its context (see cancelRequest) shuts the stream down:
	requestContext, cancelRequest := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(requestContext, "GET", config.FeaturesURL(), nil)
	if err != nil {
		cancelRequest()
		client.logger.WithError(err).Error("Error preparing request")
		return nil, err
	}
	client.cancelRequest = cancelRequest

	// Subscribe to the stream. The read timeout means a dead connection is detected and reconnected (with the
	// Last-Event-ID) rather than blocking forever. Reconnects back off exponentially (with jitter) up to the
	// configured maximum delay:
	stream, err := eventsource.SubscribeWithRequestAndOptions(req,
		eventsource.StreamOptionHTTPClient(newSSEHTTPClient()),
		eventsource.StreamOptionReadTimeout(config.SSEReadTimeout()),
		eventsource.StreamOptionUseBackoff(config.SSEMaxRetryDelay()),
		eventsource.StreamOptionUseJitter(sseJitterRatio),
		eventsource.StreamOptionErrorHandler(client.handleStreamError),
		eventsource.StreamOptionLogger(eventsourceLogger{logger: client.logger}),
	)
	if err != nil {
		// A status that means we must never connect (236, or a rejected key or server address) stops the client,
		// which is returned so that it stays stopped. Any other error can be retried by connecting again:
		if client.handleStreamError(err); client.stopped.Load() {
			return client, nil
		}
		cancelRequest()
		client.logger.WithError(err).Error("Error subscribing to server")
		return nil, err
	}
	client.events = stream.Events

	return client, nil
}

// streaming client does not support server evaluated SSE
func (c *StreamingClient) ContextChange(header string) {
	// empty
}

// fatalErrorFunc is the default handler for unrecoverable asynchronous errors, used when no other is set. It
// only logs: the client has already stopped, and existing data continues to be served, so the host process
// is left running:
func (c *StreamingClient) fatalErrorFunc(err error, message string, details map[string]interface{}) {
	c.logger.WithError(err).WithFields(details).Error(message)
}

// reportFatalError passes an unrecoverable error to the handler set on this client, or else the one on the
// config, or else the default. The config is checked each time so that a handler added after Connect is used:
func (c *StreamingClient) reportFatalError(err error, message string, details map[string]interface{}) {
	var handler interfaces.ErrorFunc
	if override := c.fatalErrorHandler.Load(); override != nil {
		handler = *override
	} else if c.config != nil {
		handler = c.config.FatalErrorHandler()
	}
	if handler == nil {
		handler = c.fatalErrorFunc
	}
	handler(err, message, details)
}

// Poll - does nothing, its only used for PassiveRest
func (c *StreamingClient) Poll() error { return nil }

// Start begins handling events from the streamer:
func (c *StreamingClient) Connect() {
	if c.stopped.Load() {
		c.logger.Warn("StreamingClient has been closed and cannot be restarted")
		return
	}

	// Set the isRunning flag:
	c.isRunning.Store(true)

	// Handle incoming events:
	go c.handleEvents()

	// Block until we have some data:
	if c.config.WaitForData != nil {
		deadline := time.Now().Add(*c.config.WaitForData)
		for !c.repository.IsReady() && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// Close shuts down the SSE connection and event handlers. A closed client cannot be connected again. The
// client also closes itself if the server tells it to stop (edge.stale, or a 236 status) or rejects the key.
func (c *StreamingClient) Close() {
	c.isRunning.Store(false)
	c.stopped.Store(true)
	if c.cancelRequest != nil {
		// the eventsource library closes the stream and releases the connection when the request is cancelled
		c.cancelRequest()
	}
}

// Stopped reports whether the client has stopped for good: it was closed, the server told it to stop, or the
// server rejected the SDK key or server address. A stopped client cannot be connected again.
func (c *StreamingClient) Stopped() bool {
	return c.stopped.Load()
}

// WithFatalErrorHandler configures an error handler which will be called for asynchronous fatal errors:
func (c *StreamingClient) WithFatalErrorHandler(fatalErrorFunc interfaces.ErrorFunc) *StreamingClient {
	c.fatalErrorHandler.Store(&fatalErrorFunc)
	return c
}
