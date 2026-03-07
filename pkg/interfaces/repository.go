package interfaces

import (
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
)

type RepositoryContext interface {
	AddNotifierBoolean(featureKey string, callbackFunc models.CallbackFuncBoolean) (notifierUUID string, err error) // Configure a notifier for a BOOLEAN value:
	AddNotifierJSON(featureKey string, callbackFunc models.CallbackFuncJSON) (notifierUUID string, err error)       // Configure a notifier for a JSON value:
	AddNotifierNumber(featureKey string, callbackFunc models.CallbackFuncNumber) (notifierUUID string, err error)   // Configure a notifier for a NUMBER value:
	AddNotifierString(featureKey string, callbackFunc models.CallbackFuncString) (notifierUUID string, err error)   // Configure a notifier for a STRING value:
	AddNotifierFeature(featureKey string, callbackFunc models.CallbackFuncFeature) (notifierUUID string, err error) // Configure a notifier for a FeatureState:
	DeleteNotifier(featureKey, notifierUUID string) error                                                           // Remove a previously configured notifier (by key and UUID, because we support more than one notifier per key)
	GetBoolean(featureKey string) (bool, error)                                                                     // Retrieve a value (by key) for a BOOLEAN feature
	GetNumber(featureKey string) (float64, error)                                                                   // Retrieve a value (by key) for a NUMBER feature
	GetRawJSON(featureKey string) (string, error)                                                                   // Retrieve a value (by key) for a JSON feature
	GetString(featureKey string) (string, error)                                                                    // Retrieve a value (by key) for a STRING feature
}

// FeatureRepository - contexts don't need to implement these and sources of features don't need them either.
// We generally don't want folks getting a hold of raw features as we can't treat the getting of their values
// properly, so we have to do it wrapped. But we do need to be able to get the raw feature the originating repository.
type FeatureRepository interface {
	GetFeature(featureKey string, recordUsage bool) (feature *models.FeatureState, matched bool, value interface{}, err error)
	GetInternalString(key string, recordUsage bool, expectedType models.FeatureValueType) (feature *models.FeatureState, matched bool, value string, err error)
	GetInternalNumber(key string, recordUsage bool) (feature *models.FeatureState, matched bool, value float64, err error)
	GetInternalBoolean(key string, recordUsage bool) (feature *models.FeatureState, matched bool, value bool, err error)
}

type Context interface {
	RepositoryContext
	Attributes() *models.Context
	Repository() Repository
	WithContext(ctx *models.Context) Context
}

// Repository for FeatureHub:
type Repository interface {
	RepositoryContext
	ReadinessListener(callbackFunc func()) // Configure the SDK with a function to call when we're ready (up and running with some data)
	IsReady() bool                         // Is the repository ready, does it have its initial state?
	WithContext(context *models.Context) Context
	AddValueInterceptor(valueInterceptor FeatureValueInterceptor)
}

type InternalRepository interface {
	ProcessFeature(feature *models.FeatureState)
	ProcessFeatures(features []*models.FeatureState)
	ProcessDeleteFeature(feature *models.FeatureState)
	IsReady() bool // Is the repository ready, does it have its initial state?
}
