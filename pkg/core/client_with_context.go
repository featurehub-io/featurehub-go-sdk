package core

import (
	"context"
	"maps"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
	"github.com/google/uuid"
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

func (cc *ClientWithContext) Boolean(context context.Context, featureKey string, defaultValue bool) bool {
	if f, err := cc.GetBoolean(context, featureKey); err != nil {
		return defaultValue
	} else {
		return f
	}
}

func (cc *ClientWithContext) Number(context context.Context, featureKey string, defaultValue float64) float64 {
	if f, err := cc.GetNumber(context, featureKey); err != nil || f == nil {
		return defaultValue
	} else {
		return *f
	}
}

func (cc *ClientWithContext) JSON(context context.Context, featureKey string, defaultValue string) string {
	if f, err := cc.GetRawJSON(context, featureKey); err != nil || f == nil {
		return defaultValue
	} else {
		return *f
	}
}

func (cc *ClientWithContext) String(context context.Context, featureKey string, defaultValue string) string {
	if f, err := cc.GetString(context, featureKey); err != nil || f == nil {
		return defaultValue
	} else {
		return *f
	}
}

// GetBoolean searches for a feature by key, returns the value as a boolean:
func (cc *ClientWithContext) GetBoolean(context context.Context, key string) (bool, error) {
	// Use the existing GetFeature method:
	fs, matched, value, err := cc.featureRepository.GetInternalBoolean(key)

	if err != nil {
		return false, err
	}
	if matched {
		if fs != nil {
			cc.used(context, key, fs, value)
		}
		return value, nil
	}

	if fs != nil {
		// Figure out which value to use:
		if calculatedValue, ok := fs.Strategies.Calculate(cc.Context, fs.ID); ok {
			if calculatedValue == nil {
				return false, errors.NewErrInvalidType("strategy returned nil for bool value")
			}

			// Assert the value:
			if strategyValue, ok := calculatedValue.(bool); ok {
				cc.used(context, key, fs, strategyValue)

				return strategyValue, nil
			}
		}

		cc.used(context, key, fs, value)
	}

	// Return the default value as a fall-back:
	return value, nil
}

// GetNumber searches for a feature by key, returns the value as a float64:
func (cc *ClientWithContext) GetNumber(context context.Context, key string) (*float64, error) {
	fs, matched, value, err := cc.featureRepository.GetInternalNumber(key)

	if err != nil {
		return nil, err
	}
	if matched {
		if fs != nil {
			cc.used(context, key, fs, value)
		}
		return value, nil
	}

	if fs != nil {
		// Figure out which value to use:
		if calculatedValue, ok := fs.Strategies.Calculate(cc.Context, fs.ID); ok {
			if calculatedValue == nil {
				return nil, nil
			}

			// Assert the value:
			if strategyValue, ok := calculatedValue.(float64); ok {
				cc.used(context, key, fs, strategyValue)

				return &strategyValue, nil
			}
		}

		cc.used(context, key, fs, value)
	}

	// Return the default value as a fall-back:
	return value, nil
}

func (cc *ClientWithContext) getContextString(context context.Context, key string, valueType models.FeatureValueType) (*string, error) {
	fs, matched, value, err := cc.featureRepository.GetInternalString(key, valueType)

	if err != nil {
		return nil, err
	}
	if matched {
		if fs != nil {
			cc.used(context, key, fs, value)
		}
		return value, nil
	}

	if fs != nil {
		// Figure out which value to use:
		if calculatedValue, ok := fs.Strategies.Calculate(cc.Context, fs.ID); ok {
			if calculatedValue == nil {
				return nil, nil
			}

			// Assert the value:
			if strategyValue, ok := calculatedValue.(string); ok {
				cc.used(context, key, fs, strategyValue)

				return &strategyValue, nil
			}
		}

		cc.used(context, key, fs, value)
	}

	// Return the default value as a fall-back:
	return value, nil
}

// GetRawJSON searches for a feature by key, returns the value as a JSON string:
func (cc *ClientWithContext) GetRawJSON(context context.Context, key string) (*string, error) {
	return cc.getContextString(context, key, models.TypeJSON)
}

// GetString searches for a feature by key, returns the value as a string:
func (cc *ClientWithContext) GetString(context context.Context, key string) (*string, error) {
	return cc.getContextString(context, key, models.TypeString)
}

func (cc *ClientWithContext) Properties(context context.Context, featureKey string) map[string]string {
	return cc.repository.Properties(context, featureKey)
}

func (cc *ClientWithContext) GetPercentageAttributes(percentageAttributes []string) string {
	if percentageAttributes == nil || len(percentageAttributes) == 0 {
		if key, ok := cc.UniqueKey(); !ok {
			cc.Context.Session = uuid.New().String()
			return cc.Context.Session
		} else {
			return key
		}
	}

	var pa = ""

	for _, key := range percentageAttributes {
		if len(pa) > 0 {
			pa += "$"
		}

		pa += cc.Context.ForPercentage(key)
	}

	return pa
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

func (cc *ClientWithContext) AddNotifierFeature(context context.Context, featureKey string, callbackFunc models.CallbackFuncFeature) (notifierUUID string, err error) {
	return cc.repository.AddNotifierFeature(context, featureKey, callbackFunc)
}

// AddNotifierBoolean configures a notifier for a BOOLEAN value:
func (cc *ClientWithContext) AddNotifierBoolean(context context.Context, featureKey string, callbackFunc models.CallbackFuncBoolean) (notifierUUID string, err error) {
	return cc.repository.AddNotifierBoolean(context, featureKey, callbackFunc)
}

// AddNotifierJSON configures a notifier for a JSON value:
func (cc *ClientWithContext) AddNotifierJSON(context context.Context, featureKey string, callbackFunc models.CallbackFuncJSON) (notifierUUID string, err error) {
	return cc.repository.AddNotifierJSON(context, featureKey, callbackFunc)
}

// AddNotifierNumber configures a notifier for a NUMBER value:
func (cc *ClientWithContext) AddNotifierNumber(context context.Context, featureKey string, callbackFunc models.CallbackFuncNumber) (notifierUUID string, err error) {
	return cc.repository.AddNotifierNumber(context, featureKey, callbackFunc)
}

// AddNotifierString configures a notifier for a STRING value:
func (cc *ClientWithContext) AddNotifierString(context context.Context, featureKey string, callbackFunc models.CallbackFuncString) (notifierUUID string, err error) {
	return cc.repository.AddNotifierString(context, featureKey, callbackFunc)
}

// DeleteNotifier removes a previously configured notifier (by key and UUID, because we support more than one notifier per key):
func (cc *ClientWithContext) DeleteNotifier(featureKey, notifierUUID string) error {
	return cc.repository.DeleteNotifier(featureKey, notifierUUID)
}

func (cc *ClientWithContext) used(context context.Context, key string, fs *models.FeatureState, value interface{}) {
	userKey, _ := cc.UniqueKey()

	cc.RecordUsageEvent(
		context,
		cc.featureRepository.UsageProvider().NewUsageFeature(
			usage.NewUsageValue(fs.ID, key, fs.EnvironmentID, value, fs.Type),
			cc.fullContext(),
			userKey))
}

/**
 * If you give it an event, it will pass it through the usage plugins and attempt to fill it in with details
 * along the way. There are some useful classes BaseUsageEvent, BaseWithFeature, BaseFeaturesCollection,
 * and BaseCollectionContext that we recommend you use as base classes, as they will have their fields
 * detected and filled in.
 *
 * @param event - something that can have "toMap()" called on it
 */

// RecordUsageEvent (event: any | UsageEvent): any;
func (cc *ClientWithContext) RecordUsageEvent(context context.Context, event usage.UsageEvent) {
	cc.featureRepository.EmitUsageEvent(context, cc.fillEvent(context, event))
}

// GetContextUsage - This gives a full event stuffed with the context and all features
func (cc *ClientWithContext) GetContextUsage(context context.Context) usage.UsageEvent {
	// user key will be filled in when fillEvent is called
	return cc.fillEvent(context, cc.featureRepository.UsageProvider().NewUsageContextCollectionEvent(""))
}

func (cc *ClientWithContext) RecordNamedUsage(context context.Context, name string, additionalParams usage.ContextRecord) {
	cc.RecordUsageEvent(context, cc.fillEvent(context, cc.featureRepository.UsageProvider().NewNamedUsageCollection(name, additionalParams)))
}

func (cc *ClientWithContext) fillEvent(context context.Context, event usage.UsageEvent) usage.UsageEvent {
	if userKey, ok := cc.UniqueKey(); ok {
		event.SetUserKey(userKey)
	}

	if featureValues, ok := event.(usage.FeaturesCollection); ok {
		featureValues.SetFeatureValues(cc.mapRepositoryFeaturesToUsageValues(context))
	}

	if collectionContext, ok := event.(usage.CollectionContext); ok {
		collectionContext.SetContextAttributes(cc.fullContext())
	}

	return event
}

func (cc *ClientWithContext) mapRepositoryFeaturesToUsageValues(context context.Context) []*usage.FeatureHubUsageValue {
	// this is a teeny bit more complicated than other languages as the getting of the value and the evaluation is
	// all done in this class.

	// grab all the feature keys, ids and types
	features := cc.featureRepository.GetFeatures()

	usageValues := make([]*usage.FeatureHubUsageValue, 0)

	var value interface{} = nil
	var ok error = nil
	var found = false

	// walk through collecting values within this context
	for _, feat := range features {
		found = true
		if feat.ValueType == models.TypeNumber {
			value, ok = cc.GetNumber(context, feat.Key)
		} else if feat.ValueType == models.TypeString {
			value, ok = cc.GetString(context, feat.Key)
		} else if feat.ValueType == models.TypeJSON {
			value, ok = cc.GetRawJSON(context, feat.Key)
		} else if feat.ValueType == models.TypeBoolean {
			value, ok = cc.GetBoolean(context, feat.Key)
		} else {
			found = false
		}

		if ok == nil && found {
			usageValues = append(usageValues, usage.NewUsageValue(feat.ID, feat.Key, feat.EnvironmentID, value, feat.ValueType))
		}
	}

	return usageValues
}

func (cc *ClientWithContext) fullContext() usage.ContextRecord {
	record := make(usage.ContextRecord)

	if cc.Context.Device != "" {
		record["device"] = cc.Context.Device
	}
	if cc.Context.Platform != "" {
		record["platform"] = cc.Context.Platform
	}
	if cc.Context.Country != "" {
		record["country"] = cc.Context.Country
	}
	if cc.Context.Version != "" {
		record["version"] = cc.Context.Version
	}
	if cc.Context.Custom != nil {
		maps.Copy(record, cc.Context.Custom)
	}

	return record
}
