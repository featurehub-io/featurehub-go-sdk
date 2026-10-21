package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// evaluations is the per-request state of evaluationPlugin, as an OpenTelemetry baggage plugin would keep it:
type evaluations struct {
	mu     sync.Mutex
	values map[string]string
}

type evaluationsKey struct{}

// evaluationPlugin records each feature evaluated during a request in the request's context:
type evaluationPlugin struct{}

func (evaluationPlugin) DefaultPluginAttributes() usage.ContextRecord { return nil }
func (evaluationPlugin) CanSendAsync() bool                           { return false }
func (evaluationPlugin) Close()                                       {}
func (evaluationPlugin) StartRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, evaluationsKey{}, &evaluations{values: map[string]string{}})
}
func (evaluationPlugin) Send(ctx context.Context, event usage.UsageEvent) context.Context {
	withFeature, ok := event.(usage.EventWithFeature)
	if !ok {
		return ctx
	}
	if evals, ok := ctx.Value(evaluationsKey{}).(*evaluations); ok {
		evals.mu.Lock()
		evals.values[withFeature.GetFeature().Key] = withFeature.GetFeature().Value
		evals.mu.Unlock()
	}
	return ctx
}

func newConfigWithFeatures(t *testing.T) *Config {
	t.Helper()
	config := NewConfig("http://localhost", "default/env/key", nil)
	repository, _ := config.repo()
	repository.ProcessFeatures([]*models.FeatureState{
		ffs(`{"key":"flag","type":"BOOLEAN","value":true,"id":"id-flag","version":1}`),
		ffs(`{"key":"colour","type":"STRING","value":"blue","id":"id-colour","version":1}`),
	})
	return config
}

func TestMiddlewareRecordsEvaluationsForTheRequest(t *testing.T) {
	config := newConfigWithFeatures(t)
	config.RegisterUsagePlugin(evaluationPlugin{})

	var evals *evaluations
	var userKey string
	handler := ContextMiddleware(config, func(r *http.Request, fhCtx interfaces.Context) {
		fhCtx.Attributes().Userkey = r.Header.Get("X-User")
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub, err := NewFromContext(r.Context())
		require.NoError(t, err)
		hub.Boolean("flag", false)
		hub.String("colour", "")

		// a goroutine of the request reads a feature too
		var wg sync.WaitGroup
		wg.Add(1)
		go func() { defer wg.Done(); hub.Boolean("flag", false) }()
		wg.Wait()

		evals = r.Context().Value(evaluationsKey{}).(*evaluations)
		fhCtx, _ := FromContext(r.Context())
		userKey = fhCtx.Attributes().Userkey
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User", "alice")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.NotNil(t, evals)
	// the plugin is synchronous, so every evaluation is recorded by the time the read returns
	assert.Equal(t, map[string]string{"flag": "on", "colour": "blue"}, evals.values)
	assert.Equal(t, "alice", userKey, "the initialiser fills in the context before the handler runs")
}

func TestEachRequestGetsItsOwnState(t *testing.T) {
	config := newConfigWithFeatures(t)
	config.RegisterUsagePlugin(evaluationPlugin{})

	first, fhFirst := NewRequestContext(context.Background(), config)
	second, _ := NewRequestContext(context.Background(), config)
	fhFirst.Boolean(first, "flag", false)

	assert.Len(t, first.Value(evaluationsKey{}).(*evaluations).values, 1)
	assert.Empty(t, second.Value(evaluationsKey{}).(*evaluations).values)
}

func TestNewRequestContextOnClosedConfig(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	config.Close()

	ctx := context.Background()
	got, fhCtx := NewRequestContext(ctx, config)
	assert.Nil(t, fhCtx)
	assert.Equal(t, ctx, got)
	assert.Equal(t, ctx, config.StartRequest(ctx))

	called := false
	ContextMiddleware(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, ok := FromContext(r.Context())
		assert.False(t, ok)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	assert.True(t, called, "the request is still handled")
}

func TestFeatureHubContextKeyDoesNotCollideWithStrings(t *testing.T) {
	config := NewConfig("http://localhost", "default/env/key", nil)
	ctx := context.WithValue(context.Background(), "featurehub", "something else") //nolint:staticcheck // the collision being tested

	_, ok := FromContext(ctx)
	assert.False(t, ok)

	ctx = StoreInContext(ctx, config.NewContext())
	_, ok = FromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, "something else", ctx.Value("featurehub"))
}
