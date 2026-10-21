package streamingclient

import (
	"context"
	goerrors "errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/launchdarkly/eventsource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func countFatalErrors(client *StreamingClient) *atomic.Int32 {
	calls := new(atomic.Int32)
	client.WithFatalErrorHandler(func(error, string, map[string]interface{}) { calls.Add(1) })
	return calls
}

func TestStopReportsOnlyTheFirstStop(t *testing.T) {
	client, _ := newTestStreamingClient(t)
	assert.True(t, client.stop(errors.ErrClientClosed))
	assert.False(t, client.stop(errors.NewStopReason(errors.ErrServerError, "")))
	assert.ErrorIs(t, client.StopReason(), errors.ErrClientClosed)
}

func TestRejectionAfterCloseDoesNotCallFatalHandler(t *testing.T) {
	client, _ := newTestStreamingClient(t)
	calls := countFatalErrors(client)

	client.Close()
	// e.g. a 401 that was already being handled when the client was closed
	client.stopForStatus(eventsource.SubscriptionError{Code: http.StatusUnauthorized, Message: "Unauthorized"})

	assert.Zero(t, calls.Load(), "the client was closed by the user, not by the rejection")
	assert.ErrorIs(t, client.StopReason(), errors.ErrClientClosed)
}

func TestOnlyTheFirstErrorEventCallsFatalHandler(t *testing.T) {
	client, _ := newTestStreamingClient(t)
	calls := countFatalErrors(client)
	client.isRunning.Store(true)

	client.stopForEvent(&testEvent{event: string(models.SSEError), data: "first"}, "error")
	client.stopForEvent(&testEvent{event: string(models.FHFailure), data: "second"}, "failure")

	assert.Equal(t, int32(1), calls.Load())
	assert.ErrorIs(t, client.StopReason(), errors.ErrServerError)
	assert.Contains(t, client.StopReason().Error(), "first")
}

func TestStoppedClientAlwaysHasItsReason(t *testing.T) {
	for i := 0; i < 200; i++ {
		client, _ := newTestStreamingClient(t)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.stopForEvent(&testEvent{event: string(models.SSEError), data: "boom"}, "error")
		}()
		for !client.Stopped() {
		}
		require.ErrorIs(t, client.StopReason(), errors.ErrServerError, "a stopped client must report the real reason")
		wg.Wait()
	}
}

func TestStopReasonsCanBeMatched(t *testing.T) {
	client, _ := newTestStreamingClient(t)
	client.stopForStatus(eventsource.SubscriptionError{Code: http.StatusForbidden, Message: "Forbidden"})
	assert.ErrorIs(t, client.StopReason(), errors.ErrKeyRejected)
	var apiErr *errors.ErrFromAPI
	assert.ErrorAs(t, client.StopReason(), &apiErr, "stop reasons from the server are still ErrFromAPI")

	stale, _ := newTestStreamingClient(t)
	stale.stopForStatus(eventsource.SubscriptionError{Code: statusStale})
	assert.ErrorIs(t, stale.StopReason(), errors.ErrServerRequestedStop)

	edgeStale, _ := newTestStreamingClient(t)
	edgeStale.handleFHConfigEvent(&testEvent{event: string(models.FHConfig), data: `{"edge.stale":true}`})
	assert.ErrorIs(t, edgeStale.StopReason(), errors.ErrServerRequestedStop)
}

func TestHeaderTimeoutErrorIsADeadline(t *testing.T) {
	transport := headerTimeoutTransport{timeout: 50 * time.Millisecond}
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })

	// a transport that fails with its own error after the timeout has fired
	dnsErr := goerrors.New("no such host")
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, dnsErr
	})
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://featurehub.example/", nil)
	_, err := transport.RoundTrip(req)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.ErrorIs(t, err, dnsErr, "the transport's own error is kept")

	// a transport that just reports the cancellation
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	_, err = transport.RoundTrip(req)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NotErrorIs(t, err, context.Canceled, "the cancellation is not the cause")
}
