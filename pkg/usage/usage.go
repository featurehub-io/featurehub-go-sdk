package usage

import (
	"context"
	"fmt"
	"sync"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
)

// Note: all NewBaseXXX functions are exposed to easily allow users of the library to create their own descendents
// of the structs to retain their own data.

// ContextRecord is a free-form map for context attributes and additional event data.
type ContextRecord = map[string]interface{}

// ConvertFunc converts a raw feature value of the given type to a string for usage reporting.
// Return an empty string to omit the value.
type ConvertFunc func(value interface{}, valueType models.FeatureValueType) string

var (
	convertMu     sync.RWMutex
	activeConvert ConvertFunc = defaultConvert
)

func defaultConvert(value interface{}, valueType models.FeatureValueType) string {
	if value == nil {
		return ""
	}
	switch valueType {
	case models.TypeBoolean:
		if b, ok := value.(bool); ok {
			if b {
				return "on"
			}
			return "off"
		}
	case models.TypeString:
		if s, ok := value.(string); ok {
			return s
		}
	case models.TypeNumber:
		if n, ok := value.(float64); ok {
			return fmt.Sprintf("%g", n)
		}
	}
	return ""
}

// SetConvertFunc replaces the global feature-value converter. Pass nil to reset to the default.
func SetConvertFunc(fn ConvertFunc) {
	convertMu.Lock()
	defer convertMu.Unlock()
	if fn != nil {
		activeConvert = fn
	} else {
		activeConvert = defaultConvert
	}
}

func convert(value interface{}, valueType models.FeatureValueType) string {
	convertMu.RLock()
	defer convertMu.RUnlock()
	return activeConvert(value, valueType)
}

// FeatureHubUsageValue holds a single feature's ID, key, environment ID, and converted value.
type FeatureHubUsageValue struct {
	ID            string
	Key           string
	EnvironmentID string
	Value         string
	ValueType     models.FeatureValueType
	RawValue      interface{}
}

// NewUsageValue constructs a FeatureHubUsageValue, converting the raw value via the active ConvertFunc.
func NewUsageValue(id, key, environmentID string, value interface{}, valueType models.FeatureValueType) *FeatureHubUsageValue {
	return &FeatureHubUsageValue{
		ID:            id,
		Key:           key,
		EnvironmentID: environmentID,
		Value:         convert(value, valueType),
		ValueType:     valueType,
		RawValue:      value,
	}
}

// NewUsageValueFromFeature constructs a FeatureHubUsageValue from a FeatureState.
func NewUsageValueFromFeature(feature *models.FeatureState) *FeatureHubUsageValue {
	return NewUsageValue(feature.ID, feature.Key, feature.EnvironmentID, feature.Value, feature.Type)
}

// UsageEvent is implemented by all usage event types.
type UsageEvent interface {
	EventName() string
	UserKey() string
	SetAdditionalData(ContextRecord)
	SetUserKey(userKey string)
	CollectUsageRecord() ContextRecord
}

// BaseUsageEvent provides a user key and optional additional data shared by all event types.
type BaseUsageEvent struct {
	userKey        string
	additionalData ContextRecord
}

func NewUsageEvent(userKey string, additionalData ContextRecord) UsageEvent {
	return NewBaseUsageEvent(userKey, additionalData)
}

// NewBaseUsageEvent constructs a BaseUsageEvent.
func NewBaseUsageEvent(userKey string, additionalData ContextRecord) *BaseUsageEvent {
	b := &BaseUsageEvent{userKey: userKey}
	if additionalData != nil {
		b.additionalData = additionalData
	} else {
		b.additionalData = make(ContextRecord)
	}
	return b
}

// UserKey returns the user key.
func (b *BaseUsageEvent) UserKey() string   { return b.userKey }
func (b *BaseUsageEvent) EventName() string { return "usage" }
func (b *BaseUsageEvent) SetUserKey(userKey string) {
	b.userKey = userKey
}

// SetAdditionalData replaces the additional data map.
func (b *BaseUsageEvent) SetAdditionalData(data ContextRecord) {
	if data != nil {
		b.additionalData = data
	} else {
		b.additionalData = make(ContextRecord)
	}
}

// CollectUsageRecord returns a shallow copy of the additional data.
func (b *BaseUsageEvent) CollectUsageRecord() ContextRecord {
	return b.baseRecord()
}

func (b *BaseUsageEvent) baseRecord() ContextRecord {
	result := make(ContextRecord, len(b.additionalData))
	for k, v := range b.additionalData {
		result[k] = v
	}
	return result
}

// BaseWithFeature tracks usage of a single feature. Its EventName is "feature".
type BaseWithFeature struct {
	BaseUsageEvent
	contextAttributes ContextRecord
	feature           *FeatureHubUsageValue
}

