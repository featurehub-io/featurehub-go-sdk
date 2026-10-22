package usage

import (
	"bytes"
	"context"
	"sync"
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

// nilPlugin is a synchronous plugin whose Send returns nil:
type nilPlugin struct{ mockPlugin }

func (p *nilPlugin) CanSendAsync() bool                               { return false }
func (p *nilPlugin) Send(context.Context, UsageEvent) context.Context { return nil }

// valuePlugin is a synchronous plugin that records the requestKey value of the context it is sent:
type valuePlugin struct {
	mockPlugin
	seen []interface{}
}

func (p *valuePlugin) CanSendAsync() bool { return false }
func (p *valuePlugin) Send(ctx context.Context, event UsageEvent) context.Context {
	p.seen = append(p.seen, ctx.Value(requestKey{}))
	return ctx
}

func TestSyncPluginReturningNilKeepsTheContext(t *testing.T) {
	adapter := NewAdapter(newMockRepo(), newTestLogger())
	adapter.RegisterPlugin(&nilPlugin{})
	next := &valuePlugin{}
	adapter.RegisterPlugin(next)

	ctx := context.WithValue(context.Background(), requestKey{}, "kept")
	result := adapter.dispatch(ctx, NewUsageEventWithFeature(&FeatureHubUsageValue{ID: "1", Key: "f", Value: "on"}, nil, ""))

	assert.Equal(t, []interface{}{"kept"}, next.seen, "the next plugin gets the context from before the nil")
	assert.Equal(t, "kept", result.Value(requestKey{}))
}

// closingPlugin records whether it was closed:
type closingPlugin struct {
	mockPlugin
	closed bool
}

func (p *closingPlugin) Close() { p.closed = true }

func TestRegisterPluginOnClosedAdapterClosesThePlugin(t *testing.T) {
	adapter := NewAdapter(newMockRepo(), newTestLogger())
	adapter.Close()
	adapter.Close() // idempotent

	plugin := &closingPlugin{}
	assert.False(t, adapter.RegisterPlugin(plugin))
	assert.True(t, plugin.closed, "a plugin registered after Close is closed, not leaked")
	assert.Empty(t, adapter.currentPlugins().all)
}

func TestNonScopedRegistrationSharesTheScopedList(t *testing.T) {
	adapter := NewAdapter(newMockRepo(), newTestLogger())
	adapter.RegisterPlugin(&scopedPlugin{start: func(ctx context.Context) context.Context { return ctx }})
	before := adapter.currentPlugins().scoped
	adapter.RegisterPlugin(newMockPlugin())

	after := adapter.currentPlugins()
	assert.Len(t, after.all, 2)
	assert.Len(t, after.scoped, 1)
	assert.Same(t, &before[0], &after.scoped[0])
}

// blockingPlugin is an asynchronous plugin whose Send waits to be released, and which records the order of
// Send and Close:
type blockingPlugin struct {
	mockPlugin
	started chan struct{}
	release chan struct{}
	mu      sync.Mutex
	order   []string
}

func (p *blockingPlugin) CanSendAsync() bool { return true }
func (p *blockingPlugin) Send(ctx context.Context, event UsageEvent) context.Context {
	close(p.started)
	<-p.release
	p.record("send")
	return ctx
}
func (p *blockingPlugin) Close() { p.record("close") }
func (p *blockingPlugin) record(what string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.order = append(p.order, what)
}

func TestAdapterCloseWaitsForSendsInProgress(t *testing.T) {
	repo := newMockRepo()
	adapter := NewAdapter(repo, newTestLogger())
	plugin := &blockingPlugin{started: make(chan struct{}), release: make(chan struct{})}
	adapter.RegisterPlugin(plugin)

	repo.emit(NewUsageEventWithFeature(&FeatureHubUsageValue{ID: "1", Key: "f", Value: "on"}, nil, ""))
	<-plugin.started

	closed := make(chan struct{})
	go func() {
		adapter.Close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("Close returned while a Send was in progress")
	case <-time.After(50 * time.Millisecond):
	}

	close(plugin.release)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close did not return once the Send finished")
	}
	assert.Equal(t, []string{"send", "close"}, plugin.order, "the plugin is closed only after its Send has finished")
}

func TestClosedAdapterSendsNothing(t *testing.T) {
	adapter := NewAdapter(newMockRepo(), newTestLogger())
	plugin := &valuePlugin{}
	adapter.RegisterPlugin(plugin)
	adapter.Close()

	// an emit that had already copied the handler before Close removed it
	adapter.dispatch(context.Background(), NewUsageEventWithFeature(&FeatureHubUsageValue{ID: "1", Key: "f", Value: "on"}, nil, ""))
	adapter.StartRequest(context.Background())

	assert.Empty(t, plugin.seen)
}
