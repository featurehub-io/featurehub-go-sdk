package core

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
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
	readinessListener func(context context.Context)
	valueInterceptors []interfaces.FeatureValueInterceptor
	usageProvider     usage.ProviderFactory
	usageStreams      map[int]usage.StreamHandler
	usageStreamsMu    sync.Mutex
	nextStreamID      int
}

func NewClientFeatureHubRepository(logger *logrus.Logger) *ClientFeatureHubRepository {
	return &ClientFeatureHubRepository{
		features:      make(map[string]*models.FeatureState),
		logger:        logger,
		notifiers:     make(notifiers),
		usageProvider: usage.DefaultProvider,
	}
}

func (r *ClientFeatureHubRepository) UsageProvider() usage.ProviderFactory {
	return r.usageProvider
}

func (r *ClientFeatureHubRepository) RegisterUsageStream(handler usage.StreamHandler) int {
	r.usageStreamsMu.Lock()
	defer r.usageStreamsMu.Unlock()
	if r.usageStreams == nil {
		r.usageStreams = make(map[int]usage.StreamHandler)
	}
	r.nextStreamID++
	r.usageStreams[r.nextStreamID] = handler
	return r.nextStreamID
}

func (r *ClientFeatureHubRepository) RemoveUsageStream(id int) {
	r.usageStreamsMu.Lock()
	defer r.usageStreamsMu.Unlock()
	delete(r.usageStreams, id)
}

func (r *ClientFeatureHubRepository) EmitUsageEvent(context context.Context, event usage.UsageEvent) {
	for _, h := range r.usageStreams {
		h(context, event)
	}
}

// ReadinessListener defines a callback function which will be triggered once the repository has received data for the first time:
func (r *ClientFeatureHubRepository) ReadinessListener(context context.Context, callbackFunc func(context context.Context)) {
	r.readinessListener = callbackFunc

	// if we are ready, we should callback
	if r.IsReady() {
		callbackFunc(context)
	}
}

func (r *ClientFeatureHubRepository) IsReady() bool {
	return r.hasData
}

// makeUsageCollectionEventFromSnapshot builds a collection event from an already-captured
// snapshot of features. The caller must NOT hold featuresMutex.
func (r *ClientFeatureHubRepository) makeUsageCollectionEventFromSnapshot(snapshot map[string]*models.FeatureState) usage.FeaturesCollection {
	ready := r.usageProvider.NewUsageCollectionEvent("system", nil)

	features := make([]*usage.FeatureHubUsageValue, 0, len(snapshot))
	for _, feature := range snapshot {
		features = append(features, r.usageProvider.NewUsageValueFromFeature(feature))
	}

	ready.SetFeatureValues(features)

	return ready
}

func (r *ClientFeatureHubRepository) triggerFullFeatureUsageDrop(snapshot map[string]*models.FeatureState) {
	if len(r.usageStreams) > 0 {
		r.EmitUsageEvent(context.Background(), r.makeUsageCollectionEventFromSnapshot(snapshot))
	}
}

// isReady triggers various notifications that the repository is ready to serve data.
// snapshot is the current feature map, already captured by the caller before calling isReady.
// The caller must NOT hold featuresMutex when calling this.
func (r *ClientFeatureHubRepository) isReady(snapshot map[string]*models.FeatureState) {
	// we only do this once, when it first has data
	if !r.hasData {
		r.hasData = true

		r.triggerFullFeatureUsageDrop(snapshot)

		if r.readinessListener != nil {
			r.logger.Trace("Calling readinessListener()")
			r.readinessListener(context.TODO())
		} else {
			r.logger.Trace("No registered readinessListener() to call")
		}
	}
}

func (r *ClientFeatureHubRepository) WithContext(context *models.Context) interfaces.Context {
	return &ClientWithContext{
		Context:           context,
		repository:        r,
		featureRepository: r,
	}
}

func (r *ClientFeatureHubRepository) AddValueInterceptor(valueInterceptor interfaces.FeatureValueInterceptor) {
	if r.valueInterceptors == nil {
		r.valueInterceptors = make([]interfaces.FeatureValueInterceptor, 0)
	}

	r.valueInterceptors = append(r.valueInterceptors, valueInterceptor)
}

