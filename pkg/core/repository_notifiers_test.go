package core

import (
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestRepositoryNotifiers(t *testing.T) {
	repository := createRepository()

	features, err := featuresFromString(`[{"key":"feature1","type":"NUMBER","value":2}]`)
	assert.NoError(t, err)

	repository.ProcessFeatures(features)

	// Load the repo up with another "features" event, same versions (zero):
	features, _ = featuresFromString(`[{"key":"feature1","type":"NUMBER","value":2}]`)
	repository.ProcessFeatures(features)

	// Load the mock apiClient up with a "feature" event:
	feature, _ := featureFromString(`{"key":"feature2","type":"BOOLEAN","value":true}`)
	assert.NoError(t, err)
	repository.ProcessFeature(feature)

	feature, _ = featureFromString(`{"key":"feature4","type":"NUMBER","value":2}`)
	repository.ProcessFeature(feature)

	// Feature1 gets one notifier:
	var callback1called int
	callbackFunc1 := func(*models.FeatureState) {
		callback1called++
	}

	repository.AddNotifierFeature("feature1", callbackFunc1)

	// Feature 2 gets 2 notifiers (1/2):
	var callback21called int
	callbackFunc21 := func(*models.FeatureState) {
		callback21called++
	}
	feature2UUID1, _ := repository.AddNotifierFeature("feature2", callbackFunc21)

	// Feature 2 gets 2 notifiers (2/2):
	var callback22called int
	callbackFunc22 := func(*models.FeatureState) {
		callback22called++
	}
	feature2UUID2, _ := repository.AddNotifierFeature("feature2", callbackFunc22)
	assert.Len(t, repository.notifiers["feature2"], 2)
	assert.NotEqual(t, feature2UUID1, feature2UUID2)

	// Feature3 gets 1 notifer, but we'll delete it before it gets called:
	var callback3called int
	callbackFunc3 := func(*models.FeatureState) {
		callback3called++
	}

	feature2Deletable, _ := repository.AddNotifierFeature("feature2", callbackFunc22)

	// we are allowed to attach a callback to a feature that does not exist yet
	_, feature3err := repository.AddNotifierFeature("feature3", callbackFunc3)
	assert.NoError(t, feature3err)

	// Feature4 gets a notifier with a nil callback:
	_, feature4Err := repository.AddNotifierFeature("feature4", nil)
	assert.Error(t, feature4Err)

	// Prove that we've added the notifiers:
	assert.Len(t, repository.notifiers, 3)
	//assert.Contains(t, logBuffer.String(), "Added a notifier")

	// Delete some notifiers:
	assert.NoError(t, repository.DeleteNotifier("feature2", feature2Deletable))
	err = repository.DeleteNotifier("feature5", "123")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrNotifierNotFound{}, err)

	// Add a readiness-listener:
	var readinessListenerCalled = false
	callbackReadiness := func() {
		readinessListenerCalled = true
	}
	repository.ReadinessListener(callbackReadiness)

	// Give the notifiers some time to think about what they've done:
	time.Sleep(250 * time.Millisecond)

	// Check that the correct callbacks were made:
	assert.Equal(t, 1, callback1called)
	assert.Equal(t, 1, callback21called)
	assert.Equal(t, 2, callback22called) // once each when added, but then the 2nd one was deleted but it was still called on adding
	assert.Equal(t, 0, callback3called)

	// Add a BOOLEAN callback:
	var callbackBooleanValue = false
	callbackBoolean := func(value bool) {
		callbackBooleanValue = value
	}
	repository.AddNotifierBoolean("booleanfeature", callbackBoolean)

	feature, _ = featureFromString(`{"key":"booleanfeature","type":"BOOLEAN","value":true}`)
	repository.ProcessFeature(feature)

	// Add a JSON callback:
	var callbackJSONValue = `{}`
	callbackJSON := func(value string) {
		callbackJSONValue = value
	}
	repository.AddNotifierJSON("jsonfeature", callbackJSON)

	// Load the mock apiClient up with a "feature" event:
	feature, _ = featureFromString(`{"key":"jsonfeature","type":"JSON","value":"{\"is_crufty\": true}"}`)
	repository.ProcessFeature(feature)

	// Add a NUMBER callback:
	var callbackNumberValue float64 = 0
	callbackNumber := func(value float64) {
		callbackNumberValue = value
	}
	repository.AddNotifierNumber("numberfeature", callbackNumber)

	// Load the mock apiClient up with a "feature" event:
	feature, _ = featureFromString(`{"key":"numberfeature","type":"NUMBER","value":123456789}`)
	repository.ProcessFeature(feature)

	// Add a STRING callback:
	var callbackStringValue = `{}`
	callbackString := func(value string) {
		callbackStringValue = value
	}
	repository.AddNotifierString("stringfeature", callbackString)

	feature, _ = featureFromString(`{"key":"stringfeature","type":"STRING","value":"this is a string"}`)
	repository.ProcessFeature(feature)

	// Give the notifiers some time to think about what they've done:
	time.Sleep(250 * time.Millisecond)

	// Check that the callback functions were all triggered (with the correct values):
	assert.Equal(t, true, callbackBooleanValue)
	assert.Equal(t, `{"is_crufty": true}`, callbackJSONValue)
	assert.Equal(t, float64(123456789), callbackNumberValue)
	assert.Equal(t, "this is a string", callbackStringValue)

	// Check that the repository triggered the readiness listener:
	assert.True(t, readinessListenerCalled)
	//assert.Contains(t, logBuffer.String(), "Calling readinessListener()")
}
