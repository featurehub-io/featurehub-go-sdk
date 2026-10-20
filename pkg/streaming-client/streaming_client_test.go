package streamingclient

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/launchdarkly/eventsource"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStream stands in for an *eventsource.Stream. Use its Close as the client's cancelRequest: it closes the
// events channel, as the real stream does when the request is cancelled.
type fakeStream struct {
	events    chan eventsource.Event
	closeOnce sync.Once
}

func newFakeStream(size int) *fakeStream {
	return &fakeStream{events: make(chan eventsource.Event, size)}
}

func (s *fakeStream) Close() {
	s.closeOnce.Do(func() { close(s.events) })
}

// newTestStreamingClient builds a StreamingClient with a buffered fake stream,
// bypassing the real SSE connection.
func newTestStreamingClient(t *testing.T) (*StreamingClient, *logrus.Logger) {
	t.Helper()
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logger.SetOutput(new(bytes.Buffer))

	repo := core.NewClientFeatureHubRepository(logger)
	stream := newFakeStream(8)
	client := &StreamingClient{
		cancelRequest: stream.Close,
		events:        stream.events,
		config:        &core.Config{Logger: logger},
		logger:        logger,
		repository:    repo,
	}
	return client, logger
}

// testEvent implements the simple "Event" interface from launchdarkly/eventsource:
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

	assert.False(t, client.stopped.Load())
	assert.False(t, client.isRunning.Load())

	client.isRunning.Store(true)
	client.Close()

	assert.True(t, client.stopped.Load())
	assert.False(t, client.isRunning.Load())

	// The stream should be closed, closing the events channel — a receive returns immediately.
	select {
	case _, ok := <-client.events:
		assert.False(t, ok, "Events channel should be closed")
	default:
		t.Fatal("Events channel was not closed")
	}
}

func TestConnectAfterCloseIsNoOp(t *testing.T) {
	client, _ := newTestStreamingClient(t)

	client.stopped.Store(true)

	client.Connect()

	assert.False(t, client.isRunning.Load(), "Connect on a stopped client must not set isRunning")
}

func TestCloseStopsEventHandlerGoroutine(t *testing.T) {
	client, _ := newTestStreamingClient(t)
	client.isRunning.Store(true)

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

func TestStreamErrorReconnectsUntilClosed(t *testing.T) {
	client, _ := newTestStreamingClient(t)
	client.isRunning.Store(true)

	assert.False(t, client.handleStreamError(eventsource.ErrReadTimeout).CloseNow, "a running client should reconnect")

	client.Close()

	assert.True(t, client.handleStreamError(eventsource.ErrReadTimeout).CloseNow, "a closed client must not reconnect")
}

func TestCloseStopsRealStream(t *testing.T) {
	server := &sseServer{}
	ts := server.start(t, func(connection int, w http.ResponseWriter, f http.Flusher, r *http.Request) {
		<-r.Context().Done()
	})

	client, _ := newSSETestClient(t, ts.URL, nil)
	require.Eventually(t, func() bool { return len(server.connections()) == 1 }, 2*time.Second, 10*time.Millisecond)

	client.Close()

	// The library closes its events channel once the stream has stopped and the connection is released:
	select {
	case _, ok := <-client.events:
		assert.False(t, ok, "Events channel should be closed")
	case <-time.After(2 * time.Second):
		t.Fatal("the eventsource stream did not stop after Close")
	}
}

func TestSSEErrorStopsClientAndCallsFatalErrorHandler(t *testing.T) {
	client, _ := newTestStreamingClient(t)

	var called bool
	var details map[string]interface{}
	client.WithFatalErrorHandler(func(err error, message string, d map[string]interface{}) {
		called = true
		details = d
	})

	client.handleSSEError(&testEvent{event: string(models.SSEError), data: "bad key"})

	assert.True(t, called, "the custom fatal error handler must be called for an error event")
	assert.Equal(t, "bad key", details["message"])
	assert.True(t, client.Stopped(), "an error event stops the client for good")
}

func TestConfigFatalErrorHandlerIsUsedForSSEError(t *testing.T) {
	server := &sseServer{}
	ts := server.start(t, func(connection int, w http.ResponseWriter, f http.Flusher, r *http.Request) {
		fmt.Fprint(w, "event: error\ndata: bad key\n\n")
		f.Flush()
		<-r.Context().Done()
	})

	called := make(chan string, 1)
	newSSETestClient(t, ts.URL, func(config *core.Config) {
		config.WithFatalErrorHandler(recordFatalErrors(called))
	})

	select {
	case message := <-called:
		assert.Equal(t, "Error from API client, stopping", message)
	case <-time.After(3 * time.Second):
		t.Fatal("the fatal error handler set on the config was not called")
	}
}

// recordFatalErrors returns a fatal error handler that sends the first message it receives to called:
func recordFatalErrors(called chan string) interfaces.ErrorFunc {
	return func(err error, message string, details map[string]interface{}) {
		select {
		case called <- message:
		default:
		}
	}
}

func TestFatalErrorHandlerAddedToConfigAfterClientIsMadeIsUsed(t *testing.T) {
	client, _ := newTestStreamingClient(t)

	// the client has no handler of its own, and the config gets one only now
	called := make(chan string, 1)
	client.config.WithFatalErrorHandler(recordFatalErrors(called))

	assert.NotPanics(t, func() {
		client.handleSSEError(&testEvent{event: string(models.SSEError), data: "bad key"})
	})

	select {
	case message := <-called:
		assert.Equal(t, "Error from API client, stopping", message)
	default:
		t.Fatal("the handler added to the config after the client was made was not called")
	}
}

func TestStreamErrorReconnectsOnServerError(t *testing.T) {
	client, _ := newTestStreamingClient(t)
	client.isRunning.Store(true)

	result := client.handleStreamError(eventsource.SubscriptionError{Code: http.StatusServiceUnavailable})

	assert.False(t, result.CloseNow, "a temporary server error should be retried")
	assert.False(t, client.stopped.Load())
}

func TestStreamErrorStatus236StopsClient(t *testing.T) {
	client, _ := newTestStreamingClient(t)
	client.isRunning.Store(true)

	result := client.handleStreamError(eventsource.SubscriptionError{Code: 236})

	assert.True(t, result.CloseNow, "236 means the server wants us to stop")
	assert.True(t, client.stopped.Load())

	client.Connect()
	assert.False(t, client.isRunning.Load(), "a client stopped by the server cannot be connected again")
}

func TestStreamErrorRejectedRequestStopsClientAndCallsFatalHandler(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client, _ := newTestStreamingClient(t)
			client.isRunning.Store(true)

			called := make(chan string, 1)
			client.WithFatalErrorHandler(recordFatalErrors(called))

			result := client.handleStreamError(eventsource.SubscriptionError{Code: status})

			assert.True(t, result.CloseNow)
			assert.True(t, client.stopped.Load())
			select {
			case message := <-called:
				assert.Contains(t, message, "rejected the SDK key")
			default:
				t.Fatal("the fatal error handler was not called")
			}
		})
	}
}

