package pollingclient

import (
	"errors"
	"sync"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	fherrors "github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/sirupsen/logrus"
)

var defaultRequestTimeout time.Duration = core.EnvOrDefaultDuration("FEATUREHUB_POLLING_REQ_TIMEOUT", 8*time.Second)

// PollingServiceProvider is a factory for creating a PollingService.
// Replace it (e.g., in tests) to inject a custom or mock implementation.
var PollingServiceProvider func(
	url string,
	frequency time.Duration,
	requestTimeout time.Duration,
	callback FeaturesFunc,
	logger *logrus.Logger,
) PollingService = func(url string, frequency, requestTimeout time.Duration, callback FeaturesFunc, logger *logrus.Logger) PollingService {
	return newPollingBase(url, frequency, requestTimeout, callback, logger)
}

// FeatureHubPollingClient implements interfaces.EdgeClient using periodic HTTP GET requests.
// It supports two modes:
//
//   - Active (EdgeActiveRest): polls on a timer that fires at the configured interval.
//     The interval may be shortened by a server-sent cache-control max-age header.
//   - Passive (EdgePassiveRest): polls on-demand via Poll(); subsequent calls return
//     immediately until the cache has expired.
type FeatureHubPollingClient struct {
	config     *core.Config
	repository interfaces.InternalRepository
	polling    PollingService
	logger     *logrus.Logger

	mu                      sync.Mutex
	startable               bool
	active                  bool
	currentTimer            *time.Timer
	whenPollingCacheExpires time.Time
}

var _ interfaces.EdgeClient = (*FeatureHubPollingClient)(nil)

// NewPollingClient creates a FeatureHubPollingClient from the provided config.
// config.EdgeType() must be core.EdgeActiveRest or core.EdgePassiveRest.
func NewPollingClient(config *core.Config, repository interfaces.InternalRepository) (*FeatureHubPollingClient, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	active := config.EdgeType() != core.EdgePassiveRest

	c := &FeatureHubPollingClient{
		config:                  config,
		repository:              repository,
		logger:                  config.Logger,
		startable:               true,
		active:                  active,
		whenPollingCacheExpires: time.Now().Add(-time.Millisecond * 100), // expired immediately
	}

	c.polling = PollingServiceProvider(
		config.PollingFeaturesURL(),
		config.Timeout(),
		defaultRequestTimeout,
		c.response,
		config.Logger,
	)

	return c, nil
}

// Connect implements interfaces.EdgeClient. It performs an initial synchronous poll
// and, if in active mode, schedules subsequent polls on a timer.
// If config.WaitForData is set, Connect blocks until the repository is ready.
func (c *FeatureHubPollingClient) Connect() {
	c.mu.Lock()
	if !c.startable {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	c.pollFunc(nil, nil)

	if c.config.WaitForData != nil {
		deadline := time.Now().Add(*c.config.WaitForData)
		for !c.repository.IsReady() && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// Poll triggers an immediate poll, respecting active/passive semantics:
//   - Active: no-op if a timer is already pending or a poll is in flight.
//   - Passive: no-op if the cache has not yet expired.
func (c *FeatureHubPollingClient) Poll() error {
	c.mu.Lock()

	if !c.startable {
		c.mu.Unlock()
		return fherrors.NewErrBadConfig("polling stopped: server returned a fatal error")
	}

	// Active: a timer or in-flight poll already covers the next poll.
	if c.active && (c.currentTimer != nil || c.polling.Busy()) {
		c.mu.Unlock()
		return nil
	}

	// Passive: cache is still fresh.
	if !c.active && time.Now().Before(c.whenPollingCacheExpires) {
		c.mu.Unlock()
		return nil
	}

	c.mu.Unlock()

	go c.pollFunc(nil, nil)
	return nil
}

// ContextChange updates the x-featurehub context header and triggers an immediate poll.
// Used for server-side feature evaluation when the user context changes.
func (c *FeatureHubPollingClient) ContextChange(header string) {
	c.polling.AttributeHeader(header)
	go c.pollFunc(nil, nil)
}

// Close stops the polling client and any pending timers.
func (c *FeatureHubPollingClient) Close() {
	c.stop()
}

func (c *FeatureHubPollingClient) stop() {
	c.mu.Lock()
	timer := c.currentTimer
	c.currentTimer = nil
	c.mu.Unlock()

	if timer != nil {
		timer.Stop()
	}
	c.polling.Stop()
}

// pollFunc performs one poll cycle and schedules the next one if appropriate.
// resolve/reject are optional callbacks for callers that need notification.
func (c *FeatureHubPollingClient) pollFunc(resolve func(), reject func(error)) {
	// Capture whether the poll was already busy before we start; busy polls coalesce
	// inside PollingBase.Poll() and we only want to schedule the next poll once.
	pollingBusy := c.polling.Busy()

	err := c.polling.Poll()
	if err != nil {
		if httpErr, ok := errors.AsType[*HTTPError](err); ok && (httpErr.StatusCode == 404 || httpErr.StatusCode == 400) {
			// Fatal: the API key is invalid or not found — stop polling.
			if httpErr.StatusCode == 404 {
				c.logger.Error("The API key provided does not exist, stopping polling.")
			}
			c.mu.Lock()
			c.startable = false
			c.mu.Unlock()
			c.stop()

			if reject != nil {
				reject(err)
			}
			return
		}

		// Transient error (503, network issue, etc.) — schedule a retry.
		c.logger.WithError(err).Warn("Poll failed, will retry")
		// we pass the resolve, reject on to the next scheduled poll so we get a callback when
		// it is successful or fails out completey.
		if !pollingBusy {
			c.scheduleNextPoll(resolve, reject)
		}
		return
	}

	// there was no error, everything OK. If the poll wasn't active before we started then
	// we initiated the poll, then we have to now schedule the next one (if appropriate)
	if !pollingBusy {
		c.scheduleNextPoll(nil, nil)
	}

	// signal back that everything is OK
	if resolve != nil {
		resolve()
	}
}

// scheduleNextPoll sets up the timer for the next active poll, or records the
// passive cache expiry time.
func (c *FeatureHubPollingClient) scheduleNextPoll(
	resolve func(),
	reject func(error),
) {
	frequency := c.polling.Frequency()

	if c.active && frequency > 0 {
		c.mu.Lock()
		c.currentTimer = time.AfterFunc(frequency, func() {
			c.mu.Lock()
			c.currentTimer = nil
			c.mu.Unlock()
			c.pollFunc(resolve, reject)
		})
		c.mu.Unlock()
	} else if !c.active {
		c.mu.Lock()
		c.whenPollingCacheExpires = time.Now().Add(frequency)
		c.mu.Unlock()
		c.logger.WithField("expires", c.whenPollingCacheExpires).Trace("Passive poll: cache expiry set")
	}
}

// response is the callback invoked by PollingBase with a decoded server response.
// It flattens features from all environments and pushes them to the repository.
func (c *FeatureHubPollingClient) response(environments []*models.FeatureEnvironmentCollection) {
	if len(environments) == 0 {
		c.logger.Trace("No environments returned, stopping polling")
		c.mu.Lock()
		c.startable = false
		c.mu.Unlock()
		c.stop()
		return
	}

	var features []*models.FeatureState
	for _, env := range environments {
		for _, f := range env.Features {
			f.EnvironmentID = env.ID
			features = append(features, f)
		}
	}

	c.repository.ProcessFeatures(features)
}
