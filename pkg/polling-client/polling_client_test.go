package pollingclient

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	fherrors "github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPollingService is a PollingService implementation for unit testing
// FeatureHubPollingClient without making real HTTP requests.
type mockPollingService struct {
	mu         sync.Mutex
	pollErr    error
	frequency  time.Duration
	stopped    bool
	busy       bool
	pollCount  int
	lastHeader string
}

func (m *mockPollingService) Poll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pollCount++
	return m.pollErr
}

func (m *mockPollingService) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
}

func (m *mockPollingService) Stopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopped
}

func (m *mockPollingService) Frequency() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.frequency
}

func (m *mockPollingService) AttributeHeader(header string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastHeader = header
}

func (m *mockPollingService) Busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.busy
}

func (m *mockPollingService) AwaitingFirstPollResult() bool {
	return m.pollCount == 0
}

func (m *mockPollingService) getPollCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pollCount
}

// newTestClient builds a FeatureHubPollingClient with a mock PollingService and
// a real ClientFeatureHubRepository.
func newTestClient(active bool, mock *mockPollingService) (*FeatureHubPollingClient, *core.ClientFeatureHubRepository) {
	logger := newTestLogger()
	repo := core.NewClientFeatureHubRepository(logger)

	cfg := &core.Config{Logger: logger}
	if active {
		cfg.ActiveRest(0)
	} else {
		cfg.PassiveRest(0)
	}
	c := &FeatureHubPollingClient{
		config:                  cfg,
		repository:              repo,
		polling:                 mock,
		logger:                  logger,
		startable:               true,
		active:                  active,
		whenPollingCacheExpires: time.Now().Add(-time.Millisecond),
	}
	return c, repo
}

// --- NewPollingClient ---

func TestNewPollingClientRejectsInvalidConfig(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(newTestLogger().Writer())
	config := &core.Config{Logger: logger} // missing SDKKey and ServerAddress

	repo := core.NewClientFeatureHubRepository(logger)
	client, err := NewPollingClient(config, repo)

	assert.Error(t, err)
	assert.Nil(t, client)
}

// --- Connect ---

func TestConnectCallsPollOnce(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour}
	c, _ := newTestClient(true, mock)

	c.Connect()

	assert.Equal(t, 1, mock.getPollCount())
}

func TestConnectActiveSetsTimerAfterSuccessfulPoll(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour} // long timer so it doesn't fire during the test
	c, _ := newTestClient(true, mock)

	c.Connect()

	c.mu.Lock()
	hasTimer := c.currentTimer != nil
	c.mu.Unlock()
	assert.True(t, hasTimer)

	c.Close()
}

func TestConnectPassiveDoesNotSetTimer(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour}
	c, _ := newTestClient(false, mock)

	c.Connect()

	c.mu.Lock()
	hasTimer := c.currentTimer != nil
	c.mu.Unlock()
	assert.False(t, hasTimer)
}

func TestConnectNoOpWhenNotStartable(t *testing.T) {
	mock := &mockPollingService{}
	c, _ := newTestClient(true, mock)
	c.startable = false

	c.Connect()

	assert.Equal(t, 0, mock.getPollCount())
}

// --- Poll ---

func TestPollActiveDedupWhenTimerSet(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour}
	c, _ := newTestClient(true, mock)
	// Simulate a timer already being set.
	c.currentTimer = time.AfterFunc(time.Hour, func() {})
	defer c.Close()

	err := c.Poll()

	assert.NoError(t, err)
	assert.Equal(t, 0, mock.getPollCount())
}

func TestPollActiveDedupWhenBusy(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour, busy: true}
	c, _ := newTestClient(true, mock)

	err := c.Poll()

	assert.NoError(t, err)
	assert.Equal(t, 0, mock.getPollCount())
}

func TestPollPassiveNoOpWhenCacheFresh(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour}
	c, _ := newTestClient(false, mock)
	c.whenPollingCacheExpires = time.Now().Add(time.Hour)

	err := c.Poll()

	assert.NoError(t, err)
	assert.Equal(t, 0, mock.getPollCount())
}

func TestPollPassivePollsWhenCacheExpired(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour}
	c, _ := newTestClient(false, mock)
	c.whenPollingCacheExpires = time.Now().Add(-time.Millisecond)

	err := c.Poll()

	assert.NoError(t, err)
	assert.Eventually(t, func() bool { return mock.getPollCount() == 1 }, time.Second, time.Millisecond)
}

func TestPollReturnsErrorWhenNotStartable(t *testing.T) {
	mock := &mockPollingService{}
	c, _ := newTestClient(true, mock)
	c.startable = false

	err := c.Poll()

	assert.Error(t, err)
}

// --- ContextChange ---

func TestContextChangeSetsHeaderAndPolls(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour}
	c, _ := newTestClient(true, mock)

	c.ContextChange("userkey=alice")

	assert.Eventually(t, func() bool { return mock.getPollCount() == 1 }, time.Second, time.Millisecond)
	mock.mu.Lock()
	assert.Equal(t, "userkey=alice", mock.lastHeader)
	mock.mu.Unlock()
}

