package streamingclient

import (
	"bytes"
	"testing"

	"github.com/donovanhide/eventsource"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestStreamingClientFeatures(t *testing.T) {

	// Make a test config (with an incorrect server address):
	config := &Config{
		WaitForData: true,
	}

	// Make a logger:
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logBuffer := new(bytes.Buffer)
	logger.SetOutput(logBuffer)

	// Use the config to make a new StreamingClient with a mock apiClient::
	client := &StreamingClient{
		apiClient: &eventsource.Stream{
			Errors: make(chan error, 100),
			Events: make(chan eventsource.Event, 100),
		},
		config:   config,
		features: make(map[string]*models.FeatureState),
		logger:   logger,
	}

	// Load the mock apiClient up with a "features" event:
	client.apiClient.Events <- &testEvent{
		data:  `[{"key":"booleanfeature","type":"BOOLEAN","value":true},{"key":"jsonfeature","type":"JSON","value":"{\"is_crufty\": true}"},{"key":"numberfeature","type":"NUMBER","value":123456789},{"key":"stringfeature","type":"STRING","value":"this is a string"}]`,
		event: "features",
	}

	// Start handling events:
	client.Start()

	// Look for a feature that doesn't exist:
	_, err := client.GetFeature("something-that-does-not-exist")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrFeatureNotFound{}, err)

	// Look for a feature that DOES exist:
	feature, err := client.GetFeature("stringfeature")
	assert.NoError(t, err)
	assert.Equal(t, models.FeatureValueType("STRING"), feature.Type)

	// Look for a boolean feature that is NOT a boolean:
	booleanFeature, err := client.GetBoolean("stringfeature")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrInvalidType{}, err)

	// Look for a boolean feature that IS a boolean:
	booleanFeature, err = client.GetBoolean("booleanfeature")
	assert.NoError(t, err)
	assert.Equal(t, true, booleanFeature)

	// Look for a json feature that is NOT JSON:
	jsonFeature, err := client.GetRawJSON("numberfeature")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrInvalidType{}, err)

	// Look for a json feature that IS json:
	jsonFeature, err = client.GetRawJSON("jsonfeature")
	assert.NoError(t, err)
	assert.Equal(t, `{"is_crufty": true}`, jsonFeature)

	// Look for a number feature that is NOT a number:
	numberFeature, err := client.GetNumber("stringfeature")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrInvalidType{}, err)

	// Look for a number feature that IS a number:
	numberFeature, err = client.GetNumber("numberfeature")
	assert.NoError(t, err)
	assert.Equal(t, float64(123456789), numberFeature)

	// Look for a string feature that is NOT a string:
	stringFeature, err := client.GetString("numberfeature")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrInvalidType{}, err)

	// Look for a string feature that DOES exist:
	stringFeature, err = client.GetString("stringfeature")
	assert.NoError(t, err)
	assert.Equal(t, "this is a string", stringFeature)
}
