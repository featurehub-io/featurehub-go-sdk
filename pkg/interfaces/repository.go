package interfaces

import (
	"context"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
	"github.com/sirupsen/logrus"
)

// RepositoryContext - this is the main interface used for requesting features, and it
// is implemented by the main repository as well as the contexts. A context allows strategies to be
// applied, whereas requesting features directly from the repository does not allow this.
type RepositoryContext interface {
	AddNotifierBoolean(context context.Context, featureKey string, callbackFunc models.CallbackFuncBoolean) (notifierUUID string, err error) // Configure a notifier for a BOOLEAN value:
	AddNotifierJSON(context context.Context, featureKey string, callbackFunc models.CallbackFuncJSON) (notifierUUID string, err error)       // Configure a notifier for a JSON value:
	AddNotifierNumber(context context.Context, featureKey string, callbackFunc models.CallbackFuncNumber) (notifierUUID string, err error)   // Configure a notifier for a NUMBER value:
	AddNotifierString(context context.Context, featureKey string, callbackFunc models.CallbackFuncString) (notifierUUID string, err error)   // Configure a notifier for a STRING value:
	AddNotifierFeature(context context.Context, featureKey string, callbackFunc models.CallbackFuncFeature) (notifierUUID string, err error) // Configure a notifier for a FeatureState:
	DeleteNotifier(featureKey, notifierUUID string) error                                                                                    // Remove a previously configured notifier (by key and UUID, because we support more than one notifier per key)
	GetBoolean(context context.Context, featureKey string) (bool, error)                                                                     // Retrieve a value (by key) for a BOOLEAN feature
	GetNumber(context context.Context, featureKey string) (*float64, error)                                                                  // Retrieve a value (by key) for a NUMBER feature
	GetRawJSON(context context.Context, featureKey string) (*string, error)                                                                  // Retrieve a value (by key) for a JSON feature
	GetString(context context.Context, featureKey string) (*string, error)                                                                   // Retrieve a value (by key) for a STRING feature
	// Number "safe" always returns 0 if it cannot find the value, or it is nil
	Number(context context.Context, featureKey string, defaultValue float64) float64
	// JSON "safe" always returns "{}" if it cannot find the value, or it is nil
	JSON(context context.Context, featureKey string, defaultValue string) string
	// String "safe" always returns "" if it cannot find the value, or it is nil
	String(context context.Context, featureKey string, defaultValue string) string
	Boolean(context context.Context, featureKey string, defaultValue bool) bool

	Properties(context context.Context, featureKey string) map[string]string

	// AllKeys returns all of the keys in the repository at this moment
	AllKeys() []string
}

// FeatureRepository - contexts don't need to implement these and sources of features don't need them either.
// We generally don't want folks getting a hold of raw features as we can't treat the getting of their values
// properly, so we have to do it wrapped. But we do need to be able to get the raw feature the originating repository.
type FeatureRepository interface {
	GetFeature(context context.Context, featureKey string) (feature *models.FeatureState, matched bool, value interface{}, err error)
	// GetInternalString - nil is a valid value for string data types
	GetInternalString(context context.Context, key string, expectedType models.FeatureValueType) (feature *models.FeatureState, matched bool, value *string, err error)
	// GetInternalNumber - nil is a valid value for numeric data types
	GetInternalNumber(context context.Context, key string) (feature *models.FeatureState, matched bool, value *float64, err error)
	// GetInternalBoolean - will always be true or false
	GetInternalBoolean(context context.Context, key string) (feature *models.FeatureState, matched bool, value bool, err error)
	GetFeatures() []*models.FeatureIdentity
	// UsageProvider simply gives us the ability to create Usage structures, and allows the user to overwrite it with their own
	UsageProvider() usage.ProviderFactory
	// This is called when a usage event is actually sent
	EmitUsageEvent(context context.Context, event usage.UsageEvent)
}

