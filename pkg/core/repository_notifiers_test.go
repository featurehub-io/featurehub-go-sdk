package core

import (
	"context"
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestRepositoryNotifiers(t *testing.T) {
	repository := createRepository()

	features, err := featuresFromString(`[{"id":"id-feature1","key":"feature1","type":"NUMBER","value":2}]`)
	assert.NoError(t, err)

	repository.ProcessFeatures(features)

	// Load the repo up with another "features" event, same versions (zero):
	features, _ = featuresFromString(`[{"id":"id-feature1","key":"feature1","type":"NUMBER","value":2}]`)
	repository.ProcessFeatures(features)

	// Load the mock apiClient up with a "feature" event:
	feature, _ := featureFromString(`{"id":"id-feature2","key":"feature2","type":"BOOLEAN","value":true}`)
	assert.NoError(t, err)
	repository.ProcessFeature(feature)

	feature, _ = featureFromString(`{"id":"id-feature4","key":"feature4","type":"NUMBER","value":2}`)
	repository.ProcessFeature(feature)

	// Feature1 gets one notifier:
	var callback1called int
	callbackFunc1 := func(_ context.Context, _ *models.FeatureState) {
		callback1called++
	}

	repository.AddNotifierFeature(context.TODO(), "feature1", callbackFunc1)

	// Feature 2 gets 2 notifiers (1/2):
	var callback21called int
	callbackFunc21 := func(_ context.Context, _ *models.FeatureState) {
		callback21called++
	}
	feature2UUID1, _ := repository.AddNotifierFeature(context.TODO(), "feature2", callbackFunc21)

	// Feature 2 gets 2 notifiers (2/2):
	var callback22called int
	callbackFunc22 := func(_ context.Context, _ *models.FeatureState) {
		callback22called++
	}
	feature2UUID2, _ := repository.AddNotifierFeature(context.TODO(), "feature2", callbackFunc22)
	assert.Len(t, repository.notifiers["feature2"], 2)
	assert.NotEqual(t, feature2UUID1, feature2UUID2)

	// Feature3 gets 1 notifer, but we'll delete it before it gets called:
	var callback3called int
	callbackFunc3 := func(_ context.Context, _ *models.FeatureState) {
		callback3called++
	}

	feature2Deletable, _ := repository.AddNotifierFeature(context.TODO(), "feature2", callbackFunc22)

	// we are allowed to attach a callback to a feature that does not exist yet
	_, feature3err := repository.AddNotifierFeature(context.TODO(), "feature3", callbackFunc3)
	assert.NoError(t, feature3err)

	// Feature4 gets a notifier with a nil callback:
	_, feature4Err := repository.AddNotifierFeature(context.TODO(), "feature4", nil)
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
	callbackReadiness := func(_ context.Context) {
		readinessListenerCalled = true
	}
	repository.ReadinessListener(context.TODO(), callbackReadiness)

	// Give the notifiers some time to think about what they've done:
	time.Sleep(250 * time.Millisecond)

	// Check that the correct callbacks were made:
	assert.Equal(t, 1, callback1called)
	assert.Equal(t, 1, callback21called)
	assert.Equal(t, 2, callback22called) // once each when added, but then the 2nd one was deleted but it was still called on adding
	assert.Equal(t, 0, callback3called)

	// Add a BOOLEAN callback:
	var callbackBooleanValue = false
	callbackBoolean := func(_ context.Context, value bool) {
		callbackBooleanValue = value
	}
	repository.AddNotifierBoolean(context.TODO(), "booleanfeature", callbackBoolean)

	feature, _ = featureFromString(`{"id":"id-booleanfeature","key":"booleanfeature","type":"BOOLEAN","value":true}`)
	repository.ProcessFeature(feature)

	// Add a JSON callback:
	var callbackJSONValue = `{}`
	callbackJSON := func(_ context.Context, value string) {
		callbackJSONValue = value
	}
	repository.AddNotifierJSON(context.TODO(), "jsonfeature", callbackJSON)

	// Load the mock apiClient up with a "feature" event:
	feature, _ = featureFromString(`{"id":"id-jsonfeature","key":"jsonfeature","type":"JSON","value":"{\"is_crufty\": true}"}`)
	repository.ProcessFeature(feature)

	// Add a NUMBER callback:
	var callbackNumberValue float64 = 0
	callbackNumber := func(_ context.Context, value float64) {
		callbackNumberValue = value
	}
	repository.AddNotifierNumber(context.TODO(), "numberfeature", callbackNumber)

	// Load the mock apiClient up with a "feature" event:
	feature, _ = featureFromString(`{"id":"id-numberfeature","key":"numberfeature","type":"NUMBER","value":123456789}`)
	repository.ProcessFeature(feature)

	// Add a STRING callback:
	var callbackStringValue = `{}`
	callbackString := func(_ context.Context, value string) {
		callbackStringValue = value
	}
	repository.AddNotifierString(context.TODO(), "stringfeature", callbackString)

	feature, _ = featureFromString(`{"id":"id-stringfeature","key":"stringfeature","type":"STRING","value":"this is a string"}`)
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

func TestFeatureNotifierWhenKeyChanges(t *testing.T) {
	repository := createRepository()

	var callbackStringValue = `{}`
	callbackString := func(_ context.Context, value string) {
		callbackStringValue = value
	}
	repository.AddNotifierString(context.TODO(), "stringfeature", callbackString)

	repository.ProcessFeature(ffs(`{"id":"id-stringfeature","key":"stringfeature","type":"STRING","value":"this is a string","version":1}`))
	fs, _, _, err := repository.GetFeature(context.TODO(), "stringfeature")
	assert.NoError(t, err)
	assert.NotNil(t, fs)

	time.Sleep(250 * time.Millisecond)
	assert.Equal(t, "this is a string", callbackStringValue)

	repository.ProcessFeature(ffs(`{"id":"id-stringfeature","key":"ออม","type":"STRING","value":"this is also","version":2}`))
	time.Sleep(250 * time.Millisecond)
	assert.Equal(t, "this is also", callbackStringValue)
	fs, _, _, err = repository.GetFeature(context.TODO(), "ออม")
	assert.NoError(t, err)
	assert.NotNil(t, fs)
	assert.Equal(t, "ออม", fs.Key)
	assert.Equal(t, "id-stringfeature", fs.ID)
}
