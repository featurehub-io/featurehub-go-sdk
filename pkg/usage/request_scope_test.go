package usage

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type requestKey struct{}

// scopedPlugin adds a value to each request's context:
type scopedPlugin struct {
	mockPlugin
	async bool
	start func(ctx context.Context) context.Context
}

func (p *scopedPlugin) CanSendAsync() bool { return p.async }
func (p *scopedPlugin) StartRequest(ctx context.Context) context.Context {
	return p.start(ctx)
}

func TestAdapterStartRequestCallsRequestScopedPlugins(t *testing.T) {
	adapter := NewAdapter(newMockRepo(), newTestLogger())
	adapter.RegisterPlugin(newMockPlugin()) // not request scoped, so not called
	adapter.RegisterPlugin(&scopedPlugin{start: func(ctx context.Context) context.Context {
		return context.WithValue(ctx, requestKey{}, "first")
	}})
	adapter.RegisterPlugin(&scopedPlugin{start: func(ctx context.Context) context.Context {
		return context.WithValue(ctx, requestKey{}, ctx.Value(requestKey{}).(string)+"+second")
	}})

	ctx := adapter.StartRequest(context.Background())

	assert.Equal(t, "first+second", ctx.Value(requestKey{}), "each plugin gets the context from the one before")
}

func TestAdapterStartRequestSurvivesPanicsAndNil(t *testing.T) {
	adapter := NewAdapter(newMockRepo(), newTestLogger())
	adapter.RegisterPlugin(&scopedPlugin{start: func(ctx context.Context) context.Context { panic("boom") }})
	adapter.RegisterPlugin(&scopedPlugin{start: func(ctx context.Context) context.Context { return nil }})
	adapter.RegisterPlugin(&scopedPlugin{start: func(ctx context.Context) context.Context {
		return context.WithValue(ctx, requestKey{}, "kept")
	}})

	ctx := adapter.StartRequest(context.Background())

	assert.NotNil(t, ctx)
	assert.Equal(t, "kept", ctx.Value(requestKey{}))
}

func TestAdapterWarnsAboutAsyncRequestScopedPlugin(t *testing.T) {
	logger := newTestLogger()
	logs := new(bytes.Buffer)
	logger.SetOutput(logs)
	adapter := NewAdapter(newMockRepo(), logger)

	adapter.RegisterPlugin(&scopedPlugin{async: true, start: func(ctx context.Context) context.Context { return ctx }})
	assert.Contains(t, logs.String(), "request-scoped usage plugin sends asynchronously")

	logs.Reset()
	adapter.RegisterPlugin(&scopedPlugin{start: func(ctx context.Context) context.Context { return ctx }})
	assert.Empty(t, logs.String())
}

// ctxPlugin is a synchronous plugin that replaces the context, as the next plugin will see it:
type ctxPlugin struct{ mockPlugin }

func (p *ctxPlugin) CanSendAsync() bool { return false }
func (p *ctxPlugin) Send(ctx context.Context, event UsageEvent) context.Context {
	return context.WithValue(ctx, requestKey{}, "replaced")
}

func TestAsyncPluginBeforeSyncPluginDoesNotRace(t *testing.T) {
	repo := newMockRepo()
	adapter := NewAdapter(repo, newTestLogger())
	seen := make(chan interface{}, 20)
	adapter.RegisterPlugin(&ctxReadingPlugin{seen: seen})
	adapter.RegisterPlugin(&ctxPlugin{})

	for i := 0; i < 20; i++ {
		repo.emit(NewUsageEventWithFeature(&FeatureHubUsageValue{ID: "1", Key: "f", Value: "on"}, nil, ""))
	}
	for i := 0; i < 20; i++ {
		select {
		case value := <-seen:
			assert.Nil(t, value, "the async plugin sees the context as it was before the sync plugin replaced it")
		case <-time.After(time.Second):
			t.Fatal("the async plugin was not called")
		}
	}
}

// ctxReadingPlugin is an asynchronous plugin that reads the context it is sent, so -race catches it being shared
// with the dispatch loop:
type ctxReadingPlugin struct {
	mockPlugin
	seen chan interface{}
}

func (p *ctxReadingPlugin) CanSendAsync() bool { return true }
func (p *ctxReadingPlugin) Send(ctx context.Context, event UsageEvent) context.Context {
	p.seen <- ctx.Value(requestKey{})
	return ctx
}

func TestNilLoggerDoesNotTurnPluginPanicsIntoCrashes(t *testing.T) {
	adapter := NewAdapter(newMockRepo(), nil)
	adapter.RegisterPlugin(&scopedPlugin{start: func(ctx context.Context) context.Context { panic("boom") }})

	assert.NotPanics(t, func() { adapter.StartRequest(context.Background()) })
}
