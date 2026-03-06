package pollingclient

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logger.SetOutput(new(bytes.Buffer))
	return logger
}

func newTestBase(handler http.HandlerFunc, frequency time.Duration) (*PollingBase, *httptest.Server, [](*models.FeatureEnvironmentCollection)) {
	var received []*models.FeatureEnvironmentCollection
	srv := httptest.NewServer(handler)
	pb := newPollingBase(
		srv.URL+"/features?apiKey=test-key",
		frequency,
		5*time.Second,
		func(envs []*models.FeatureEnvironmentCollection) { received = envs },
		newTestLogger(),
	)
	return pb, srv, received
}

// --- AttributeHeader ---

func TestAttributeHeaderEmpty(t *testing.T) {
	pb := newPollingBase("http://example.com/features?apiKey=x", time.Minute, time.Second, nil, newTestLogger())

	pb.AttributeHeader("")

	pb.mu.Lock()
	defer pb.mu.Unlock()
	assert.Equal(t, "", pb.header)
	assert.Equal(t, "0", pb.shaHeader)
}

func TestAttributeHeaderNonEmpty(t *testing.T) {
	pb := newPollingBase("http://example.com/features?apiKey=x", time.Minute, time.Second, nil, newTestLogger())

	const header = "country=Thailand"
	h := sha256.Sum256([]byte(header))
	expectedSha := base64.URLEncoding.EncodeToString(h[:])

	pb.AttributeHeader(header)

	pb.mu.Lock()
	defer pb.mu.Unlock()
	assert.Equal(t, header, pb.header)
	assert.Equal(t, expectedSha, pb.shaHeader)
}

// --- parseCacheControl ---

func TestParseCacheControlUpdatesFrequency(t *testing.T) {
	pb := newPollingBase("http://example.com/features?apiKey=x", time.Minute, time.Second, nil, newTestLogger())

	pb.parseCacheControl("public, max-age=30")

	assert.Equal(t, 30*time.Second, pb.Frequency())
}

func TestParseCacheControlIgnoresZero(t *testing.T) {
	pb := newPollingBase("http://example.com/features?apiKey=x", time.Minute, time.Second, nil, newTestLogger())

	pb.parseCacheControl("max-age=0")

	assert.Equal(t, time.Minute, pb.Frequency())
}

func TestParseCacheControlIgnoresAbsent(t *testing.T) {
	pb := newPollingBase("http://example.com/features?apiKey=x", time.Minute, time.Second, nil, newTestLogger())

	pb.parseCacheControl("no-cache")

	assert.Equal(t, time.Minute, pb.Frequency())
}

// --- Stop ---

func TestStopMarksStopped(t *testing.T) {
	pb := newPollingBase("http://example.com/features?apiKey=x", time.Minute, time.Second, nil, newTestLogger())

	pb.Stop()

	assert.True(t, pb.Stopped())
}

func TestStopUnblocksWaiters(t *testing.T) {
	pb := newPollingBase("http://example.com/features?apiKey=x", time.Minute, time.Second, nil, newTestLogger())

	// Add waiters directly by simulating busy state.
	ch1 := make(chan error, 1)
	ch2 := make(chan error, 1)
	pb.mu.Lock()
	pb.busy = true
	pb.waiters = []chan error{ch1, ch2}
	pb.mu.Unlock()

	pb.Stop()

	err1 := <-ch1
	err2 := <-ch2
	assert.Error(t, err1)
	assert.Error(t, err2)
}

func TestPollWhenStoppedIsNoop(t *testing.T) {
	pb := newPollingBase("http://example.com/features?apiKey=x", time.Minute, time.Second, nil, newTestLogger())
	pb.Stop()

	err := pb.Poll()

	assert.NoError(t, err)
}

// --- Poll: HTTP responses ---

func TestPollDeliversFeaturesOn200(t *testing.T) {
	envs := []*models.FeatureEnvironmentCollection{
		{ID: "env-1", Features: []*models.FeatureState{
			{Key: "flag", Type: models.TypeBoolean, Value: true, Version: 1},
		}},
	}
	body, _ := json.Marshal(envs)

	var received []*models.FeatureEnvironmentCollection
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	pb := newPollingBase(
		srv.URL+"/features?apiKey=test",
		time.Minute,
		5*time.Second,
		func(e []*models.FeatureEnvironmentCollection) { received = e },
		newTestLogger(),
	)

	err := pb.Poll()

	require.NoError(t, err)
	require.Len(t, received, 1)
	assert.Equal(t, "env-1", received[0].ID)
	assert.False(t, pb.AwaitingFirstPollResult())
}

func TestPollNoCallbackOn304(t *testing.T) {
	callbackCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	pb := newPollingBase(
		srv.URL+"/features?apiKey=test",
		time.Minute,
		5*time.Second,
		func(_ []*models.FeatureEnvironmentCollection) { callbackCalled = true },
		newTestLogger(),
	)

	err := pb.Poll()

	assert.NoError(t, err)
	assert.False(t, callbackCalled)
}

func TestPollReturnsHTTPErrorOn404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	pb := newPollingBase(srv.URL+"/features?apiKey=test", time.Minute, 5*time.Second, func(_ []*models.FeatureEnvironmentCollection) {}, newTestLogger())

	err := pb.Poll()

	var httpErr *HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, 404, httpErr.StatusCode)
}

