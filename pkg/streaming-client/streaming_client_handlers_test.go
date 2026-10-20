package streamingclient

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestStreamingClientHandlers(t *testing.T) {

	// Make a test config (with an incorrect server address):
	config := &core.Config{
		WaitForData: new(time.Hour),
		SDKKey:      "environment-id/api-key",
	}

	// Make a logger:
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logBuffer := new(bytes.Buffer)
	logger.SetOutput(logBuffer)

	repository := core.NewClientFeatureHubRepository(logger)

	// Use the config to make a new StreamingClient with a mock stream:
	stream := newFakeStream(100)
	client := &StreamingClient{
		cancelRequest: stream.Close,
		events:        stream.events,
		config:        config,
		logger:        logger,
		repository:    repository,
	}

	// Load the mock stream up with a "feature" event:
	client.events <- &testEvent{
		data:  `{"id":"id-anotherfeature","key":"anotherfeature","type":"BOOLEAN","value":false,"version":3}`,
		event: "feature",
	}

	// Load the mock stream up with a "feature" event (but with an out-of-sync version):
	client.events <- &testEvent{
		data:  `{"id":"id-anotherfeature","key":"anotherfeature","type":"BOOLEAN","value":false,"version":2}`,
		event: "feature",
	}

	// Load the mock stream up with a "feature" event (which we'll delete):
	client.events <- &testEvent{
		data:  `{"id":"id-featuretodelete","key":"featuretodelete","type":"BOOLEAN","value":true}`,
		event: "feature",
	}

	// Load the mock stream up with a "delete_feature" event:
	client.events <- &testEvent{
		data:  `{"id":"id-featuretodelete","key":"featuretodelete","type":"BOOLEAN","value":false,"version":2}`,
		event: "delete_feature",
	}

	// Load the mock stream up with a "config_stale" event:
	client.events <- &testEvent{
		data:  `{"edge.stale":true}`,
		event: "config",
	}

	// Start handling events:
	client.Connect()

	// Make sure new features with old versions don't clobber values:
	anotherFeature, _, _, err := repository.GetFeature(context.TODO(), "anotherfeature")
	assert.NoError(t, err)
	assert.Equal(t, int64(3), anotherFeature.Version)
	assert.Equal(t, "environment-id", anotherFeature.EnvironmentID)

	// Make sure features get deleted:
	deletedFeature, _, _, err := repository.GetFeature(context.TODO(), "featuretodelete")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrFeatureNotFound{}, err)
	assert.Nil(t, deletedFeature)
	assert.Contains(t, logBuffer.String(), "Deleted a feature")

	// Check that the client knew not to trigger the readiness listener (because there was none):
	assert.Contains(t, logBuffer.String(), "No registered readinessListener() to call")

	// Check that the client knew not to trigger the readiness listener (because there was none):
	assert.Contains(t, logBuffer.String(), "The FeatureHub server has requested that we close our connection")
}
