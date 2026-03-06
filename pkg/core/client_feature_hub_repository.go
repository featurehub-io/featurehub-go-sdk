package core

import (
	"fmt"
	"sync"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ClientFeatureHubRepository holds the local cache of features, notifiers, and readiness state:
type ClientFeatureHubRepository struct {
	features          map[string]*models.FeatureState
	featuresMutex     sync.Mutex
	hasData           bool
	logger            *logrus.Logger
	notifiers         notifiers
	notifiersMutex    sync.Mutex
	readinessListener func()
}

func NewClientFeatureHubRepository(logger *logrus.Logger) *ClientFeatureHubRepository {
	return &ClientFeatureHubRepository{
		features:  make(map[string]*models.FeatureState),
		logger:    logger,
		notifiers: make(notifiers),
	}
}

// ReadinessListener defines a callback function which will be triggered once the repository has received data for the first time:
func (r *ClientFeatureHubRepository) ReadinessListener(callbackFunc func()) {
	r.readinessListener = callbackFunc

	// if we are ready, we should callback
	if r.IsReady() {
		callbackFunc()
	}
}

func (r *ClientFeatureHubRepository) IsReady() bool {
	return r.hasData
}

// isReady triggers various notifications that the repository is ready to serve data:
func (r *ClientFeatureHubRepository) isReady() {
	if !r.hasData {
		r.hasData = true
		if r.readinessListener != nil {
			r.logger.Trace("Calling readinessListener()")
			r.readinessListener()
		} else {
			r.logger.Trace("No registered readinessListener() to call")
		}
	}
}

func (r *ClientFeatureHubRepository) WithContext(context *models.Context) interfaces.Context {
	return &ClientWithContext{
		Context:    context,
		repository: r,
	}
}

// GetFeature searches for a feature by key:
func (r *ClientFeatureHubRepository) GetFeature(key string) (*models.FeatureState, error) {
	r.featuresMutex.Lock()
	defer r.featuresMutex.Unlock()

	if feature, ok := r.features[key]; ok {
		r.logger.WithField("key", key).Trace("Found feature")
		return feature, nil
	}

	r.logger.WithField("key", key).Trace("Feature not found")
	return nil, errors.NewErrFeatureNotFound(key)
}

// GetBoolean searches for a feature by key, returns the value as a boolean:
func (r *ClientFeatureHubRepository) GetBoolean(key string) (bool, error) {
	feature, err := r.GetFeature(key)
	if err != nil {
		return false, err
	}
	return feature.AsBoolean()
}

// GetNumber searches for a feature by key, returns the value as a float64:
func (r *ClientFeatureHubRepository) GetNumber(key string) (float64, error) {
	feature, err := r.GetFeature(key)
	if err != nil {
		return 0, err
	}
	return feature.AsNumber()
}

// GetRawJSON searches for a feature by key, returns the value as a JSON string:
func (r *ClientFeatureHubRepository) GetRawJSON(key string) (string, error) {
	feature, err := r.GetFeature(key)
	if err != nil {
		return "{}", err
	}
	return feature.AsRawJSON()
}

// GetString searches for a feature by key, returns the value as a string:
func (r *ClientFeatureHubRepository) GetString(key string) (string, error) {
	feature, err := r.GetFeature(key)
	if err != nil {
		return "", err
	}
	return feature.AsString()
}

func (r *ClientFeatureHubRepository) AddNotifierFeature(featureKey string, callbackFunc models.CallbackFuncFeature) (string, error) {
	if callbackFunc == nil {
		return "", errors.NewErrInvalidNotifierCallback("AddNotifierFeature needs a callbackFunc")
	}

	return r.addNotifier(notifier{
		callbackFuncFeature: callbackFunc,
		featureKey:          featureKey,
	}, models.TypeFeature)
}

// AddNotifierBoolean adds a notifier callback for a BOOLEAN feature:
func (r *ClientFeatureHubRepository) AddNotifierBoolean(featureKey string, callbackFunc models.CallbackFuncBoolean) (string, error) {
	if callbackFunc == nil {
		return "", errors.NewErrInvalidNotifierCallback("AddNotifierBoolean needs a callbackFunc")
	}

	return r.addNotifier(notifier{
		callbackFuncBoolean: callbackFunc,
		featureKey:          featureKey,
	}, models.TypeBoolean)
}

// AddNotifierJSON adds a notifier callback for a JSON feature:
func (r *ClientFeatureHubRepository) AddNotifierJSON(featureKey string, callbackFunc models.CallbackFuncJSON) (string, error) {
	if callbackFunc == nil {
		return "", errors.NewErrInvalidNotifierCallback("AddNotifierJSON needs a callbackFunc")
	}

	return r.addNotifier(notifier{
		callbackFuncJSON: callbackFunc,
		featureKey:       featureKey,
	}, models.TypeJSON)
}

// AddNotifierNumber adds a notifier callback for a NUMBER feature:
func (r *ClientFeatureHubRepository) AddNotifierNumber(featureKey string, callbackFunc models.CallbackFuncNumber) (string, error) {
	if callbackFunc == nil {
		return "", errors.NewErrInvalidNotifierCallback("AddNotifierNumber needs a callbackFunc")
	}

	return r.addNotifier(notifier{
		callbackFuncNumber: callbackFunc,
		featureKey:         featureKey,
	}, models.TypeNumber)
}

// AddNotifierString adds a notifier callback for a STRING feature:
func (r *ClientFeatureHubRepository) AddNotifierString(featureKey string, callbackFunc models.CallbackFuncString) (string, error) {
	if callbackFunc == nil {
		return "", errors.NewErrInvalidNotifierCallback("AddNotifierString needs a callbackFunc")
	}

	return r.addNotifier(notifier{
		callbackFuncString: callbackFunc,
		featureKey:         featureKey,
	}, models.TypeString)
}

// addNotifier
// for historical reasons we can attach to features we don't have, but it means more checking when we do the  triggering
// that we have the right feature type that matches the right notifier callback (see notify function)
func (r *ClientFeatureHubRepository) addNotifier(newNotifier notifier, expectedValueType models.FeatureValueType) (string, error) {
	feature, _ := r.GetFeature(newNotifier.getKey())

	if feature != nil && expectedValueType != models.TypeFeature && feature.Type != expectedValueType {
		return "", errors.NewErrInvalidType("feature is not expected type")
	}

	r.notifiersMutex.Lock()
	defer r.notifiersMutex.Unlock()

	notifierUUID := uuid.New()
	newNotifier.uuid = notifierUUID.String()
	r.notifiers.add(newNotifier)
	r.logger.WithField("key", newNotifier.featureKey).WithField("uuid", newNotifier.uuid).Debug("Added a notifier")

	if feature != nil {
		newNotifier.notify(feature)
	}

	return newNotifier.uuid, nil
}

// DeleteNotifier removes a notifier callback:
func (r *ClientFeatureHubRepository) DeleteNotifier(featureKey, notifierUUID string) error {
	r.notifiersMutex.Lock()
	defer r.notifiersMutex.Unlock()

	featureKeyNotifiers, featureKeyExists := r.notifiers[featureKey]
	if !featureKeyExists {
		err := errors.NewErrNotifierNotFound(featureKey)
		r.logger.WithError(err).WithField("key", featureKey).Error("Attempt to delete a notifier that doesn't exist")
		return err
	}

	if _, notifierExists := featureKeyNotifiers[notifierUUID]; !notifierExists {
		err := errors.NewErrNotifierNotFound(fmt.Sprintf("%s/%s", featureKey, notifierUUID))
		r.logger.WithError(err).WithField("key", featureKey).Error("Attempt to delete a notifier that doesn't exist")
		return err
	}

	delete(r.notifiers[featureKey], notifierUUID)
	r.logger.WithField("key", featureKey).WithField("uuid", notifierUUID).Debug("Deleted a notifier")
	return nil
}

// ProcessFeature updates a single feature from an SSE event payload (version-aware):
func (r *ClientFeatureHubRepository) ProcessFeature(feature *models.FeatureState) {

	r.featuresMutex.Lock()
	defer r.featuresMutex.Unlock()
	if currentFeature, ok := r.features[feature.Key]; ok {
		if feature.Version <= currentFeature.Version {
			r.logger.WithField("key", feature.Key).Debug("Received an old feature from server")
			return
		}
	}

	r.logger.WithField("key", feature.Key).Debug("Received a new feature from server")
	r.features[feature.Key] = feature
	r.notify(feature)
	r.isReady()
}

// ProcessFeatures replaces the entire feature set from an SSE event payload:
func (r *ClientFeatureHubRepository) ProcessFeatures(features []*models.FeatureState) {

	newFeatures := make(map[string]*models.FeatureState)
	for _, f := range features {
		newFeatures[f.Key] = f
	}

	r.featuresMutex.Lock()
	oldFeatures := r.features
	r.features = newFeatures
	r.isReady()
	r.featuresMutex.Unlock()

	for _, newFeature := range newFeatures {
		if oldFeature, ok := oldFeatures[newFeature.Key]; ok {
			if newFeature.Version <= oldFeature.Version {
				continue
			}
		}
		r.notify(newFeature)
	}

	r.hasData = true

	r.logger.Debugf("Received %d features from server", len(features))
}

// ProcessDeleteFeature removes a feature from the cache from an SSE event payload:
func (r *ClientFeatureHubRepository) ProcessDeleteFeature(feature *models.FeatureState) {

	r.featuresMutex.Lock()
	defer r.featuresMutex.Unlock()
	delete(r.features, feature.Key)
	r.logger.WithField("key", feature.Key).Debug("Deleted a feature")
}

// notify triggers all callbacks registered for the given feature:
func (r *ClientFeatureHubRepository) notify(feature *models.FeatureState) error {
	r.notifiersMutex.Lock()
	defer r.notifiersMutex.Unlock()

	featureKeyNotifiers, featureKeyExists := r.notifiers[feature.Key]
	if !featureKeyExists {
		err := errors.NewErrNotifierNotFound(feature.Key)
		r.logger.WithError(err).WithField("key", feature.Key).Trace("Attempt to call a notifier that doesn't exist")
		return err
	}

	for _, notifier := range featureKeyNotifiers {
		notifier.notify(feature)
		r.logger.WithField("key", feature.Key).WithField("uuid", notifier.uuid).Debug("Triggered a notifier")
	}

	return nil
}
