package core

import (
	"context"
	"sync"
	"sync/atomic"
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
	var callback1called atomic.Int32
	callbackFunc1 := func(_ context.Context, _ *models.FeatureState) {
		callback1called.Add(1)
	}

	repository.AddNotifierFeature(context.TODO(), "feature1", callbackFunc1)

	// Feature 2 gets 2 notifiers (1/2):
	var callback21called atomic.Int32
	callbackFunc21 := func(_ context.Context, _ *models.FeatureState) {
		callback21called.Add(1)
	}
	feature2UUID1, _ := repository.AddNotifierFeature(context.TODO(), "feature2", callbackFunc21)

	// Feature 2 gets 2 notifiers (2/2):
	var callback22called atomic.Int32
	callbackFunc22 := func(_ context.Context, _ *models.FeatureState) {
		callback22called.Add(1)
	}
	feature2UUID2, _ := repository.AddNotifierFeature(context.TODO(), "feature2", callbackFunc22)
	assert.Len(t, repository.notifiers["feature2"], 2)
	assert.NotEqual(t, feature2UUID1, feature2UUID2)

	// Feature3 gets 1 notifer, but we'll delete it before it gets called:
	var callback3called atomic.Int32
	callbackFunc3 := func(_ context.Context, _ *models.FeatureState) {
		callback3called.Add(1)
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
	var readinessListenerCalled atomic.Bool
	callbackReadiness := func(_ context.Context) {
		readinessListenerCalled.Store(true)
	}
	repository.ReadinessListener(context.TODO(), callbackReadiness)

	// Check that the correct callbacks were made (notifiers are called asynchronously):
	assert.Eventually(t, func() bool {
		// callback22 is called once each when added, the 2nd one was deleted but it was still called on adding
		return callback1called.Load() == 1 && callback21called.Load() == 1 && callback22called.Load() == 2
	}, time.Second, time.Millisecond)
	assert.Equal(t, int32(0), callback3called.Load())

	// the typed notifiers below are called asynchronously, so guard the values they record:
	var mu sync.Mutex

	// Add a BOOLEAN callback:
	var callbackBooleanValue = false
	callbackBoolean := func(_ context.Context, value bool) {
		mu.Lock()
		defer mu.Unlock()
		callbackBooleanValue = value
	}
	repository.AddNotifierBoolean(context.TODO(), "booleanfeature", callbackBoolean)

	feature, _ = featureFromString(`{"id":"id-booleanfeature","key":"booleanfeature","type":"BOOLEAN","value":true}`)
	repository.ProcessFeature(feature)

	// Add a JSON callback:
	var callbackJSONValue = `{}`
	callbackJSON := func(_ context.Context, value string) {
		mu.Lock()
		defer mu.Unlock()
		callbackJSONValue = value
	}
	repository.AddNotifierJSON(context.TODO(), "jsonfeature", callbackJSON)

	// Load the mock apiClient up with a "feature" event:
	feature, _ = featureFromString(`{"id":"id-jsonfeature","key":"jsonfeature","type":"JSON","value":"{\"is_crufty\": true}"}`)
	repository.ProcessFeature(feature)

	// Add a NUMBER callback:
	var callbackNumberValue float64 = 0
	callbackNumber := func(_ context.Context, value float64) {
		mu.Lock()
		defer mu.Unlock()
		callbackNumberValue = value
	}
	repository.AddNotifierNumber(context.TODO(), "numberfeature", callbackNumber)

	// Load the mock apiClient up with a "feature" event:
	feature, _ = featureFromString(`{"id":"id-numberfeature","key":"numberfeature","type":"NUMBER","value":123456789}`)
	repository.ProcessFeature(feature)

	// Add a STRING callback:
	var callbackStringValue = `{}`
	callbackString := func(_ context.Context, value string) {
		mu.Lock()
		defer mu.Unlock()
		callbackStringValue = value
	}
	repository.AddNotifierString(context.TODO(), "stringfeature", callbackString)

	feature, _ = featureFromString(`{"id":"id-stringfeature","key":"stringfeature","type":"STRING","value":"this is a string"}`)
	repository.ProcessFeature(feature)

	// Check that the callback functions were all triggered (with the correct values):
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return callbackBooleanValue &&
			callbackJSONValue == `{"is_crufty": true}` &&
			callbackNumberValue == float64(123456789) &&
			callbackStringValue == "this is a string"
	}, time.Second, time.Millisecond)

	// Check that the repository triggered the readiness listener:
	assert.True(t, readinessListenerCalled.Load())
	//assert.Contains(t, logBuffer.String(), "Calling readinessListener()")
}

func TestFeatureNotifierWhenKeyChanges(t *testing.T) {
	repository := createRepository()

	var mu sync.Mutex
	var callbackStringValue = `{}`
	callbackString := func(_ context.Context, value string) {
		mu.Lock()
		defer mu.Unlock()
		callbackStringValue = value
	}
	stringValue := func() string {
		mu.Lock()
		defer mu.Unlock()
		return callbackStringValue
	}
	repository.AddNotifierString(context.TODO(), "stringfeature", callbackString)

	repository.ProcessFeature(ffs(`{"id":"id-stringfeature","key":"stringfeature","type":"STRING","value":"this is a string","version":1}`))
	fs, _, _, err := repository.GetFeature(context.TODO(), "stringfeature")
	assert.NoError(t, err)
	assert.NotNil(t, fs)

	assert.Eventually(t, func() bool { return stringValue() == "this is a string" }, time.Second, time.Millisecond)

	repository.ProcessFeature(ffs(`{"id":"id-stringfeature","key":"ออม","type":"STRING","value":"this is also","version":2}`))
	assert.Eventually(t, func() bool { return stringValue() == "this is also" }, time.Second, time.Millisecond)
	fs, _, _, err = repository.GetFeature(context.TODO(), "ออม")
	assert.NoError(t, err)
	assert.NotNil(t, fs)
	assert.Equal(t, "ออม", fs.Key)
	assert.Equal(t, "id-stringfeature", fs.ID)
}