// --- Fatal error handling ---

func TestFatal404StopsClient(t *testing.T) {
	mock := &mockPollingService{
		frequency: time.Hour,
		pollErr:   &HTTPError{StatusCode: 404, Message: "404 Not Found"},
	}
	c, _ := newTestClient(true, mock)

	c.pollFunc(nil, nil)

	c.mu.Lock()
	startable := c.startable
	c.mu.Unlock()
	assert.False(t, startable)
	assert.True(t, mock.Stopped())
	assert.ErrorIs(t, c.StopReason(), fherrors.ErrKeyRejected)
}

func TestFatal400StopsClient(t *testing.T) {
	mock := &mockPollingService{
		frequency: time.Hour,
		pollErr:   &HTTPError{StatusCode: 400, Message: "400 Bad Request"},
	}
	c, _ := newTestClient(true, mock)

	c.pollFunc(nil, nil)

	c.mu.Lock()
	startable := c.startable
	c.mu.Unlock()
	assert.False(t, startable)
	assert.True(t, mock.Stopped())
}

func TestFatal404CallsReject(t *testing.T) {
	mock := &mockPollingService{
		frequency: time.Hour,
		pollErr:   &HTTPError{StatusCode: 404, Message: "not found"},
	}
	c, _ := newTestClient(true, mock)

	var rejected error
	c.pollFunc(nil, func(err error) { rejected = err })

	assert.Error(t, rejected)
}

func TestTransientErrorSchedulesRetry(t *testing.T) {
	mock := &mockPollingService{
		frequency: time.Hour,
		pollErr:   fmt.Errorf("connection refused"),
	}
	c, _ := newTestClient(true, mock)

	c.pollFunc(nil, nil)

	// Client should still be startable after a transient error.
	c.mu.Lock()
	startable := c.startable
	c.mu.Unlock()
	assert.True(t, startable)

	// A retry timer should have been set.
	c.mu.Lock()
	hasTimer := c.currentTimer != nil
	c.mu.Unlock()
	assert.True(t, hasTimer)

	c.Close()
}

// --- response: feature flattening ---

func TestResponseFlattensFeatureSetsEnvironmentID(t *testing.T) {
	mock := &mockPollingService{}
	c, repo := newTestClient(true, mock)

	c.response([]*models.FeatureEnvironmentCollection{
		{
			ID: "env-abc",
			Features: []*models.FeatureState{
				{ID: "id-flag-a", Key: "flag-a", Type: models.TypeBoolean, Value: true, Version: 1},
				{ID: "id-flag-b", Key: "flag-b", Type: models.TypeString, Value: "hello", Version: 1},
			},
		},
	})

	fs, _, _, err := repo.GetFeature(context.TODO(), "flag-a")
	require.NoError(t, err)
	assert.Equal(t, "env-abc", fs.EnvironmentID)

	fs, _, _, err = repo.GetFeature(context.TODO(), "flag-b")
	require.NoError(t, err)
	assert.Equal(t, "env-abc", fs.EnvironmentID)
}

func TestResponseMultipleEnvironmentsFlattened(t *testing.T) {
	mock := &mockPollingService{}
	c, repo := newTestClient(true, mock)

	c.response([]*models.FeatureEnvironmentCollection{
		{ID: "env-1", Features: []*models.FeatureState{
			{ID: "id-flag-1", Key: "flag-1", Type: models.TypeBoolean, Value: true, Version: 1},
		}},
		{ID: "env-2", Features: []*models.FeatureState{
			{ID: "id-flag-2", Key: "flag-2", Type: models.TypeString, Value: "x", Version: 1},
		}},
	})

	fs1, _, _, err := repo.GetFeature(context.TODO(), "flag-1")
	require.NoError(t, err)
	assert.Equal(t, "env-1", fs1.EnvironmentID)

	fs2, _, _, err := repo.GetFeature(context.TODO(), "flag-2")
	require.NoError(t, err)
	assert.Equal(t, "env-2", fs2.EnvironmentID)
}

func TestResponseEmptyEnvironmentsStopsClient(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour}
	c, _ := newTestClient(true, mock)

	c.response([]*models.FeatureEnvironmentCollection{})

	c.mu.Lock()
	startable := c.startable
	c.mu.Unlock()
	assert.False(t, startable)
	assert.True(t, mock.Stopped())
	assert.ErrorIs(t, c.StopReason(), fherrors.ErrNoEnvironments)
}

// --- Passive: cache expiry is updated after poll ---

func TestPassiveScheduleNextPollSetsCacheExpiry(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour}
	c, _ := newTestClient(false, mock)
	// Cache is already expired so Poll() will call through.
	c.whenPollingCacheExpires = time.Now().Add(-time.Millisecond)

	before := time.Now()
	c.Poll()

	assert.Eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.whenPollingCacheExpires.After(before)
	}, time.Second, time.Millisecond)
}

// --- timer lifecycle ---

