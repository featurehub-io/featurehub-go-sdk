package pollingclient

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/sirupsen/logrus"
)

// FeaturesFunc is called with the parsed response from a successful poll.
type FeaturesFunc func(environments []*models.FeatureEnvironmentCollection)

// PollingService is the interface for the low-level HTTP polling mechanism.
type PollingService interface {
	Frequency() time.Duration
	Poll() error
	Stop()
	Stopped() bool
	AttributeHeader(header string)
	Busy() bool
	AwaitingFirstPollResult() bool
}

// HTTPError carries an HTTP status code from a failed poll response, allowing
// callers to distinguish fatal (4xx) from transient errors.
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// PollingBase implements PollingService and handles the low-level HTTP GET polling.
// Concurrent calls to Poll() while a poll is already in flight will coalesce:
// they block and share the result of the single in-flight request.
//
// Hooks (Preload, Postload, Postdecode) are function fields that can be
// replaced to intercept or short-circuit the poll lifecycle.
type PollingBase struct {
	// Preload is called before the HTTP request is sent.
	// Return true to abort the poll without error.
	Preload func(req *http.Request, url string) bool

	// Postload is called immediately after receiving the HTTP response.
	// Return true to abort further processing without error.
	Postload func(resp *http.Response) bool

	// Postdecode is called after the response body has been decoded into environments.
	// Return true to suppress the callback (features will not be delivered to the repository).
	Postdecode func(environments []*models.FeatureEnvironmentCollection) bool

	url       string
	frequency time.Duration
	callback  FeaturesFunc
	logger    *logrus.Logger
	client    *http.Client

	mu                      sync.Mutex
	stopped                 bool
	header                  string
	shaHeader               string
	etag                    string
	busy                    bool
	awaitingFirstPollResult bool
	waiters                 []chan error
}

var _ PollingService = (*PollingBase)(nil)

func newPollingBase(
	url string,
	frequency time.Duration,
	requestTimeout time.Duration,
	callback FeaturesFunc,
	logger *logrus.Logger,
) *PollingBase {
	pb := &PollingBase{
		url:                     url,
		frequency:               frequency,
		callback:                callback,
		shaHeader:               "0",
		awaitingFirstPollResult: true,
		client:                  &http.Client{Timeout: requestTimeout},
		logger:                  logger,
	}
	pb.Preload = func(_ *http.Request, _ string) bool { return false }
	pb.Postload = func(_ *http.Response) bool { return false }
	pb.Postdecode = func(_ []*models.FeatureEnvironmentCollection) bool { return false }
	return pb
}

// Frequency returns the current polling frequency (may be updated from cache-control headers).
func (b *PollingBase) Frequency() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.frequency
}

// Stopped reports whether Stop has been called.
func (b *PollingBase) Stopped() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stopped
}

// Busy reports whether a poll is currently in flight.
func (b *PollingBase) Busy() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.busy
}

// AwaitingFirstPollResult reports whether no poll has successfully completed yet.
func (b *PollingBase) AwaitingFirstPollResult() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.awaitingFirstPollResult
}

// AttributeHeader sets the x-featurehub context header and recomputes its SHA-256
// base64-URL-safe hash (used as the contextSha query parameter).
func (b *PollingBase) AttributeHeader(header string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.header = header
	if header == "" {
		b.shaHeader = "0"
	} else {
		h := sha256.Sum256([]byte(header))
		b.shaHeader = base64.URLEncoding.EncodeToString(h[:])
	}
}

// Stop marks the poller as stopped and unblocks any waiting callers with an error.
func (b *PollingBase) Stop() {
	b.mu.Lock()
	b.stopped = true
	b.busy = false
	waiters := b.waiters
	b.waiters = nil
	b.mu.Unlock()

	for _, ch := range waiters {
		ch <- fmt.Errorf("polling stopped")
		close(ch)
	}
}

// Poll performs a single HTTP GET poll. If a poll is already in flight, the caller
// blocks and receives the same result when the in-flight request completes.
func (b *PollingBase) Poll() error {
	b.mu.Lock()

	if b.busy {
		ch := make(chan error, 1)
		b.waiters = append(b.waiters, ch)
		b.mu.Unlock()
		return <-ch
	}

	if b.stopped {
		b.mu.Unlock()
		return nil
	}

	b.busy = true
	b.mu.Unlock()

	err := b.doPoll()

	b.mu.Lock()
	b.busy = false
	b.awaitingFirstPollResult = false
	waiters := b.waiters
	b.waiters = nil
	b.mu.Unlock()

	for _, ch := range waiters {
		ch <- err
		close(ch)
	}

	return err
}

func (b *PollingBase) doPoll() error {
	b.mu.Lock()
	header := b.header
	shaHeader := b.shaHeader
	etag := b.etag
	b.mu.Unlock()

	// Append contextSha to the URL (base URL already contains ?apiKey=...)
	pollURL := fmt.Sprintf("%s&contextSha=%s", b.url, shaHeader)

	req, err := http.NewRequest(http.MethodGet, pollURL, nil)
	if err != nil {
		return err
	}

	if header != "" {
		req.Header.Set("x-featurehub", header)
	}
	if etag != "" {
		req.Header.Set("if-none-match", etag)
	}

	if b.Preload(req, pollURL) {
		return nil
	}

	b.logger.WithField("url", pollURL).Trace("Polling FeatureHub server")

	resp, err := b.client.Do(req)
	// if we have an error, we have no Body is the assumption
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if b.Postload(resp) {
		return nil
	}

	b.parseCacheControl(resp.Header.Get("Cache-Control"))

	if resp.StatusCode == http.StatusNotModified {
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 236 {
		return &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    resp.Status,
		}
	}

	b.mu.Lock()
	b.etag = resp.Header.Get("ETag")
	b.mu.Unlock()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var environments []*models.FeatureEnvironmentCollection
	if err := json.Unmarshal(body, &environments); err != nil {
		return err
	}

	if !b.Postdecode(environments) {
		b.callback(environments)
	}

	// 236 = server signals no further updates will occur (equivalent to SSE edge.stale)
	if resp.StatusCode == 236 {
		b.mu.Lock()
		b.stopped = true
		b.mu.Unlock()
	}

	return nil
}

var maxAgeRegexp = regexp.MustCompile(`max-age=(\d+)`)

// parseCacheControl updates the polling frequency from the server's cache-control max-age directive.
func (b *PollingBase) parseCacheControl(cacheHeader string) {
	if cacheHeader == "" {
		return
	}
	matches := maxAgeRegexp.FindStringSubmatch(cacheHeader)
	if len(matches) < 2 {
		return
	}
	secs, err := strconv.Atoi(matches[1])
	if err != nil || secs <= 0 {
		return
	}
	b.mu.Lock()
	b.frequency = time.Duration(secs) * time.Second
	b.mu.Unlock()
	b.logger.WithField("frequency", b.frequency).Trace("Updated polling frequency from cache-control")
}