func TestReconnectAnswered236StopsRealStream(t *testing.T) {
	server := &sseServer{status: func(connection int) int {
		if connection > 1 {
			return 236
		}
		return http.StatusOK
	}}
	ts := server.start(t, func(connection int, w http.ResponseWriter, f http.Flusher, r *http.Request) {
		// a short retry so the reconnect happens quickly, then end the connection
		fmt.Fprint(w, "retry: 10\nid: 1\nevent: features\ndata: [{\"id\":\"1\",\"key\":\"flag\",\"type\":\"BOOLEAN\",\"value\":true,\"version\":1}]\n\n")
		f.Flush()
	})

	client, repo := newSSETestClient(t, ts.URL, nil)

	require.Eventually(t, client.stopped.Load, 3*time.Second, 10*time.Millisecond)

	// no more reconnects after the 236
	time.Sleep(200 * time.Millisecond)
	assert.Len(t, server.connections(), 2)
	assert.True(t, repo.Boolean(context.Background(), "flag", false), "existing data is still served")
}

func TestReconnectAnswered404CallsFatalHandlerAndStopsRealStream(t *testing.T) {
	server := &sseServer{status: func(connection int) int {
		if connection > 1 {
			return http.StatusNotFound
		}
		return http.StatusOK
	}}
	ts := server.start(t, func(connection int, w http.ResponseWriter, f http.Flusher, r *http.Request) {
		fmt.Fprint(w, "retry: 10\n\n")
		f.Flush()
	})

	called := make(chan string, 1)
	client, _ := newSSETestClient(t, ts.URL, func(config *core.Config) {
		config.WithFatalErrorHandler(recordFatalErrors(called))
	})

	select {
	case message := <-called:
		assert.Contains(t, message, "rejected the SDK key")
	case <-time.After(3 * time.Second):
		t.Fatal("the fatal error handler was not called for a 404")
	}

	assert.True(t, client.stopped.Load())
	time.Sleep(200 * time.Millisecond)
	assert.Len(t, server.connections(), 2)
}

func TestEdgeStaleStopsClientPermanently(t *testing.T) {
	client, _ := newTestStreamingClient(t)
	client.isRunning.Store(true)

	client.events <- &testEvent{event: string(models.FHConfig), data: `{"edge.stale":true}`}

	done := make(chan struct{})
	go func() {
		client.handleEvents()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handleEvents did not exit after edge.stale")
	}

	assert.True(t, client.stopped.Load())
	assert.False(t, client.isRunning.Load())

	// the stream has been shut down
	_, ok := <-client.events
	assert.False(t, ok, "Events channel should be closed")

	client.Connect()
	assert.False(t, client.isRunning.Load(), "a client stopped by edge.stale cannot be connected again")
	assert.True(t, client.handleStreamError(eventsource.ErrReadTimeout).CloseNow, "and it must not reconnect")
}