func TestConcurrentPollsKeepOnlyOneTimer(t *testing.T) {
	mock := &mockPollingService{frequency: 40 * time.Millisecond}
	c, _ := newTestClient(true, mock)
	defer c.Close()

	// two polls that both see Busy()==false, as Connect + ContextChange do
	c.pollFunc(nil, nil)
	c.pollFunc(nil, nil)
	assert.Equal(t, 2, mock.getPollCount())

	// with a single timer we get ~1 poll per period; duplicated timers would double it
	time.Sleep(100 * time.Millisecond)
	assert.LessOrEqual(t, mock.getPollCount(), 5)
}

func TestNoTimerScheduledAfterClose(t *testing.T) {
	mock := &mockPollingService{frequency: 10 * time.Millisecond}
	c, _ := newTestClient(true, mock)

	c.Close()
	c.scheduleNextPoll(nil, nil)

	c.mu.Lock()
	assert.Nil(t, c.currentTimer)
	c.mu.Unlock()

	time.Sleep(40 * time.Millisecond)
	assert.Equal(t, 0, mock.getPollCount())
}

func TestNoTimerScheduledWhenPollerStoppedByServer(t *testing.T) {
	mock := &mockPollingService{frequency: 10 * time.Millisecond, stopped: true}
	c, _ := newTestClient(true, mock)

	c.pollFunc(nil, nil)

	c.mu.Lock()
	assert.Nil(t, c.currentTimer)
	c.mu.Unlock()
}

func TestServerStopMakesClientUnstartable(t *testing.T) {
	// a 236 response leaves the poller stopped after a successful poll
	mock := &mockPollingService{frequency: 10 * time.Millisecond, stopped: true}
	c, _ := newTestClient(true, mock)

	c.pollFunc(nil, nil)

	c.mu.Lock()
	assert.False(t, c.startable, "a client stopped by the server cannot be started again")
	c.mu.Unlock()

	pollsBefore := mock.getPollCount()
	c.Connect()
	c.ContextChange("userkey=fred")
	assert.Error(t, c.Poll())

	time.Sleep(40 * time.Millisecond)
	assert.Equal(t, pollsBefore, mock.getPollCount(), "no further polls once the server has told us to stop")
}

func TestFatal401And403StopClient(t *testing.T) {
	for _, status := range []int{401, 403} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			mock := &mockPollingService{
				frequency: time.Hour,
				pollErr:   &HTTPError{StatusCode: status, Message: fmt.Sprint(status)},
			}
			c, _ := newTestClient(true, mock)

			c.pollFunc(nil, nil)

			c.mu.Lock()
			startable := c.startable
			c.mu.Unlock()
			assert.False(t, startable, "a refused request cannot succeed, so polling stops for good")
			assert.True(t, mock.Stopped())
			assert.Error(t, c.Poll())
		})
	}
}

func TestRejectedPollCallsFatalErrorHandler(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour, pollErr: &HTTPError{StatusCode: 401, Message: "401"}}
	c, _ := newTestClient(true, mock)

	var messages []string
	c.config.WithFatalErrorHandler(func(err error, message string, details map[string]interface{}) {
		messages = append(messages, message)
	})

	c.pollFunc(nil, nil)

	assert.Len(t, messages, 1, "a rejected poll after Connect is passed to the fatal error handler")
	assert.True(t, c.Stopped())
}

func TestRejectedFirstPollDuringConnectCallsFatalErrorHandler(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour, pollErr: &HTTPError{StatusCode: 404, Message: "404 Not Found"}}
	c, _ := newTestClient(true, mock)

	calls := 0
	c.config.WithFatalErrorHandler(func(error, string, map[string]interface{}) { calls++ })

	c.Connect()

	assert.Equal(t, 1, calls)
	assert.True(t, c.Stopped())
	assert.Contains(t, c.StopReason().Error(), "404")
	assert.ErrorIs(t, c.StopReason(), fherrors.ErrKeyRejected)
}

func TestRejectedResponseSeenByTwoPollsIsReportedOnce(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour, pollErr: &HTTPError{StatusCode: 401, Message: "401"}}
	c, _ := newTestClient(true, mock)

	calls := 0
	c.config.WithFatalErrorHandler(func(error, string, map[string]interface{}) { calls++ })

	// e.g. Connect's poll and one started by ContextChange both get the rejection
	c.pollFunc(nil, nil)
	c.pollFunc(nil, nil)

	assert.Equal(t, 1, calls)
}

func TestStopReasonForClose(t *testing.T) {
	c, _ := newTestClient(true, &mockPollingService{frequency: time.Hour})
	assert.Nil(t, c.StopReason())

	c.Close()
	assert.ErrorIs(t, c.StopReason(), fherrors.ErrClientClosed)
}

func TestPollingConnectStopsWaitingForDataWhenClientStops(t *testing.T) {
	mock := &mockPollingService{frequency: time.Hour, pollErr: &HTTPError{StatusCode: 404, Message: "404"}}
	c, _ := newTestClient(true, mock)
	wait := 5 * time.Second
	c.config.WaitForData = &wait

	start := time.Now()
	c.Connect()

	assert.Less(t, time.Since(start), time.Second, "Connect must not wait out WaitForData once the client has stopped")
}
