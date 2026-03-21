package interfaces

import (
	"context"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
)

type ErrorFunc func(error, string, map[string]interface{})

// FeatureValueInterceptor pairs an intercept function with a Close function for
// lifecycle management. Intercept is called on every feature read; Close releases
// any resources held by the interceptor (e.g. file-watch goroutines).
//
// If a feature value override is provided and matched, regardless of strategies and context,
// its value will be used.
type FeatureValueInterceptor struct {
	// Intercept is called to check for a value override. Return (value, true) to
	// supply an override, or (nil, false) to pass through. The feature argument
	// may be nil when the key is unknown. Value is returned first, matched bool second.
	Intercept func(ctx context.Context, key string, repo FeatureRepository, feature *models.FeatureState) (value interface{}, matched bool)
	// Close releases resources held by this interceptor. Safe to call multiple times.
	Close func()
}

// NewInterceptor constructs a FeatureValueInterceptor with a no-op Close,
// for interceptors that hold no external resources.
func NewInterceptor(fn func(ctx context.Context, key string, repo FeatureRepository, feature *models.FeatureState) (value interface{}, matched bool)) FeatureValueInterceptor {
	return FeatureValueInterceptor{Intercept: fn, Close: func() {}}
}
