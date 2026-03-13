package core

import (
	"context"
	"fmt"
	"net/http"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
)

const featureHubContextKey = "featurehub"

// ContextFeatureHub implements interfaces.FeatureHubContext by delegating to an
// interfaces.Context retrieved from a Go context.Context, using that same Go context
// as the context parameter for every delegated call.
type ContextFeatureHub struct {
	ctx   context.Context
	fhCtx interfaces.Context
}

func ContextMiddleware(fhConfig interfaces.FeatureHubConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(StoreInContext(r.Context(), fhConfig.NewContext())))
		})
	}
}

// StoreInContext stores the context given in the Golang context and passes that context back again.
func StoreInContext(ctx context.Context, fhCtx interfaces.Context) context.Context {
	return context.WithValue(ctx, featureHubContextKey, fhCtx)
}

// NewFromContext retrieves an interfaces.Context stored under the "featurehub" key in ctx,
// wraps it in a ContextFeatureHub, and returns it. Returns an error if the key is absent
// or the value is not an interfaces.Context.
func NewFromContext(ctx context.Context) (*ContextFeatureHub, error) {
	val := ctx.Value(featureHubContextKey)
	if val == nil {
		return nil, fmt.Errorf("no featurehub context found in context")
	}
	fhCtx, ok := val.(interfaces.Context)
	if !ok {
		return nil, fmt.Errorf("featurehub context value is not an interfaces.Context")
	}
	return &ContextFeatureHub{ctx: ctx, fhCtx: fhCtx}, nil
}

func (c *ContextFeatureHub) AddNotifierBoolean(featureKey string, callbackFunc models.CallbackFuncBoolean) (string, error) {
	return c.fhCtx.AddNotifierBoolean(c.ctx, featureKey, callbackFunc)
}

func (c *ContextFeatureHub) AddNotifierJSON(featureKey string, callbackFunc models.CallbackFuncJSON) (string, error) {
	return c.fhCtx.AddNotifierJSON(c.ctx, featureKey, callbackFunc)
}

func (c *ContextFeatureHub) AddNotifierNumber(featureKey string, callbackFunc models.CallbackFuncNumber) (string, error) {
	return c.fhCtx.AddNotifierNumber(c.ctx, featureKey, callbackFunc)
}

func (c *ContextFeatureHub) AddNotifierString(featureKey string, callbackFunc models.CallbackFuncString) (string, error) {
	return c.fhCtx.AddNotifierString(c.ctx, featureKey, callbackFunc)
}

func (c *ContextFeatureHub) AddNotifierFeature(featureKey string, callbackFunc models.CallbackFuncFeature) (string, error) {
	return c.fhCtx.AddNotifierFeature(c.ctx, featureKey, callbackFunc)
}

func (c *ContextFeatureHub) DeleteNotifier(featureKey, notifierUUID string) error {
	return c.fhCtx.DeleteNotifier(featureKey, notifierUUID)
}

func (c *ContextFeatureHub) GetBoolean(featureKey string) (bool, error) {
	return c.fhCtx.GetBoolean(c.ctx, featureKey)
}

func (c *ContextFeatureHub) GetNumber(featureKey string) (*float64, error) {
	return c.fhCtx.GetNumber(c.ctx, featureKey)
}

func (c *ContextFeatureHub) GetRawJSON(featureKey string) (*string, error) {
	return c.fhCtx.GetRawJSON(c.ctx, featureKey)
}

func (c *ContextFeatureHub) GetString(featureKey string) (*string, error) {
	return c.fhCtx.GetString(c.ctx, featureKey)
}

func (c *ContextFeatureHub) Number(featureKey string, defaultValue float64) float64 {
	return c.fhCtx.Number(c.ctx, featureKey, defaultValue)
}

func (c *ContextFeatureHub) JSON(featureKey string, defaultValue string) string {
	return c.fhCtx.JSON(c.ctx, featureKey, defaultValue)
}

func (c *ContextFeatureHub) String(featureKey string, defaultValue string) string {
	return c.fhCtx.String(c.ctx, featureKey, defaultValue)
}

func (c *ContextFeatureHub) Boolean(featureKey string, defaultValue bool) bool {
	return c.fhCtx.Boolean(c.ctx, featureKey, defaultValue)
}

func (c *ContextFeatureHub) Properties(featureKey string) map[string]string {
	return c.fhCtx.Properties(c.ctx, featureKey)
}

func (c *ContextFeatureHub) AllKeys() []string {
	return c.fhCtx.AllKeys()
}

func (c *ContextFeatureHub) Attributes() *models.Context {
	return c.fhCtx.Attributes()
}

func (c *ContextFeatureHub) WithContext(ctx *models.Context) interfaces.FeatureHubContext {
	return &ContextFeatureHub{ctx: c.ctx, fhCtx: c.fhCtx.WithContext(ctx)}
}

func (c *ContextFeatureHub) RecordUsageEvent(event usage.UsageEvent) {
	c.fhCtx.RecordUsageEvent(c.ctx, event)
}

func (c *ContextFeatureHub) GetContextUsage() usage.UsageEvent {
	return c.fhCtx.GetContextUsage(c.ctx)
}

func (c *ContextFeatureHub) RecordNamedUsage(name string, additionalParams usage.ContextRecord) {
	c.fhCtx.RecordNamedUsage(c.ctx, name, additionalParams)
}

func (c *ContextFeatureHub) AsConvertibleString(key string) (string, error) {
	return c.fhCtx.AsConvertibleString(c.ctx, key)
}