type EventWithFeature interface {
	UsageEvent
	SetContextAttributes(contextAttributes ContextRecord)
	SetFeature(feature *FeatureHubUsageValue)
	GetFeature() *FeatureHubUsageValue
}

// NewUsageEventWithFeature constructs a BaseWithFeature.
func NewUsageEventWithFeature(feature *FeatureHubUsageValue, contextAttributes ContextRecord, userKey string) EventWithFeature {
	return newUsageEventWithFeature(feature, contextAttributes, userKey)
}

func newUsageEventWithFeature(feature *FeatureHubUsageValue, contextAttributes ContextRecord, userKey string) *BaseWithFeature {
	return &BaseWithFeature{
		BaseUsageEvent:    *NewBaseUsageEvent(userKey, nil),
		contextAttributes: contextAttributes,
		feature:           feature,
	}
}

// EventName returns "feature".
func (*BaseWithFeature) EventName() string { return "feature" }

// Feature returns the associated FeatureHubUsageValue.
func (e *BaseWithFeature) Feature() *FeatureHubUsageValue           { return e.feature }
func (e *BaseWithFeature) SetFeature(feature *FeatureHubUsageValue) { e.feature = feature }
func (e *BaseWithFeature) GetFeature() *FeatureHubUsageValue        { return e.feature }

// ContextAttributes returns the context attributes.
func (e *BaseWithFeature) ContextAttributes() ContextRecord { return e.contextAttributes }
func (e *BaseWithFeature) SetContextAttributes(contextAttributes ContextRecord) {
	e.contextAttributes = contextAttributes
}

// CollectUsageRecord merges additional data, context attributes, and feature fields.
func (e *BaseWithFeature) CollectUsageRecord() ContextRecord {
	result := e.BaseUsageEvent.CollectUsageRecord()
	for k, v := range e.contextAttributes {
		result[k] = v
	}
	result["feature"] = e.feature.Key
	result["value"] = e.feature.Value
	result["id"] = e.feature.ID
	if e.feature.EnvironmentID != "" {
		result["environmentId"] = e.feature.EnvironmentID
	}
	return result
}

// BaseFeaturesCollection tracks usage of multiple features in one event.
// Its EventName is "feature-collection".
type BaseFeaturesCollection struct {
	BaseUsageEvent
	FeatureValues []*FeatureHubUsageValue
}

type FeaturesCollection interface {
	UsageEvent
	SetFeatureValues(featureValues []*FeatureHubUsageValue)
	GetFeatureValues() []*FeatureHubUsageValue
}

// NewUsageFeaturesCollection constructs an empty BaseFeaturesCollection.
func NewUsageFeaturesCollection(userKey string, additionalData ContextRecord) FeaturesCollection {
	return NewBaseUsageFeaturesCollection(userKey, additionalData)
}

func NewBaseUsageFeaturesCollection(userKey string, additionalData ContextRecord) *BaseFeaturesCollection {
	return &BaseFeaturesCollection{
		BaseUsageEvent: *NewBaseUsageEvent(userKey, additionalData),
		FeatureValues:  make([]*FeatureHubUsageValue, 0),
	}
}

// EventName returns "feature-collection".
func (*BaseFeaturesCollection) EventName() string { return "feature-collection" }
func (c *BaseFeaturesCollection) GetFeatureValues() []*FeatureHubUsageValue {
	return c.FeatureValues
}

func (c *BaseFeaturesCollection) SetFeatureValues(featureValues []*FeatureHubUsageValue) {
	c.FeatureValues = featureValues
}

// CollectUsageRecord merges additional data with feature key→value pairs.
func (c *BaseFeaturesCollection) CollectUsageRecord() ContextRecord {
	result := c.BaseUsageEvent.CollectUsageRecord()
	for _, fv := range c.FeatureValues {
		result[fv.Key] = fv.Value
	}
	return result
}

// BaseCollectionContext extends BaseFeaturesCollection with client context attributes.
// Its EventName is "feature-collection-context".
type BaseCollectionContext struct {
	BaseFeaturesCollection
	ContextAttributes ContextRecord
}

type CollectionContext interface {
	FeaturesCollection
	SetContextAttributes(contextAttributes ContextRecord)
}

// NewUsageFeaturesCollectionContext constructs an empty BaseCollectionContext.
func NewUsageFeaturesCollectionContext(userKey string, additionalData ContextRecord) CollectionContext {
	return NewBaseUsageFeaturesCollectionContext(userKey, additionalData)
}

func NewBaseUsageFeaturesCollectionContext(userKey string, additionalData ContextRecord) *BaseCollectionContext {
	return &BaseCollectionContext{
		BaseFeaturesCollection: *NewBaseUsageFeaturesCollection(userKey, additionalData),
		ContextAttributes:      make(ContextRecord),
	}
}

