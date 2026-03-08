package core

import (
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
)

// ClientWithContext bundles a Context with a repository:
type ClientWithContext struct {
	*models.Context
	repository        interfaces.RepositoryContext
	featureRepository interfaces.FeatureRepository
}

func (cc *ClientWithContext) Attributes() *models.Context {
	return cc.Context
}

// Repository provides access to the repository:
func (cc *ClientWithContext) Repository() interfaces.RepositoryContext {
	return cc.repository
}

// GetBoolean searches for a feature by key, returns the value as a boolean:
func (cc *ClientWithContext) GetBoolean(key string) (bool, error) {
	// Use the existing GetFeature method:
	fs, matched, value, err := cc.featureRepository.GetInternalBoolean(key, false)
	if err != nil {
		return false, err
	}

	// it was overridden, so return that
	if matched {
		return value, nil
	}

	if fs != nil {
		// Figure out which value to use:
		if calculatedValue := fs.Strategies.Calculate(cc.Context); calculatedValue != nil {
			// Assert the value:
			if strategyValue, ok := calculatedValue.(bool); ok {
				return strategyValue, nil
			}
		}
	}

	// Return the default value as a fall-back:
	return value, nil
}

// GetNumber searches for a feature by key, returns the value as a float64:
func (cc *ClientWithContext) GetNumber(key string) (float64, error) {

	fs, matched, value, err := cc.featureRepository.GetInternalNumber(key, false)
	if err != nil {
		return 0, err
	}

	// it was overridden, so return that
	if matched {
		return value, nil
	}

	if fs != nil {
		// Figure out which value to use:
		if calculatedValue := fs.Strategies.Calculate(cc.Context); calculatedValue != nil {

			// Assert the value:
			if strategyValue, ok := calculatedValue.(float64); ok {
				return strategyValue, nil
			}
		}
	}

	// Return the default value as a fall-back:
	return value, nil
}

func (cc *ClientWithContext) getContextString(key string, valueType models.FeatureValueType) (string, error) {
	fs, matched, value, err := cc.featureRepository.GetInternalString(key, false, valueType)
	if err != nil {
		return "{}", err
	}

	// it was overridden, so return that
	if matched {
		return value, nil
	}

	if fs != nil {
		// Figure out which value to use:
		if calculatedValue := fs.Strategies.Calculate(cc.Context); calculatedValue != nil {

			// Assert the value:
			if strategyValue, ok := calculatedValue.(string); ok {
				return strategyValue, nil
			}
		}
	}

	// Return the default value as a fall-back:
	return value, nil
}

// GetRawJSON searches for a feature by key, returns the value as a JSON string:
func (cc *ClientWithContext) GetRawJSON(key string) (string, error) {
	return cc.getContextString(key, models.TypeJSON)
}

// GetString searches for a feature by key, returns the value as a string:
func (cc *ClientWithContext) GetString(key string) (string, error) {
	return cc.getContextString(key, models.TypeString)
}

// WithContext returns a new clientWithContext:
// - the underlying repository is inherited
// - the context is replaced with the one provided
func (cc *ClientWithContext) WithContext(context *models.Context) interfaces.Context {
	return &ClientWithContext{
		Context:           context,
		repository:        cc.repository,
		featureRepository: cc.featureRepository,
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
