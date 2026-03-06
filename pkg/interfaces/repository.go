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
	GetFeature(featureKey string) (*models.FeatureState, error)                                                     // Retrieve a feature (by key) (value is an interface{})
	GetNumber(featureKey string) (float64, error)                                                                   // Retrieve a value (by key) for a NUMBER feature
	GetRawJSON(featureKey string) (string, error)                                                                   // Retrieve a value (by key) for a JSON feature
	GetString(featureKey string) (string, error)                                                                    // Retrieve a value (by key) for a STRING feature
}

// Repository for FeatureHub:
type Repository interface {
	RepositoryContext
	ReadinessListener(callbackFunc func()) // Configure the SDK with a function to call when we're ready (up and running with some data)
	IsReady() bool                         // Is the repository ready, does it have its initial state?
	WithContext(context *models.Context) Context
}

type InternalRepository interface {
	ProcessFeature(feature *models.FeatureState)
	ProcessFeatures(features []*models.FeatureState)
	ProcessDeleteFeature(feature *models.FeatureState)
	IsReady() bool // Is the repository ready, does it have its initial state?
}
