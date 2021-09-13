package streamingclient

import (
	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
)

// ClientWithContext bundles a Context with a client:
type ClientWithContext struct {
	*models.Context
	client interfaces.Client
	config *Config
}

// Client provides access to the client:
func (cc *ClientWithContext) Client() interfaces.Client {
	return cc.client
}

// GetFeature searches for a feature by key:
func (cc *ClientWithContext) GetFeature(key string) (*models.FeatureState, error) {
	return cc.client.GetFeature(key)
}

// GetBoolean searches for a feature by key, returns the value as a boolean:
func (cc *ClientWithContext) GetBoolean(key string) (bool, error) {

	// Use the existing GetFeature method:
	fs, err := cc.client.GetFeature(key)
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
	fs, err := cc.client.GetFeature(key)
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
	fs, err := cc.client.GetFeature(key)
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
	fs, err := cc.client.GetFeature(key)
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

// WithContext returns a new clienWithContext:
// - the underlying client is inherited
// - the context is replaced with the one provided
func (cc *ClientWithContext) WithContext(context *models.Context) *ClientWithContext {
	return cc.config.WithContext(context)
}

// AddAnalyticsCollector configures a new analytics collector, add it to the list:
func (cc *ClientWithContext) AddAnalyticsCollector(newAnalyticsCollector interfaces.AnalyticsCollector) {
	cc.client.AddAnalyticsCollector(newAnalyticsCollector)
}

// AddNotifierBoolean configures a notifier for a BOOLEAN value:
func (cc *ClientWithContext) AddNotifierBoolean(featureKey string, callbackFunc models.CallbackFuncBoolean) (notifierUUID string) {
	return cc.client.AddNotifierBoolean(featureKey, callbackFunc)
}

// AddNotifierFeature configures a notifier for a generic feature:
func (cc *ClientWithContext) AddNotifierFeature(featureKey string, callbackFunc models.CallbackFuncFeature) (notifierUUID string) {
	return cc.client.AddNotifierFeature(featureKey, callbackFunc)
}

// AddNotifierJSON configures a notifier for a JSON value:
func (cc *ClientWithContext) AddNotifierJSON(featureKey string, callbackFunc models.CallbackFuncJSON) (notifierUUID string) {
	return cc.client.AddNotifierJSON(featureKey, callbackFunc)
}

// AddNotifierNumber configures a notifier for a NUMBER value:
func (cc *ClientWithContext) AddNotifierNumber(featureKey string, callbackFunc models.CallbackFuncNumber) (notifierUUID string) {
	return cc.client.AddNotifierNumber(featureKey, callbackFunc)
}

// AddNotifierString configures a notifier for a STRING value:
func (cc *ClientWithContext) AddNotifierString(featureKey string, callbackFunc models.CallbackFuncString) (notifierUUID string) {
	return cc.client.AddNotifierString(featureKey, callbackFunc)
}

// DeleteNotifier removes a previously configured notifier (by key and UUID, because we support more than one notifier per key):
func (cc *ClientWithContext) DeleteNotifier(featureKey, notifierUUID string) error {
	return cc.client.DeleteNotifier(featureKey, notifierUUID)
}

// LogAnalyticsEvent sends an analytics event (non-blocking, fire and forget):
func (cc *ClientWithContext) LogAnalyticsEvent(action string, other map[string]string) {
	cc.client.LogAnalyticsEvent(action, other)
}

// LogAnalyticsEventSync sends an analytics event, and wait for it to complete:
func (cc *ClientWithContext) LogAnalyticsEventSync(action string, other map[string]string) error {
	return cc.LogAnalyticsEventSync(action, other)
}

// ReadinessListener adds a function which will be called when the client is ready:
func (cc *ClientWithContext) ReadinessListener(callbackFunc func()) {
	cc.ReadinessListener(callbackFunc)
}