type Context interface {
	RepositoryContext
	// Attributes - allows you to get the existing attributes for evaluation
	Attributes() *models.Context
	// WithContext - replaces the existing context with this new one
	WithContext(ctx *models.Context) Context
	RecordUsageEvent(context context.Context, event usage.UsageEvent)
	GetContextUsage(context context.Context) usage.UsageEvent
	RecordNamedUsage(context context.Context, name string, additionalParams usage.ContextRecord)
	AsConvertibleString(context context.Context, key string) (string, error)
}

// FeatureHubContext mirrors Context but omits the context.Context parameter from every method,
// providing a simpler API for callers that do not need context propagation.
type FeatureHubContext interface {
	AddNotifierBoolean(featureKey string, callbackFunc models.CallbackFuncBoolean) (notifierUUID string, err error)
	AddNotifierJSON(featureKey string, callbackFunc models.CallbackFuncJSON) (notifierUUID string, err error)
	AddNotifierNumber(featureKey string, callbackFunc models.CallbackFuncNumber) (notifierUUID string, err error)
	AddNotifierString(featureKey string, callbackFunc models.CallbackFuncString) (notifierUUID string, err error)
	AddNotifierFeature(featureKey string, callbackFunc models.CallbackFuncFeature) (notifierUUID string, err error)
	DeleteNotifier(featureKey, notifierUUID string) error
	GetBoolean(featureKey string) (bool, error)
	GetNumber(featureKey string) (*float64, error)
	GetRawJSON(featureKey string) (*string, error)
	GetString(featureKey string) (*string, error)
	Number(featureKey string, defaultValue float64) float64
	JSON(featureKey string, defaultValue string) string
	String(featureKey string, defaultValue string) string
	Boolean(featureKey string, defaultValue bool) bool
	Properties(featureKey string) map[string]string
	AllKeys() []string
	Attributes() *models.Context
	WithContext(ctx *models.Context) FeatureHubContext
	RecordUsageEvent(event usage.UsageEvent)
	GetContextUsage() usage.UsageEvent
	RecordNamedUsage(name string, additionalParams usage.ContextRecord)
	AsConvertibleString(key string) (string, error)
}

// FeatureHubConfig is the interface for the core SDK configuration and lifecycle. It covers
// connection management, feature context creation, and configuration options, allowing
// *core.Config to be substituted with a test double or alternative implementation.
type FeatureHubConfig interface {
	// Connection lifecycle
	Connect() (FeatureHubConfig, error)
	Build(ctx *models.Context) (FeatureHubConfig, error)
	Close()

	// Feature context creation
	NewContext() Context
	WithContext(ctx *models.Context) Context

	// Readiness
	IsReady() bool
	ReadinessListener(context context.Context, callbackFunc func(context context.Context))

	// Validation and metadata
	Validate() error
	EnvironmentID() string
	ClientEvaluated() bool
	FeaturesURL() string
	PollingFeaturesURL() string
	Timeout() time.Duration
	EdgeType() models.EdgeType

	// Configuration (fluent)
	WithSDKKey(key string) FeatureHubConfig
	WithLogLevel(logLevel logrus.Level) FeatureHubConfig
	WithWaitForData(value time.Duration) FeatureHubConfig
	WithFatalErrorHandler(fatalErrorFunc ErrorFunc) FeatureHubConfig
	RegisterUsagePlugin(plugin usage.Plugin) FeatureHubConfig
	AddValueInterceptor(valueInterceptor FeatureValueInterceptor)
}

type InternalRepository interface {
	ProcessFeature(feature *models.FeatureState)
	ProcessFeatures(features []*models.FeatureState)
	ProcessDeleteFeature(feature *models.FeatureState)
	AddValueInterceptor(valueInterceptor FeatureValueInterceptor)
	WithContext(context *models.Context) Context
	IsReady() bool                                                                         // Is the repository ready, does it have its initial state?
	ReadinessListener(context context.Context, callbackFunc func(context context.Context)) // Configure the SDK with a function to call when we're ready (up and running with some data)
}