func TestFirstConnectionRejectedReturnsStoppedClient(t *testing.T) {
	for _, status := range []int{236, http.StatusUnauthorized, http.StatusNotFound} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := &sseServer{status: func(connection int) int { return status }}
			ts := server.start(t, func(connection int, w http.ResponseWriter, f http.Flusher, r *http.Request) {})

			called := make(chan string, 1)
			config := core.NewConfig(ts.URL, "default/env/key", nil)
			config.Logger.SetOutput(new(bytes.Buffer))
			config.WithFatalErrorHandler(recordFatalErrors(called))
			repo := core.NewClientFeatureHubRepository(config.Logger)
			config.SetRepository(repo)

			client, err := NewStreamingClient(config, repo)
			require.NoError(t, err, "a client that must never connect is returned, stopped, so it stays stopped")
			require.NotNil(t, client)
			assert.True(t, client.Stopped())

			client.Connect()
			assert.False(t, client.isRunning.Load())
			assert.Len(t, server.connections(), 1)

			// Config.Connect reports this as ErrEdgeStopped, so the handler is not called from inside Connect
			assert.Empty(t, called, "the fatal error handler is not called for the first connection")
		})
	}
}

func TestFirstConnectionServerErrorReturnsError(t *testing.T) {
	server := &sseServer{status: func(connection int) int { return http.StatusServiceUnavailable }}
	ts := server.start(t, func(connection int, w http.ResponseWriter, f http.Flusher, r *http.Request) {})

	config := core.NewConfig(ts.URL, "default/env/key", nil)
	config.Logger.SetOutput(new(bytes.Buffer))
	repo := core.NewClientFeatureHubRepository(config.Logger)
	config.SetRepository(repo)

	// a temporary error can be retried by connecting again, so no client is kept
	client, err := NewStreamingClient(config, repo)
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestDefaultFatalErrorHandlerOnlyLogs(t *testing.T) {
	client, logger := newTestStreamingClient(t)
	logs := new(bytes.Buffer)
	logger.SetOutput(logs)
	exited := false
	logger.ExitFunc = func(int) { exited = true }

	client.handleSSEError(&testEvent{event: string(models.SSEError), data: "bad key"})

	assert.False(t, exited, "the default handler must not exit the host process")
	assert.Contains(t, logs.String(), "Error from API client")
}

func TestFatalErrorHandlerCanBeSetWhileErrorsAreReported(t *testing.T) {
	client, _ := newTestStreamingClient(t)
	client.config.Logger = client.logger

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			client.reportFatalError(nil, "error", nil)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			client.config.WithFatalErrorHandler(func(error, string, map[string]interface{}) {})
			client.WithFatalErrorHandler(func(error, string, map[string]interface{}) {})
		}
	}()
	wg.Wait() // the race detector reports any unsynchronised access
}

func TestStreamHTTPClientLimitsWaitForResponseHeaders(t *testing.T) {
	transport := newSSEHTTPClient().Transport.(*http.Transport)
	assert.Equal(t, sseResponseHeaderTimeout, transport.ResponseHeaderTimeout)
	assert.Zero(t, newSSEHTTPClient().Timeout, "an overall timeout would cut the long-lived stream")
}

func TestErrorAndFailureEventsStopClientEvenWithData(t *testing.T) {
	for _, eventName := range []models.Event{models.SSEError, models.FHFailure} {
		t.Run(string(eventName), func(t *testing.T) {
			client, _ := newTestStreamingClient(t)
			called := make(chan string, 1)
			client.WithFatalErrorHandler(recordFatalErrors(called))
			client.isRunning.Store(true)

			client.events <- &testEvent{event: string(models.FHFeatures), data: `[{"id":"1","key":"flag","type":"BOOLEAN","value":true,"version":1}]`}
			client.events <- &testEvent{event: string(eventName), data: "went wrong"}

			done := make(chan struct{})
			go func() {
				client.handleEvents()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("handleEvents did not exit")
			}

			assert.True(t, client.repository.IsReady())
			assert.True(t, client.Stopped(), "the client disconnects and never retries")
			assert.True(t, client.handleStreamError(eventsource.ErrReadTimeout).CloseNow)
			select {
			case message := <-called:
				assert.Contains(t, message, "stopping")
			default:
				t.Fatal("the fatal error handler was not called")
			}
		})
	}
}

func TestConnectStopsWaitingForDataWhenClientStops(t *testing.T) {
	client, _ := newTestStreamingClient(t)
	wait := 5 * time.Second
	client.config.WaitForData = &wait

	client.events <- &testEvent{event: string(models.FHConfig), data: `{"edge.stale":true}`}

	start := time.Now()
	client.Connect()

	assert.Less(t, time.Since(start), time.Second, "Connect must not wait out WaitForData once the client has stopped")
	assert.True(t, client.Stopped())
}

func TestStreamHTTPClientUsesReplacedDefaultTransport(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("mocked") })

	var client *http.Client
	assert.NotPanics(t, func() { client = newSSEHTTPClient() })
	assert.Nil(t, client.Transport, "a nil Transport means the replaced http.DefaultTransport is used")
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestStreamHTTPClientIsShared(t *testing.T) {
	assert.Same(t, sseHTTPClient(), sseHTTPClient(), "all streaming clients share one connection pool")
}
