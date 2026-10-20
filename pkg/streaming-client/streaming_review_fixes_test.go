package streamingclient

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func newReviewTestClient(waitFor time.Duration) (*StreamingClient, *core.ClientFeatureHubRepository) {
	logger := logrus.New()
	logger.SetOutput(new(bytes.Buffer))

	config := &core.Config{Logger: logger, WaitForData: &waitFor}
	repo := core.NewClientFeatureHubRepository(logger)
	config.SetRepository(repo)

	stream := newFakeStream(100)
	return &StreamingClient{
		cancelRequest: stream.Close,
		events:        stream.events,
		config:        config,
		logger:        logger,
		repository:    repo,
	}, repo
}

func TestMalformedFeaturesEventDoesNotStopEventHandling(t *testing.T) {
	client, repo := newReviewTestClient(2 * time.Second)

	client.events <- &testEvent{event: "features", data: `not json`}
	client.events <- &testEvent{event: "features", data: `[{"id":"1","key":"flag","type":"BOOLEAN","value":true,"version":1}]`}

	client.Connect()

	assert.True(t, repo.IsReady())
	assert.True(t, repo.Boolean(context.Background(), "flag", false))
}

func TestConnectHonoursWaitForDataTimeout(t *testing.T) {
	client, repo := newReviewTestClient(100 * time.Millisecond)

	start := time.Now()
	client.Connect()

	assert.Less(t, time.Since(start), time.Second)
	assert.False(t, repo.IsReady())
}