func (r *ClientFeatureHubRepository) AllKeys() []string {
	r.featuresMutex.Lock()

	defer r.featuresMutex.Unlock()

	keys := make([]string, 0, len(r.features))

	for k := range r.features {
		keys = append(keys, k)
	}

	return keys
}

// GetFeature searches for a feature by key:
func (r *ClientFeatureHubRepository) GetFeature(context context.Context, key string) (feature *models.FeatureState, matched bool, value interface{}, err error) {
	r.featuresMutex.Lock()
	defer r.featuresMutex.Unlock()

	feature, ok := r.features[key]

	// regardless of whether we found it or not, we need to walk the interceptors passing what we found
	if r.valueInterceptors != nil {
		for _, valueInterceptor := range r.valueInterceptors {
			if value, matched := valueInterceptor(context, key, feature); matched {
				r.logger.WithField("key", key).Trace("Found matching interceptor")
				return feature, matched, value, nil
			}
		}
	}

	if ok {
		r.logger.WithField("key", key).Trace("Found feature")
		return feature, false, feature.Value, nil
	}

	r.logger.WithField("key", key).Trace("Feature not found")
	return nil, false, nil, errors.NewErrFeatureNotFound(key)
}

func (r *ClientFeatureHubRepository) GetInternalBoolean(context context.Context, key string) (feature *models.FeatureState, matched bool, value bool, err error) {
	fs, matched, valueRaw, err := r.GetFeature(context, key)

	if err != nil {
		return fs, matched, false, err
	}
	if fs != nil && fs.Type != models.TypeBoolean {
		return fs, matched, false, errors.NewErrInvalidType(string(fs.Type))
	}

	if value, ok := valueRaw.(bool); ok {
		return fs, matched, value, nil
	}

	if valueStr, okStr := valueRaw.(string); okStr {
		p, e := strconv.ParseBool(valueStr)
		return fs, matched, p, e
	}

	return fs, matched, false, errors.NewErrInvalidType(key)
}

// GetBoolean searches for a feature by key, returns the value as a boolean:
func (r *ClientFeatureHubRepository) GetBoolean(context context.Context, key string) (bool, error) {
	_, _, value, err := r.GetInternalBoolean(context, key)
	return value, err
}

func (r *ClientFeatureHubRepository) GetInternalNumber(context context.Context, key string) (feature *models.FeatureState, matched bool, value *float64, err error) {
	fs, matched, valueRaw, err := r.GetFeature(context, key)

	if err != nil {
		return fs, matched, nil, err
	}

	if fs != nil && fs.Type != models.TypeNumber {
		return fs, matched, nil, errors.NewErrInvalidType(string(fs.Type))
	}

	if valueRaw == nil {
		return fs, matched, nil, nil
	}

	if value, ok := valueRaw.(float64); ok {
		return fs, matched, &value, nil
	}

	if valueStr, okStr := valueRaw.(string); okStr {
		float, ferr := strconv.ParseFloat(valueStr, 64)
		return fs, matched, &float, ferr
	}

	return fs, matched, nil, errors.NewErrInvalidType(key)
}

// GetNumber searches for a feature by key, returns the value as a float64:
func (r *ClientFeatureHubRepository) GetNumber(context context.Context, key string) (*float64, error) {
	_, _, value, err := r.GetInternalNumber(context, key)
	return value, err
}

func (r *ClientFeatureHubRepository) GetInternalString(context context.Context, key string, expectedType models.FeatureValueType) (feature *models.FeatureState, matched bool, value *string, err error) {
	fs, matched, valueRaw, err := r.GetFeature(context, key)

	if err != nil {
		return fs, matched, nil, err
	}

	if fs != nil && fs.Type != expectedType {
		return fs, matched, nil, errors.NewErrInvalidType(string(expectedType))
	}

	if valueRaw == nil {
		return fs, matched, nil, nil
	}

	if valueStr, okStr := valueRaw.(string); okStr {
		return fs, matched, &valueStr, nil
	}

	// its invalid because it is not nil and is not a string
	return fs, matched, nil, errors.NewErrInvalidType(key)
}

