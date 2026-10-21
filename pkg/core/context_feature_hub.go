package core

import (
	"context"
	"fmt"
	"net/http"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
)

// contextKey is the type of the key the FeatureHub context is stored under in a Go context. An unexported type
// means no other package's key can collide with it:
type contextKey struct{}

var featureHubContextKey = contextKey{}

// ContextFeatureHub implements interfaces.FeatureHubContext by delegating to an
// interfaces.Context retrieved from a Go context.Context, using that same Go context
// as the context parameter for every delegated call.
type ContextFeatureHub struct {
	ctx   context.Context
	fhCtx interfaces.Context
}

// RequestInitialiser fills in a request's FeatureHub context from the request (e.g. the user key from the
// authenticated user), before the handler reads any features:
type RequestInitialiser func(r *http.Request, fhCtx interfaces.Context)

// ContextMiddleware makes a FeatureHub context for each request with NewRequestContext, runs the initialisers on
// it, and passes the request on with the context stored in it (retrieve it with NewFromContext or FromContext).
// If the config has been closed, the request is passed on unchanged.
func ContextMiddleware(fhConfig interfaces.FeatureHubConfig, initialisers ...RequestInitialiser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, fhCtx := NewRequestContext(r.Context(), fhConfig)
			if fhCtx == nil {
				next.ServeHTTP(w, r)
				return
			}
			r = r.WithContext(ctx)
			for _, initialise := range initialisers {
				initialise(r, fhCtx)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// NewRequestContext starts a request outside of HTTP (e.g. gRPC, or a message from a queue): it makes a new
// FeatureHub context, stores it in ctx, and lets request-scoped usage plugins add their state (see
// usage.RequestScopedPlugin). It returns the new Go context, to be used for the rest of the request, and the
// FeatureHub context so its attributes can be set. If the config has been closed, it returns ctx and nil.
func NewRequestContext(ctx context.Context, fhConfig interfaces.FeatureHubConfig) (context.Context, interfaces.Context) {
	fhCtx := fhConfig.NewContext()
	if fhCtx == nil {
		return ctx, nil
	}
	return fhConfig.StartRequest(StoreInContext(ctx, fhCtx)), fhCtx
}

// StoreInContext stores the context given in the Golang context and passes that context back again.
func StoreInContext(ctx context.Context, fhCtx interfaces.Context) context.Context {
	return context.WithValue(ctx, featureHubContextKey, fhCtx)
}

// FromContext returns the FeatureHub context stored in ctx (by ContextMiddleware, NewRequestContext or
// StoreInContext), if there is one:
func FromContext(ctx context.Context) (interfaces.Context, bool) {
	fhCtx, ok := ctx.Value(featureHubContextKey).(interfaces.Context)
	return fhCtx, ok && fhCtx != nil
}

// NewFromContext retrieves the interfaces.Context stored in ctx, wraps it in a ContextFeatureHub, and returns it.
// Returns an error if there is none.
func NewFromContext(ctx context.Context) (*ContextFeatureHub, error) {
	fhCtx, ok := FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no featurehub context found in context")
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
