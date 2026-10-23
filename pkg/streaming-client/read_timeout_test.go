package streamingclient

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/stretchr/testify/assert"
)

func newTimeoutTestClient(t *testing.T, serverURL string, readTimeout time.Duration) (*StreamingClient, *core.ClientFeatureHubRepository) {
	return newSSETestClient(t, serverURL, func(config *core.Config) { config.WithSSEReadTimeout(readTimeout) })
}

func TestSilentStreamsReconnectWithLastEventIDAfterReadTimeout(t *testing.T) {
	server := &sseServer{}
	ts := server.start(t, func(connection int, w http.ResponseWriter, f http.Flusher, r *http.Request) {
		if connection == 1 {
			// a short retry so the reconnects happen quickly
			fmt.Fprint(w, "retry: 10\nid: 42\nevent: features\ndata: [{\"id\":\"1\",\"key\":\"flag\",\"type\":\"BOOLEAN\",\"value\":true,\"version\":1}]\n\n")
			f.Flush()
		}
		// then go (or stay) quiet, as the server does when the Last-Event-ID is current. The server never ends a
		// connection and sends its headers at once, so each connection can only end with the client's read timeout
		<-r.Context().Done()
	})
	client, repo := newSSETestClient(t, ts.URL, func(config *core.Config) {
		config.WithSSEReadTimeout(100 * time.Millisecond)
		// only to keep the test fast: reconnects with no events back off, doubling up to this
		config.WithSSEMaxRetryDelay(50 * time.Millisecond)
	})

	assert.Eventually(t, func() bool { return len(server.connections()) >= 6 }, 5*time.Second, 10*time.Millisecond)

	// a silent stream is not an error: the client keeps reconnecting (it never stops, so the fatal error handler
	// is never called) and the repository keeps serving its data
	assert.False(t, client.Stopped())
	assert.Nil(t, client.StopReason())
	assert.True(t, repo.IsReady())
	_, _, value, err := repo.GetFeature(context.Background(), "flag")
	assert.NoError(t, err)
	assert.Equal(t, true, value)

	connections := server.connections()
	assert.Equal(t, "", connections[0], "the first connection has no Last-Event-ID")
	for _, lastEventID := range connections[1:] {
		assert.Equal(t, "42", lastEventID, "every reconnect resumes from the last event received")
	}
}

func TestStreamReceivingDataIsNotCutByReadTimeout(t *testing.T) {
	server := &sseServer{}
	ts := server.start(t, func(connection int, w http.ResponseWriter, f http.Flusher, r *http.Request) {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for version := 1; ; version++ {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				// a short retry, so a timeout-triggered reconnect would show up within the test
				fmt.Fprintf(w, "retry: 10\nevent: %s\ndata: {\"id\":\"1\",\"key\":\"flag\",\"type\":\"BOOLEAN\",\"value\":true,\"version\":%d}\n\n", models.FHFeature, version)
				f.Flush()
			}
		}
	})

	_, repo := newTimeoutTestClient(t, ts.URL, 200*time.Millisecond)

	// well past the read timeout, but data keeps arriving so the connection must stay up
	time.Sleep(700 * time.Millisecond)
	assert.Len(t, server.connections(), 1)
	assert.True(t, repo.Boolean(context.Background(), "flag", false))
}

func TestStreamingClientDoesNotModifyDefaultHTTPClient(t *testing.T) {
	server := &sseServer{}
	ts := server.start(t, func(connection int, w http.ResponseWriter, f http.Flusher, r *http.Request) {
		<-r.Context().Done()
	})

	newTimeoutTestClient(t, ts.URL, time.Minute)

	assert.Nil(t, http.DefaultClient.CheckRedirect)
}

func TestSSEReadTimeoutConfig(t *testing.T) {
	t.Setenv("FEATUREHUB_SSE_READ_TIMEOUT", "")
	assert.Equal(t, core.DefaultSSEReadTimeout, core.NewConfig("http://localhost", "default/env/key", nil).SSEReadTimeout())
	assert.Greater(t, core.DefaultSSEReadTimeout, 3*time.Minute)

	// a Config built directly gets the default too
	assert.Equal(t, core.DefaultSSEReadTimeout, (&core.Config{}).SSEReadTimeout())

	// zero (or less) means the default
	assert.Equal(t, core.DefaultSSEReadTimeout, core.NewConfig("http://localhost", "default/env/key", nil).WithSSEReadTimeout(0).SSEReadTimeout())
	assert.Equal(t, core.DefaultSSEReadTimeout, core.NewConfig("http://localhost", "default/env/key", nil).WithSSEReadTimeout(-time.Second).SSEReadTimeout())

	t.Setenv("FEATUREHUB_SSE_READ_TIMEOUT", "10m")
	config := core.NewConfig("http://localhost", "default/env/key", nil)
	assert.Equal(t, 10*time.Minute, config.SSEReadTimeout())

	config.WithSSEReadTimeout(5 * time.Minute)
	assert.Equal(t, 5*time.Minute, config.SSEReadTimeout())
}

func TestSSEMaxRetryDelayConfig(t *testing.T) {
	t.Setenv("FEATUREHUB_SSE_MAX_RETRY_DELAY", "")
	assert.Equal(t, core.DefaultSSEMaxRetryDelay, core.NewConfig("http://localhost", "default/env/key", nil).SSEMaxRetryDelay())
	assert.Equal(t, core.DefaultSSEMaxRetryDelay, (&core.Config{}).SSEMaxRetryDelay())

	t.Setenv("FEATUREHUB_SSE_MAX_RETRY_DELAY", "2m")
	config := core.NewConfig("http://localhost", "default/env/key", nil)
	assert.Equal(t, 2*time.Minute, config.SSEMaxRetryDelay())

	config.WithSSEMaxRetryDelay(30 * time.Second)
	assert.Equal(t, 30*time.Second, config.SSEMaxRetryDelay())

	// zero (or less) means the default
	config.WithSSEMaxRetryDelay(0)
	assert.Equal(t, core.DefaultSSEMaxRetryDelay, config.SSEMaxRetryDelay())
}