// GetRawJSON searches for a feature by key, returns the value as a JSON string:
func (r *ClientFeatureHubRepository) GetRawJSON(context context.Context, key string) (*string, error) {
	_, _, value, err := r.GetInternalString(context, key, models.TypeJSON)
	return value, err
}

// GetString searches for a feature by key, returns the value as a string:
func (r *ClientFeatureHubRepository) GetString(context context.Context, key string) (*string, error) {
	_, _, value, err := r.GetInternalString(context, key, models.TypeString)
	return value, err
}

func (r *ClientFeatureHubRepository) Boolean(context context.Context, featureKey string, defaultValue bool) bool {
	if f, e := r.GetBoolean(context, featureKey); e == nil {
		return defaultValue
	} else {
		return f
	}
}

func (r *ClientFeatureHubRepository) Number(context context.Context, featureKey string, defaultValue float64) float64 {
	if f, e := r.GetNumber(context, featureKey); e == nil || f == nil {
		return defaultValue
	} else {
		return *f
	}
}

func (r *ClientFeatureHubRepository) JSON(context context.Context, featureKey string, defaultValue string) string {
	if f, e := r.GetRawJSON(context, featureKey); e == nil || f == nil {
		return defaultValue
	} else {
		return *f
	}
}

func (r *ClientFeatureHubRepository) String(context context.Context, featureKey string, defaultValue string) string {
	if f, e := r.GetString(context, featureKey); e == nil || f == nil {
		return defaultValue
	} else {
		return *f
	}
}

func (r *ClientFeatureHubRepository) Properties(context context.Context, featureKey string) map[string]string {
	r.featuresMutex.Lock()
	defer r.featuresMutex.Unlock()

	feature, ok := r.features[featureKey]

	if ok {
		return feature.Properties
	}

	return nil
}

func (r *ClientFeatureHubRepository) AddNotifierFeature(context context.Context, featureKey string, callbackFunc models.CallbackFuncFeature) (string, error) {
	if callbackFunc == nil {
		return "", errors.NewErrInvalidNotifierCallback("AddNotifierFeature needs a callbackFunc")
	}

	return r.addNotifier(context, notifier{
		callbackFuncFeature: callbackFunc,
		featureKey:          featureKey,
	}, models.TypeFeature)
}

// AddNotifierBoolean adds a notifier callback for a BOOLEAN feature:
func (r *ClientFeatureHubRepository) AddNotifierBoolean(context context.Context, featureKey string, callbackFunc models.CallbackFuncBoolean) (string, error) {
	if callbackFunc == nil {
		return "", errors.NewErrInvalidNotifierCallback("AddNotifierBoolean needs a callbackFunc")
	}

	return r.addNotifier(context, notifier{
		callbackFuncBoolean: callbackFunc,
		featureKey:          featureKey,
	}, models.TypeBoolean)
}

// AddNotifierJSON adds a notifier callback for a JSON feature:
func (r *ClientFeatureHubRepository) AddNotifierJSON(context context.Context, featureKey string, callbackFunc models.CallbackFuncJSON) (string, error) {
	if callbackFunc == nil {
		return "", errors.NewErrInvalidNotifierCallback("AddNotifierJSON needs a callbackFunc")
	}

	return r.addNotifier(context, notifier{
		callbackFuncJSON: callbackFunc,
		featureKey:       featureKey,
	}, models.TypeJSON)
}

// AddNotifierNumber adds a notifier callback for a NUMBER feature:
func (r *ClientFeatureHubRepository) AddNotifierNumber(context context.Context, featureKey string, callbackFunc models.CallbackFuncNumber) (string, error) {
	if callbackFunc == nil {
		return "", errors.NewErrInvalidNotifierCallback("AddNotifierNumber needs a callbackFunc")
	}

	return r.addNotifier(context, notifier{
		callbackFuncNumber: callbackFunc,
		featureKey:         featureKey,
	}, models.TypeNumber)
}

// AddNotifierString adds a notifier callback for a STRING feature:
func (r *ClientFeatureHubRepository) AddNotifierString(context context.Context, featureKey string, callbackFunc models.CallbackFuncString) (string, error) {
	if callbackFunc == nil {
		return "", errors.NewErrInvalidNotifierCallback("AddNotifierString needs a callbackFunc")
	}

	return r.addNotifier(context, notifier{
		callbackFuncString: callbackFunc,
		featureKey:         featureKey,
	}, models.TypeString)
}

