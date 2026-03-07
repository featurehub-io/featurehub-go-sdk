package interfaces

import "github.com/featurehub-io/featurehub-go-sdk/pkg/models"

type ErrorFunc func(error, string, map[string]interface{})

// FeatureValueInterceptor instances of this function are designed to allow
// the overriding of feature values by collecting them from elsewhere. If an
// existing feature is found with the key, it will be passed, the interceptor should
// pass back if it found the override and the value it discovered if so. It can
// convert the value into any supported type, bool, number, string or nil.
//
// If a feature value override is provided and matched, regardless of strategies and context,
// its value will be used.
type FeatureValueInterceptor func(key string, feature *models.FeatureState) (matched bool, value interface{})
