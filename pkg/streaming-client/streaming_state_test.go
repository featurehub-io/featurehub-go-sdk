package streamingclient

import (
	"bytes"
	"testing"
	"time"

	"github.com/donovanhide/eventsource"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestStreamingClientFeatures(t *testing.T) {

	waitPeriod := time.Second * 10
	// Make a logger:
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logBuffer := new(bytes.Buffer)
	logger.SetOutput(logBuffer)

	// Make a test config (with an incorrect server address):
	config := &core.Config{
		Logger:      logger,
		WaitForData: &waitPeriod,
	}

	repo := core.NewClientFeatureHubRepository(logger)
	config.SetRepository(repo)

	// Use the config to make a new StreamingClient with a mock apiClient
	client := &StreamingClient{
		apiClient: &eventsource.Stream{
			Errors: make(chan error, 100),
			Events: make(chan eventsource.Event, 100),
		},
		config:     config,
		logger:     logger,
		repository: repo,
	}

	// Load the mock apiClient up with a "features" event:
	client.apiClient.Events <- &testEvent{
		data:  `[{"id":"id-bool","key":"booleanfeature","type":"BOOLEAN","value":true},{"id":"id-json","key":"jsonfeature","type":"JSON","value":"{\"is_crufty\": true}"},{"id":"id-num","key":"numberfeature","type":"NUMBER","value":123456789},{"id":"id-str","key":"stringfeature","type":"STRING","value":"this is a string"}]`,
		event: "features",
	}

	// Start handling events: (we have 10 seconds to find it, since its already in the stream should be plenty
	client.Connect()

	// Look for a feature that doesn't exist:
	_, _, _, err := repo.GetFeature("something-that-does-not-exist")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrFeatureNotFound{}, err)

	// Look for a feature that DOES exist:
	feature, _, _, err := repo.GetFeature("stringfeature")
	assert.NoError(t, err)
	assert.Equal(t, models.FeatureValueType("STRING"), feature.Type)
}
