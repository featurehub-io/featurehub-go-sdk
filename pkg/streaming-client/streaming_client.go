package streamingclient

import (
	"context"
	goerrors "errors"
	"fmt"
	"io"
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
	stopReason        atomic.Pointer[error] // why the client stopped, set by the first stop
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

// headerTimeoutTransport limits how long a request waits for the response headers, and then lets the body (the
// long-lived stream) run without a limit. It sends each request through whatever http.DefaultTransport is at the
// time, so a transport the host has replaced (e.g. to add tracing, or a mock in tests) is always used, and all
// streaming clients share its connection pool.
type headerTimeoutTransport struct {
	timeout time.Duration
}

func (t headerTimeoutTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	timer := time.AfterFunc(t.timeout, cancel)

	resp, err := http.DefaultTransport.RoundTrip(req.WithContext(ctx))

	if !timer.Stop() {
		// the timer fired, so the request has been (or is about to be) cancelled
		if err == nil {
			resp.Body.Close()
		}
		cancel()
		timeoutErr := fmt.Errorf("no response from %s within %s: %w", req.URL.Host, t.timeout, context.DeadlineExceeded)
		if err != nil && !goerrors.Is(err, context.Canceled) {
			// keep the transport's own error (e.g. a DNS failure) if it is not just the cancellation
			return nil, fmt.Errorf("%w: %w", timeoutErr, err)
		}
		return nil, timeoutErr
	}
	if err != nil {
		cancel()
		return nil, err
	}

	// the request's context must stay alive while the body is read, and is released when the body is closed
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// cancelOnClose cancels a request's context when its response body is closed:
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnClose) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

// sseHTTPClient is used by all streaming clients. It has no overall timeout (the stream is long lived), only a
// limit on waiting for the response headers:
var sseHTTPClient = &http.Client{Transport: headerTimeoutTransport{timeout: sseResponseHeaderTimeout}}

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
		eventsource.StreamOptionHTTPClient(sseHTTPClient),
		eventsource.StreamOptionReadTimeout(config.SSEReadTimeout()),
		eventsource.StreamOptionUseBackoff(config.SSEMaxRetryDelay()),
		eventsource.StreamOptionUseJitter(sseJitterRatio),
		eventsource.StreamOptionErrorHandler(client.handleStreamError),
		eventsource.StreamOptionLogger(eventsourceLogger{logger: client.logger}),
	)
	if err != nil {
		// A status that means we must never connect (236, or a rejected key or server address) stops the client,
		// which is returned so that it stays stopped (Connect then returns ErrEdgeStopped with the reason). Any
		// other error can be retried by connecting again:
		if subscriptionErr, stop := stopStatus(err); stop {
			client.stopForStatus(subscriptionErr)
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

// reportFatalError passes an error that has stopped the client to the handler set on this client, or else to the
// config (its handler, or a log if it has none). The config is checked each time so that a handler added after
// Connect is used. The SDK never stops the host process:
func (c *StreamingClient) reportFatalError(err error, message string, details map[string]interface{}) {
	if override := c.fatalErrorHandler.Load(); override != nil {
		(*override)(err, message, details)
	} else if c.config != nil {
		c.config.ReportFatalError(err, message, details)
	} else {
		c.logger.WithError(err).WithFields(details).Error(message)
	}
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
		for !c.repository.IsReady() && !c.stopped.Load() && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// Close shuts down the SSE connection and event handlers. A closed client cannot be connected again. The
// client also closes itself if the server tells it to stop (edge.stale, or a 236 status), rejects the key or
// server address, or sends an error or failure event.
func (c *StreamingClient) Close() {
	c.stop(errors.ErrClientClosed)
}

// stop stops the client for good, and reports whether this was the first stop. The reason given by the first
// stop is kept, for StopReason. It is stored before stopped is set, so a client seen as stopped has its reason:
func (c *StreamingClient) stop(reason error) bool {
	c.isRunning.Store(false)
	first := c.stopReason.CompareAndSwap(nil, &reason)
	c.stopped.Store(true)
	if c.cancelRequest != nil {
		// the eventsource library closes the stream and releases the connection when the request is cancelled
		c.cancelRequest()
	}
	return first
}

// Stopped reports whether the client has stopped for good: it was closed, the server told it to stop, the
// server rejected the SDK key or server address, or the server sent an error. A stopped client cannot be
// connected again.
func (c *StreamingClient) Stopped() bool {
	return c.stopped.Load()
}

// StopReason returns why the client stopped, or nil if it has not:
func (c *StreamingClient) StopReason() error {
	if reason := c.stopReason.Load(); reason != nil {
		return *reason
	}
	if c.stopped.Load() {
		return errors.ErrClientClosed
	}
	return nil
}

// WithFatalErrorHandler configures an error handler which will be called for asynchronous fatal errors:
func (c *StreamingClient) WithFatalErrorHandler(fatalErrorFunc interfaces.ErrorFunc) *StreamingClient {
	c.fatalErrorHandler.Store(&fatalErrorFunc)
	return c
}