// EventName returns "feature-collection-context".
func (*BaseCollectionContext) EventName() string { return "feature-collection-context" }
func (c *BaseCollectionContext) SetContextAttributes(contextAttributes ContextRecord) {
	c.ContextAttributes = contextAttributes
}

// CollectUsageRecord merges feature collection data with context attributes.
func (c *BaseCollectionContext) CollectUsageRecord() ContextRecord {
	result := c.BaseFeaturesCollection.CollectUsageRecord()
	for k, v := range c.ContextAttributes {
		result[k] = v
	}
	return result
}

// BaseUsageNamedFeaturesCollection is a BaseCollectionContext with a custom event name.
type BaseUsageNamedFeaturesCollection struct {
	BaseCollectionContext
	name string
}

type UsageNamedFeaturesCollection interface {
	CollectionContext
}

// NewUsageNamedFeaturesCollection constructs a UsageNamedFeaturesCollection.
func NewUsageNamedFeaturesCollection(name, userKey string, additionalData ContextRecord) UsageNamedFeaturesCollection {
	return NewBaseUsageNamedFeaturesCollection(name, userKey, additionalData)
}

func NewBaseUsageNamedFeaturesCollection(name, userKey string, additionalData ContextRecord) *BaseUsageNamedFeaturesCollection {
	return &BaseUsageNamedFeaturesCollection{
		BaseCollectionContext: *NewBaseUsageFeaturesCollectionContext(userKey, additionalData),
		name:                  name,
	}
}

// EventName returns the custom name.
func (c *BaseUsageNamedFeaturesCollection) EventName() string { return c.name }

// Plugin is implemented by usage event consumers.
type Plugin interface {
	DefaultPluginAttributes() ContextRecord
	Send(context context.Context, event UsageEvent) context.Context
}

// Provider is a factory for creating usage events and values.
type Provider struct{}

type ProviderFactory interface {
	NewUsageValue(id, key, environmentID string, value interface{}, valueType models.FeatureValueType) *FeatureHubUsageValue
	NewUsageValueFromFeature(feature *models.FeatureState) *FeatureHubUsageValue
	NewUsageEvent(userKey string, additionalData ContextRecord) UsageEvent
	NewUsageFeature(feature *FeatureHubUsageValue, contextAttributes ContextRecord, userKey string) EventWithFeature
	NewUsageCollectionEvent(userKey string, additionalData ContextRecord) FeaturesCollection
	NewUsageContextCollectionEvent(userKey string, additionalData ContextRecord) CollectionContext
	NewNamedUsageCollection(name string, userKey string, additionalData ContextRecord) UsageNamedFeaturesCollection
}

// DefaultProvider is the package-level default Provider.
var DefaultProvider = &Provider{}

// NewUsageValue creates a FeatureHubUsageValue from raw fields.
func (*Provider) NewUsageValue(id, key, environmentID string, value interface{}, valueType models.FeatureValueType) *FeatureHubUsageValue {
	return NewUsageValue(id, key, environmentID, value, valueType)
}

// NewUsageValueFromFeature creates a FeatureHubUsageValue from a FeatureState.
func (*Provider) NewUsageValueFromFeature(feature *models.FeatureState) *FeatureHubUsageValue {
	return NewUsageValueFromFeature(feature)
}

func (*Provider) NewUsageEvent(userKey string, additionalData ContextRecord) UsageEvent {
	return NewUsageEvent(userKey, additionalData)
}

// NewUsageFeature creates a BaseWithFeature.
func (*Provider) NewUsageFeature(feature *FeatureHubUsageValue, contextAttributes ContextRecord, userKey string) EventWithFeature {
	return NewUsageEventWithFeature(feature, contextAttributes, userKey)
}

// NewUsageCollectionEvent creates an empty BaseFeaturesCollection.
func (*Provider) NewUsageCollectionEvent(userKey string, additionalData ContextRecord) FeaturesCollection {
	return NewUsageFeaturesCollection(userKey, additionalData)
}

// NewUsageContextCollectionEvent creates an empty BaseCollectionContext.
func (*Provider) NewUsageContextCollectionEvent(userKey string, additionalData ContextRecord) CollectionContext {
	return NewUsageFeaturesCollectionContext(userKey, additionalData)
}

// NewNamedUsageCollection creates a UsageNamedFeaturesCollection with the given name.
func (*Provider) NewNamedUsageCollection(name string, userKey string, additionalData ContextRecord) UsageNamedFeaturesCollection {
	return NewUsageNamedFeaturesCollection(name, userKey, additionalData)
}
