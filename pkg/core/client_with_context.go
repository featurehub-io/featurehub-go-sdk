package core

import (
	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
)

// ClientWithContext bundles a Context with a repository:
type ClientWithContext struct {
	*models.Context
	repository interfaces.Repository
}

func (cc *ClientWithContext) Attributes() *models.Context {
	return cc.Context
}

// Repository provides access to the repository:
func (cc *ClientWithContext) Repository() interfaces.Repository {
	return cc.repository
}

// GetFeature searches for a feature by key:
func (cc *ClientWithContext) GetFeature(key string) (*models.FeatureState, error) {
	return cc.repository.GetFeature(key)
}

// GetBoolean searches for a feature by key, returns the value as a boolean:
func (cc *ClientWithContext) GetBoolean(key string) (bool, error) {

	// Use the existing GetFeature method:
	fs, err := cc.repository.GetFeature(key)
	if err != nil {
		return false, err
	}

	// Make sure the feature is the correct type:
	if fs.Type != models.TypeBoolean {
		return false, errors.NewErrInvalidType(string(fs.Type))
	}

	// Assert the value:
	defaultValue, ok := fs.Value.(bool)
	if !ok {
		return false, errors.NewErrInvalidType("Unable to assert value as a bool")
	}

	// Figure out which value to use:
	if calculatedValue := fs.Strategies.Calculate(cc.Context); calculatedValue != nil {

		// Assert the value:
		if strategyValue, ok := calculatedValue.(bool); ok {
			return strategyValue, nil
		}
	}

	// Return the default value as a fall-back:
	return defaultValue, nil
}

// GetNumber searches for a feature by key, returns the value as a float64:
func (cc *ClientWithContext) GetNumber(key string) (float64, error) {

	// Use the existing GetFeature method:
	fs, err := cc.repository.GetFeature(key)
	if err != nil {
		return 0, err
	}

	// Make sure the feature is the correct type:
	if fs.Type != models.TypeNumber {
		return 0, errors.NewErrInvalidType(string(fs.Type))
	}

	// Assert the value:
	defaultValue, ok := fs.Value.(float64)
	if !ok {
		return 0, errors.NewErrInvalidType("Unable to assert value as a float64")
	}

	// Figure out which value to use:
	if calculatedValue := fs.Strategies.Calculate(cc.Context); calculatedValue != nil {

		// Assert the value:
		if strategyValue, ok := calculatedValue.(float64); ok {
			return strategyValue, nil
		}
	}

	// Return the default value as a fall-back:
	return defaultValue, nil
}

// GetRawJSON searches for a feature by key, returns the value as a JSON string:
func (cc *ClientWithContext) GetRawJSON(key string) (string, error) {

	// Use the existing GetFeature method:
	fs, err := cc.repository.GetFeature(key)
	if err != nil {
		return "{}", err
	}

	// Make sure the feature is the correct type:
	if fs.Type != models.TypeJSON {
		return "{}", errors.NewErrInvalidType(string(fs.Type))
	}

	// Assert the value:
	defaultValue, ok := fs.Value.(string)
	if !ok {
		return "{}", errors.NewErrInvalidType("Unable to assert value as a string")
	}

	// Figure out which value to use:
	if calculatedValue := fs.Strategies.Calculate(cc.Context); calculatedValue != nil {

		// Assert the value:
		if strategyValue, ok := calculatedValue.(string); ok {
			return strategyValue, nil
		}
	}

	// Return the default value as a fall-back:
	return defaultValue, nil
}

// GetString searches for a feature by key, returns the value as a string:
func (cc *ClientWithContext) GetString(key string) (string, error) {

	// Use the existing GetFeature method:
	fs, err := cc.repository.GetFeature(key)
	if err != nil {
		return "", err
	}

	// Make sure the feature is the correct type:
	if fs.Type != models.TypeString {
		return "", errors.NewErrInvalidType(string(fs.Type))
	}

	// Assert the value:
	defaultValue, ok := fs.Value.(string)
	if !ok {
		return "", errors.NewErrInvalidType("Unable to assert value as a string")
	}

	// Figure out which value to use:
	if calculatedValue := fs.Strategies.Calculate(cc.Context); calculatedValue != nil {

		// Assert the value:
		if strategyValue, ok := calculatedValue.(string); ok {
			return strategyValue, nil
		}
	}

	// Return the default value as a fall-back:
	return defaultValue, nil
}

func (cc *ClientWithContext) IsReady() bool {
	return cc.repository.IsReady()
}

// WithContext returns a new clientWithContext:
// - the underlying repository is inherited
// - the context is replaced with the one provided
func (cc *ClientWithContext) WithContext(context *models.Context) interfaces.Context {
	return &ClientWithContext{
		Context:    context,
		repository: cc.repository,
	}
}

func (cc *ClientWithContext) AddNotifierFeature(featureKey string, callbackFunc models.CallbackFuncFeature) (notifierUUID string, err error) {
	return cc.repository.AddNotifierFeature(featureKey, callbackFunc)
}

// AddNotifierBoolean configures a notifier for a BOOLEAN value:
func (cc *ClientWithContext) AddNotifierBoolean(featureKey string, callbackFunc models.CallbackFuncBoolean) (notifierUUID string, err error) {
	return cc.repository.AddNotifierBoolean(featureKey, callbackFunc)
}

// AddNotifierJSON configures a notifier for a JSON value:
func (cc *ClientWithContext) AddNotifierJSON(featureKey string, callbackFunc models.CallbackFuncJSON) (notifierUUID string, err error) {
	return cc.repository.AddNotifierJSON(featureKey, callbackFunc)
}

// AddNotifierNumber configures a notifier for a NUMBER value:
func (cc *ClientWithContext) AddNotifierNumber(featureKey string, callbackFunc models.CallbackFuncNumber) (notifierUUID string, err error) {
	return cc.repository.AddNotifierNumber(featureKey, callbackFunc)
}

// AddNotifierString configures a notifier for a STRING value:
func (cc *ClientWithContext) AddNotifierString(featureKey string, callbackFunc models.CallbackFuncString) (notifierUUID string, err error) {
	return cc.repository.AddNotifierString(featureKey, callbackFunc)
}

// DeleteNotifier removes a previously configured notifier (by key and UUID, because we support more than one notifier per key):
func (cc *ClientWithContext) DeleteNotifier(featureKey, notifierUUID string) error {
	return cc.repository.DeleteNotifier(featureKey, notifierUUID)
}

// ReadinessListener adds a function which will be called when the repository is ready:
func (cc *ClientWithContext) ReadinessListener(callbackFunc func()) {
	cc.repository.ReadinessListener(callbackFunc)
}