// addNotifier
// for historical reasons we can attach to features we don't have, but it means more checking when we do the  triggering
// that we have the right feature type that matches the right notifier callback (see notify function)
func (r *ClientFeatureHubRepository) addNotifier(context context.Context, newNotifier notifier, expectedValueType models.FeatureValueType) (string, error) {
	feature, _, _, _ := r.GetFeature(context, newNotifier.getKey())

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
		newNotifier.notify(context, feature)
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

	var currentFeature *models.FeatureState

	for _, findFeature := range r.features {
		if findFeature.ID == feature.ID {
			if findFeature.Key != feature.Key {
				if findFeature.Version > feature.Version {
					r.logger.WithField("key", feature.Key).Debug("Received an old feature from server with a changed key")
					// the existing one is newer, bounce the process request
					return
				}
				// the old feature with this key is not the same
				// remove the old feature key, it will be added back in with the new key later
				delete(r.features, findFeature.Key)

				// the key has changed so we need to change the notifiers as well
				r.notifiersMutex.Lock()

				if foundNotifiers, ok := r.notifiers[findFeature.Key]; ok {
					// swap the notifiers over to the new key
					r.notifiers[feature.Key] = foundNotifiers
					// remove the notifiers from the old key
					delete(r.notifiers, findFeature.Key)
				}

				r.notifiersMutex.Unlock()
			} else {
				currentFeature = findFeature
			}

			break
		}
	}

	if currentFeature != nil && feature.Version <= currentFeature.Version {
		r.logger.WithField("key", feature.Key).Debug("Received an old feature from server")
		return
	}

	r.logger.WithField("key", feature.Key).Debug("Received a new feature from server")
	r.features[feature.Key] = feature
	snapshot := r.features

	r.notify(context.TODO(), feature)
	r.isReady(snapshot)
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
	r.featuresMutex.Unlock()

	r.isReady(newFeatures)

	for _, newFeature := range newFeatures {
		if oldFeature, ok := oldFeatures[newFeature.Key]; ok {
			if newFeature.Version <= oldFeature.Version {
				continue
			}
		}
		r.notify(context.TODO(), newFeature)
	}

	r.hasData = true

	r.logger.Debugf("Received %d features from server", len(features))
}

// ProcessDeleteFeature removes a feature from the cache from an SSE event payload:
func (r *ClientFeatureHubRepository) ProcessDeleteFeature(feature *models.FeatureState) {

	r.featuresMutex.Lock()
	defer r.featuresMutex.Unlock()

	// try and find the feature with the same ID and delete that one,
	// the key doesn't matter
	for _, findFeature := range r.features {
		if findFeature.ID == feature.ID {
			r.logger.WithField("key", findFeature.Key).Debug("Deleted a feature")
			delete(r.features, findFeature.Key)
			break
		}
	}

	r.logger.WithField("key", feature.Key).Debug("Deleted a feature")
}

func (r *ClientFeatureHubRepository) GetFeatures() []*models.FeatureIdentity {
	r.featuresMutex.Lock()

	defer r.featuresMutex.Unlock()

	features := make([]*models.FeatureIdentity, 0, len(r.features))

	for _, f := range r.features {
		features = append(features, &models.FeatureIdentity{ID: f.ID, Key: f.Key, ValueType: f.Type, EnvironmentID: f.EnvironmentID})
	}

	return features
}

// notify triggers all callbacks registered for the given feature:
func (r *ClientFeatureHubRepository) notify(context context.Context, feature *models.FeatureState) error {
	r.notifiersMutex.Lock()
	defer r.notifiersMutex.Unlock()

	featureKeyNotifiers, featureKeyExists := r.notifiers[feature.Key]
	if !featureKeyExists {
		return nil
	}

	for _, notifier := range featureKeyNotifiers {
		notifier.notify(context, feature)
		r.logger.WithField("key", feature.Key).WithField("uuid", notifier.uuid).Debug("Triggered a notifier")
	}

	return nil
}
