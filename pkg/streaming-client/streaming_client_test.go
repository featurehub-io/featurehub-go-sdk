package streamingclient

import (
	"bytes"
	"testing"
	"time"

	"github.com/donovanhide/eventsource"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// newTestStreamingClient builds a StreamingClient with buffered channels,
// bypassing the real SSE connection.
func newTestStreamingClient(t *testing.T) (*StreamingClient, *logrus.Logger) {
	t.Helper()
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logger.SetOutput(new(bytes.Buffer))

	repo := core.NewClientFeatureHubRepository(logger)
	client := &StreamingClient{
		apiClient: &eventsource.Stream{
			Events: make(chan eventsource.Event, 8),
			Errors: make(chan error, 8),
		},
		config:     &core.Config{Logger: logger},
		logger:     logger,
		repository: repo,
	}
	client.WithFatalErrorHandler(client.fatalErrorFunc)
	return client, logger
}

// testEvent implements the simple "Event" interface from donovanhide/eventsource:
type testEvent struct {
	data  string
	event string
	id    string
}

func (e *testEvent) Data() string  { return e.data }
func (e *testEvent) Event() string { return e.event }
func (e *testEvent) Id() string    { return e.id }

func TestStreamingClient(t *testing.T) {
	timeout := time.Hour
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logBuffer := new(bytes.Buffer)
	logger.SetOutput(logBuffer)

	// Make a test config (with an incorrect server address):
	config := &core.Config{
		LogLevel:      logrus.FatalLevel,
		Logger:        logger,
		SDKKey:        "environment-id/my-secret-api-key",
		ServerAddress: "http://streams.test:8086",
		WaitForData:   &timeout,
	}

	// Attempt to make a new client (config has a non-existent hostname):
	client, err := NewStreamingClient(config, core.NewClientFeatureHubRepository(logger))
	assert.Error(t, err)
	assert.Implements(t, new(interfaces.EdgeClient), client)
}

func TestCloseSetsFlagsAndClosesChannels(t *testing.T) {
	client, _ := newTestStreamingClient(t)

	assert.False(t, client.stopped)
	assert.False(t, client.isRunning)

	client.isRunning = true
	client.Close()

	assert.True(t, client.stopped)
	assert.False(t, client.isRunning)

	// The eventsource channels should be closed — a receive returns immediately.
	select {
	case _, ok := <-client.apiClient.Events:
		assert.False(t, ok, "Events channel should be closed")
	default:
		t.Fatal("Events channel was not closed")
	}
	select {
	case _, ok := <-client.apiClient.Errors:
		assert.False(t, ok, "Errors channel should be closed")
	default:
		t.Fatal("Errors channel was not closed")
	}
}

func TestConnectAfterCloseIsNoOp(t *testing.T) {
	client, _ := newTestStreamingClient(t)

	client.stopped = true

	client.Connect()

	assert.False(t, client.isRunning, "Connect on a stopped client must not set isRunning")
}

func TestCloseStopsEventHandlerGoroutine(t *testing.T) {
	client, _ := newTestStreamingClient(t)
	client.isRunning = true

	done := make(chan struct{})
	go func() {
		client.handleEvents()
		close(done)
	}()

	client.Close()

	select {
	case <-done:
		// goroutine exited as expected
	case <-time.After(time.Second):
		t.Fatal("handleEvents goroutine did not exit after Close")
	}
}

func TestCloseStopsErrorHandlerGoroutine(t *testing.T) {
	client, _ := newTestStreamingClient(t)
	client.isRunning = true

	done := make(chan struct{})
	go func() {
		client.handleErrors()
		close(done)
	}()

	client.Close()

	select {
	case <-done:
		// goroutine exited as expected
	case <-time.After(time.Second):
		t.Fatal("handleErrors goroutine did not exit after Close")
	}
}
