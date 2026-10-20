package streamingclient

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	"github.com/stretchr/testify/require"
)

// sseServer records each connection's Last-Event-ID header and hands the connection to onConnect.
type sseServer struct {
	mu           sync.Mutex
	lastEventIDs []string
	// status, if set, gives the HTTP status for each connection (numbered from 1). Connections given a status
	// other than 200 are answered with just that status, and onConnect is not called.
	status func(connection int) int
}

func (s *sseServer) connections() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.lastEventIDs...)
}

func (s *sseServer) start(t *testing.T, onConnect func(connection int, w http.ResponseWriter, f http.Flusher, r *http.Request)) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.lastEventIDs = append(s.lastEventIDs, r.Header.Get("Last-Event-ID"))
		connection := len(s.lastEventIDs)
		s.mu.Unlock()

		if s.status != nil {
			if status := s.status(connection); status != http.StatusOK {
				w.WriteHeader(status)
				return
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f := w.(http.Flusher)
		f.Flush()

		onConnect(connection, w, f, r)
	}))
	t.Cleanup(server.Close)
	return server
}

// newSSETestClient connects a real StreamingClient to serverURL. configure, if given, can change the config
// before the client is made. The client is closed when the test ends.
func newSSETestClient(t *testing.T, serverURL string, configure func(config *core.Config)) (*StreamingClient, *core.ClientFeatureHubRepository) {
	t.Helper()
	config := core.NewConfig(serverURL, "default/env/key", nil)
	config.Logger.SetOutput(new(bytes.Buffer))
	if configure != nil {
		configure(config)
	}

	repo := core.NewClientFeatureHubRepository(config.Logger)
	config.SetRepository(repo)

	client, err := NewStreamingClient(config, repo)
	require.NoError(t, err)
	client.Connect()
	t.Cleanup(client.Close)

	return client, repo
}