func TestPollStopsAndDeliversFeaturesOn236(t *testing.T) {
	envs := []*models.FeatureEnvironmentCollection{{ID: "env-1"}}
	body, _ := json.Marshal(envs)
	var received []*models.FeatureEnvironmentCollection

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(236)
		w.Write(body)
	}))
	defer srv.Close()

	pb := newPollingBase(
		srv.URL+"/features?apiKey=test",
		time.Minute,
		5*time.Second,
		func(e []*models.FeatureEnvironmentCollection) { received = e },
		newTestLogger(),
	)

	err := pb.Poll()

	require.NoError(t, err)
	assert.Len(t, received, 1)
	assert.True(t, pb.Stopped())
}

// --- Poll: request headers ---

func TestPollSendsETagOnSubsequentRequest(t *testing.T) {
	callCount := 0
	var receivedEtag string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		receivedEtag = r.Header.Get("if-none-match")
		if callCount == 1 {
			w.Header().Set("ETag", "abc123")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]*models.FeatureEnvironmentCollection{{ID: "e1"}})
		} else {
			w.WriteHeader(http.StatusNotModified)
		}
	}))
	defer srv.Close()

	pb := newPollingBase(
		srv.URL+"/features?apiKey=test",
		time.Minute,
		5*time.Second,
		func(_ []*models.FeatureEnvironmentCollection) {},
		newTestLogger(),
	)

	require.NoError(t, pb.Poll())
	require.NoError(t, pb.Poll())

	assert.Equal(t, "abc123", receivedEtag)
}

func TestPollSendsContextHeader(t *testing.T) {
	var receivedXFH string
	var receivedContextSha string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedXFH = r.Header.Get("x-featurehub")
		receivedContextSha = r.URL.Query().Get("contextSha")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*models.FeatureEnvironmentCollection{})
	}))
	defer srv.Close()

	pb := newPollingBase(srv.URL+"/features?apiKey=test", time.Minute, 5*time.Second, func(_ []*models.FeatureEnvironmentCollection) {}, newTestLogger())
	const header = "userkey=bob"
	pb.AttributeHeader(header)

	require.NoError(t, pb.Poll())

	assert.Equal(t, header, receivedXFH)
	h := sha256.Sum256([]byte(header))
	assert.Equal(t, base64.URLEncoding.EncodeToString(h[:]), receivedContextSha)
}

func TestPollDefaultContextShaIsZero(t *testing.T) {
	var receivedContextSha string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContextSha = r.URL.Query().Get("contextSha")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*models.FeatureEnvironmentCollection{})
	}))
	defer srv.Close()

	pb := newPollingBase(srv.URL+"/features?apiKey=test", time.Minute, 5*time.Second, func(_ []*models.FeatureEnvironmentCollection) {}, newTestLogger())

	require.NoError(t, pb.Poll())

	assert.Equal(t, "0", receivedContextSha)
}

func TestPollCacheControlUpdatesFrequency(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=45")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*models.FeatureEnvironmentCollection{})
	}))
	defer srv.Close()

	pb := newPollingBase(srv.URL+"/features?apiKey=test", time.Minute, 5*time.Second, func(_ []*models.FeatureEnvironmentCollection) {}, newTestLogger())

	require.NoError(t, pb.Poll())

	assert.Equal(t, 45*time.Second, pb.Frequency())
}

// --- Hooks ---

func TestPreloadAbortSkipsPoll(t *testing.T) {
	serverCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	pb := newPollingBase(srv.URL+"/features?apiKey=test", time.Minute, 5*time.Second, func(_ []*models.FeatureEnvironmentCollection) {}, newTestLogger())
	pb.Preload = func(_ *http.Request, _ string) bool { return true }

	err := pb.Poll()

	assert.NoError(t, err)
	assert.False(t, serverCalled)
}

func TestPostloadAbortSuppressesProcessing(t *testing.T) {
	callbackCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*models.FeatureEnvironmentCollection{{ID: "e1"}})
	}))
	defer srv.Close()

	pb := newPollingBase(
		srv.URL+"/features?apiKey=test",
		time.Minute,
		5*time.Second,
		func(_ []*models.FeatureEnvironmentCollection) { callbackCalled = true },
		newTestLogger(),
	)
	pb.Postload = func(_ *http.Response) bool { return true }

	err := pb.Poll()

	assert.NoError(t, err)
	assert.False(t, callbackCalled)
}

func TestPostdecodeAbortSuppressesCallback(t *testing.T) {
	callbackCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*models.FeatureEnvironmentCollection{{ID: "e1"}})
	}))
	defer srv.Close()

	pb := newPollingBase(
		srv.URL+"/features?apiKey=test",
		time.Minute,
		5*time.Second,
		func(_ []*models.FeatureEnvironmentCollection) { callbackCalled = true },
		newTestLogger(),
	)
	pb.Postdecode = func(_ []*models.FeatureEnvironmentCollection) bool { return true }

	err := pb.Poll()

	assert.NoError(t, err)
	assert.False(t, callbackCalled)
}

// --- Concurrency: coalescing ---

func TestPollConcurrentCallsCoalesce(t *testing.T) {
	// gate controls when the server responds, so we can have multiple goroutines
	// queued before any poll completes.
	gate := make(chan struct{})
	serverCallCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-gate
		serverCallCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*models.FeatureEnvironmentCollection{})
	}))
	defer srv.Close()

	callbackCount := 0
	pb := newPollingBase(
		srv.URL+"/features?apiKey=test",
		time.Minute,
		5*time.Second,
		func(_ []*models.FeatureEnvironmentCollection) { callbackCount++ },
		newTestLogger(),
	)

	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pb.Poll()
		}()
	}

	// Wait for the first goroutine to acquire the busy lock, then open the gate.
	time.Sleep(50 * time.Millisecond)
	close(gate)
	wg.Wait()

	assert.Equal(t, 1, serverCallCount, "server should be called exactly once")
}
