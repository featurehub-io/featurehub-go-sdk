package usage

import (
	"context"
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// --- defaultConvert ---

func TestDefaultConvertBooleanTrue(t *testing.T) {
	assert.Equal(t, "on", defaultConvert(true, models.TypeBoolean))
}

func TestDefaultConvertBooleanFalse(t *testing.T) {
	assert.Equal(t, "off", defaultConvert(false, models.TypeBoolean))
}

func TestDefaultConvertString(t *testing.T) {
	assert.Equal(t, "hello", defaultConvert("hello", models.TypeString))
}

func TestDefaultConvertNumber(t *testing.T) {
	assert.Equal(t, "42", defaultConvert(float64(42), models.TypeNumber))
	assert.Equal(t, "3.14", defaultConvert(3.14, models.TypeNumber))
}

func TestDefaultConvertJSONReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", defaultConvert(`{"a":1}`, models.TypeJSON))
}

func TestDefaultConvertNilReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", defaultConvert(nil, models.TypeBoolean))
}

// --- SetConvertFunc ---

func TestSetConvertFuncCustom(t *testing.T) {
	t.Cleanup(func() { SetConvertFunc(nil) }) // restore default

	SetConvertFunc(func(_ interface{}, _ models.FeatureValueType) string {
		return "custom"
	})

	assert.Equal(t, "custom", convert("anything", models.TypeString))
}

func TestSetConvertFuncNilResetsToDefault(t *testing.T) {
	SetConvertFunc(func(_ interface{}, _ models.FeatureValueType) string { return "x" })
	assert.Equal(t, "x", convert(true, models.TypeBoolean))
	SetConvertFunc(nil)

	assert.Equal(t, "on", convert(true, models.TypeBoolean))
}

// --- NewUsageValue ---

func TestNewUsageValue(t *testing.T) {
	v := NewUsageValue("id-1", "my-flag", "env-1", true, models.TypeBoolean)
	assert.Equal(t, "id-1", v.ID)
	assert.Equal(t, "my-flag", v.Key)
	assert.Equal(t, "env-1", v.EnvironmentID)
	assert.Equal(t, "on", v.Value)
}

func TestNewUsageValueFromFeature(t *testing.T) {
	fs := &models.FeatureState{ID: "fs-1", Key: "flag", EnvironmentID: "env-abc", Value: "hello", Type: models.TypeString}
	v := NewUsageValueFromFeature(fs)
	assert.Equal(t, "fs-1", v.ID)
	assert.Equal(t, "flag", v.Key)
	assert.Equal(t, "env-abc", v.EnvironmentID)
	assert.Equal(t, "hello", v.Value)
}

// --- BaseUsageEvent ---

func TestBaseUsageEventUserKey(t *testing.T) {
	b := NewBaseUsageEvent("kwong", nil)
	assert.Equal(t, "kwong", b.UserKey())
}

func TestBaseUsageEventCollectUsageRecord(t *testing.T) {
	b := NewBaseUsageEvent("bob", ContextRecord{"x": "1"})
	rec := b.CollectUsageRecord()
	assert.Equal(t, "1", rec["x"])
}

func TestBaseUsageEventCollectUsageRecordIsCopy(t *testing.T) {
	b := NewBaseUsageEvent("", ContextRecord{"k": "v"})
	rec := b.CollectUsageRecord()
	rec["k"] = "mutated"

	// Original should be unchanged.
	original := b.CollectUsageRecord()
	assert.Equal(t, "v", original["k"])
}

func TestBaseUsageEventSetAdditionalData(t *testing.T) {
	b := NewBaseUsageEvent("", nil)
	b.SetAdditionalData(ContextRecord{"a": "b"})
	assert.Equal(t, "", b.UserKey())
	rec := b.CollectUsageRecord()
	assert.Equal(t, "b", rec["a"])
}

// --- BaseWithFeature ---

func TestUsageEventWithFeatureEventName(t *testing.T) {
	fv := &FeatureHubUsageValue{ID: "1", Key: "flag", Value: "on"}
	e := NewUsageEventWithFeature(fv, nil, "user-1")
	assert.Equal(t, "feature", e.EventName())
}

func TestUsageEventWithFeatureUserKey(t *testing.T) {
	fv := &FeatureHubUsageValue{ID: "1", Key: "flag", Value: "on"}
	e := NewUsageEventWithFeature(fv, nil, "user-1")
	assert.Equal(t, "user-1", e.UserKey())
}

func TestUsageEventWithFeatureCollectUsageRecord(t *testing.T) {
	fv := &FeatureHubUsageValue{ID: "id-abc", Key: "my-flag", EnvironmentID: "env-xyz", Value: "on"}
	ctx := ContextRecord{"country": "Thailand"}
	e := NewUsageEventWithFeature(fv, ctx, "")

	rec := e.CollectUsageRecord()
	assert.Equal(t, "my-flag", rec["feature"])
	assert.Equal(t, "on", rec["value"])
	assert.Equal(t, "id-abc", rec["id"])
	assert.Equal(t, "env-xyz", rec["environmentId"])
	assert.Equal(t, "Thailand", rec["country"])
}

func TestUsageEventWithFeatureCollectUsageRecordOmitsEmptyEnvironmentID(t *testing.T) {
	fv := &FeatureHubUsageValue{ID: "id-abc", Key: "my-flag", Value: "on"}
	e := NewUsageEventWithFeature(fv, nil, "")

	rec := e.CollectUsageRecord()
	_, hasEnvID := rec["environmentId"]
	assert.False(t, hasEnvID, "environmentId should be absent when empty")
}

func TestUsageEventWithFeatureCollectUsageRecordNilContext(t *testing.T) {
	fv := &FeatureHubUsageValue{ID: "1", Key: "f", Value: "off"}
	e := NewUsageEventWithFeature(fv, nil, "")

	rec := e.CollectUsageRecord()
	assert.Equal(t, "f", rec["feature"])
}

func TestUsageEventWithFeatureMergeOrder(t *testing.T) {
	// feature fields should win over context attributes if keys overlap.
	fv := &FeatureHubUsageValue{ID: "1", Key: "my-flag", Value: "on"}
	ctx := ContextRecord{"feature": "should-be-overwritten"}
	e := NewUsageEventWithFeature(fv, ctx, "")

	rec := e.CollectUsageRecord()
	assert.Equal(t, "my-flag", rec["feature"])
}

// --- BaseFeaturesCollection ---

func TestUsageFeaturesCollectionEventName(t *testing.T) {
	c := NewUsageFeaturesCollection("", nil)
	assert.Equal(t, "feature-collection", c.EventName())
}

func TestUsageFeaturesCollectionCollectUsageRecord(t *testing.T) {
	c := NewUsageFeaturesCollection("", nil)
	c.SetFeatureValues([]*FeatureHubUsageValue{
		{ID: "1", Key: "flag-a", Value: "on", RawValue: true},
		{ID: "2", Key: "flag-b", Value: "hello", RawValue: "hello"},
	})
	c.SetAdditionalData(ContextRecord{"extra": "data"})

	rec := c.CollectUsageRecord()
	assert.Equal(t, "on", rec["flag-a"])
	assert.Equal(t, true, rec["flag-a_raw"])
	assert.Equal(t, "hello", rec["flag-b"])
	assert.Equal(t, "hello", rec["flag-b_raw"])
	assert.Equal(t, "data", rec["extra"])
	assert.Equal(t, "flag-a,flag-b", rec["fhub_keys"])
}

func TestUsageFeaturesCollectionFhubKeysEmptyWhenNoFeatures(t *testing.T) {
	c := NewUsageFeaturesCollection("", nil)

	rec := c.CollectUsageRecord()
	assert.Equal(t, "", rec["fhub_keys"])
}

func TestUsageFeaturesCollectionSingleFeatureRaw(t *testing.T) {
	c := NewUsageFeaturesCollection("", nil)
	c.SetFeatureValues([]*FeatureHubUsageValue{
		{ID: "1", Key: "flag", Value: "off", RawValue: false},
	})

	rec := c.CollectUsageRecord()
	assert.Equal(t, "off", rec["flag"])
	assert.Equal(t, false, rec["flag_raw"])
	assert.Equal(t, "flag", rec["fhub_keys"])
}

// --- BaseCollectionContext ---

func TestUsageFeaturesCollectionContextEventName(t *testing.T) {
	c := NewUsageFeaturesCollectionContext("", nil)
	assert.Equal(t, "feature-collection-context", c.EventName())
}

func TestUsageFeaturesCollectionContextUserKey(t *testing.T) {
	c := NewUsageFeaturesCollectionContext("carol", nil)
	assert.Equal(t, "carol", c.UserKey())
}

func TestUsageFeaturesCollectionContextCollectUsageRecord(t *testing.T) {
	c := NewUsageFeaturesCollectionContext("", nil)
	c.SetFeatureValues([]*FeatureHubUsageValue{
		{ID: "1", Key: "flag-x", Value: "off", RawValue: false},
	})
	c.SetContextAttributes(ContextRecord{"country": "Thailand"})

	rec := c.CollectUsageRecord()
	assert.Equal(t, "off", rec["flag-x"])
	assert.Equal(t, false, rec["flag-x_raw"])
	assert.Equal(t, "flag-x", rec["fhub_keys"])
	assert.Equal(t, "Thailand", rec["country"])
}

// --- UsageNamedFeaturesCollection ---

func TestUsageNamedFeaturesCollectionEventName(t *testing.T) {
	c := NewUsageNamedFeaturesCollection("my-event", "", nil)
	assert.Equal(t, "my-event", c.EventName())
}

func TestUsageNamedFeaturesCollectionCollectUsageRecord(t *testing.T) {
	c := NewUsageNamedFeaturesCollection("checkout", "orm", ContextRecord{"session": "s1"})
	c.SetFeatureValues([]*FeatureHubUsageValue{
		{ID: "1", Key: "promo", Value: "on", RawValue: true},
	})
	c.SetContextAttributes(ContextRecord{"device": "mobile"})

	rec := c.CollectUsageRecord()
	assert.Equal(t, "s1", rec["session"])
	assert.Equal(t, "on", rec["promo"])
	assert.Equal(t, true, rec["promo_raw"])
	assert.Equal(t, "promo", rec["fhub_keys"])
	assert.Equal(t, "mobile", rec["device"])
	assert.Equal(t, "orm", c.UserKey())
}

// --- Provider ---

func TestProviderNewUsageValue(t *testing.T) {
	v := DefaultProvider.NewUsageValue("id", "key", "env-1", float64(7), models.TypeNumber)
	assert.Equal(t, "7", v.Value)
	assert.Equal(t, "env-1", v.EnvironmentID)
}

func TestProviderNewUsageValueFromFeature(t *testing.T) {
	fs := &models.FeatureState{ID: "fid", Key: "fkey", EnvironmentID: "env-2", Value: false, Type: models.TypeBoolean}
	v := DefaultProvider.NewUsageValueFromFeature(fs)
	assert.Equal(t, "off", v.Value)
	assert.Equal(t, "env-2", v.EnvironmentID)
}

func TestProviderNewUsageFeature(t *testing.T) {
	fv := &FeatureHubUsageValue{ID: "1", Key: "f", Value: "on"}
	e := DefaultProvider.NewUsageFeature(fv, nil, "")
	assert.Equal(t, "feature", e.EventName())
}

func TestProviderNewUsageCollectionEvent(t *testing.T) {
	c := DefaultProvider.NewUsageCollectionEvent("", nil)
	assert.Equal(t, "feature-collection", c.EventName())
}

func TestProviderNewUsageContextCollectionEvent(t *testing.T) {
	c := DefaultProvider.NewUsageContextCollectionEvent("eve", nil)
	assert.Equal(t, "feature-collection-context", c.EventName())
	assert.Equal(t, "eve", c.UserKey())
}

func TestProviderNewNamedUsageCollection(t *testing.T) {
	c := DefaultProvider.NewNamedUsageCollection("my-page", "", nil)
	assert.Equal(t, "my-page", c.EventName())
}

// --- Adapter ---

// mockRepo implements StreamableRepository for testing.
type mockRepo struct {
	nextID   int
	handlers map[int]StreamHandler
}

func newMockRepo() *mockRepo {
	return &mockRepo{handlers: make(map[int]StreamHandler)}
}

func (m *mockRepo) RegisterUsageStream(handler StreamHandler) int {
	m.nextID++
	m.handlers[m.nextID] = handler
	return m.nextID
}

func (m *mockRepo) RemoveUsageStream(id int) {
	delete(m.handlers, id)
}

func (m *mockRepo) emit(event UsageEvent) {
	for _, h := range m.handlers {
		h(context.TODO(), event)
	}
}

// mockPlugin records calls to Send via a buffered channel.
type mockPlugin struct {
	ch chan UsageEvent
}

func newMockPlugin() *mockPlugin {
	return &mockPlugin{ch: make(chan UsageEvent, 8)}
}

func (p *mockPlugin) DefaultPluginAttributes() ContextRecord { return nil }
func (p *mockPlugin) CanSendAsync() bool                     { return true }
func (p *mockPlugin) Close()                                 {}
func (p *mockPlugin) Send(ctx context.Context, event UsageEvent) context.Context {
	p.ch <- event
	return ctx
}

// wait blocks until n events arrive or the timeout elapses.
func (p *mockPlugin) wait(t *testing.T, n int, timeout time.Duration) []UsageEvent {
	t.Helper()
	events := make([]UsageEvent, 0, n)
	deadline := time.After(timeout)
	for len(events) < n {
		select {
		case e := <-p.ch:
			events = append(events, e)
		case <-deadline:
			t.Fatalf("timed out waiting for %d events; got %d", n, len(events))
		}
	}
	return events
}

func newTestLogger() *logrus.Logger {
	l := logrus.New()
	l.SetLevel(logrus.TraceLevel)
	return l
}

func TestAdapterDispatchesToPlugin(t *testing.T) {
	repo := newMockRepo()
	adapter := NewAdapter(repo, newTestLogger())
	plugin := newMockPlugin()
	adapter.RegisterPlugin(plugin)

	fv := &FeatureHubUsageValue{ID: "1", Key: "f", Value: "on"}
	event := NewUsageEventWithFeature(fv, nil, "")
	repo.emit(event)

	received := plugin.wait(t, 1, time.Second)
	assert.Equal(t, event, received[0])
}

func TestAdapterDispatchesToMultiplePlugins(t *testing.T) {
	repo := newMockRepo()
	adapter := NewAdapter(repo, newTestLogger())
	p1, p2 := newMockPlugin(), newMockPlugin()
	adapter.RegisterPlugin(p1)
	adapter.RegisterPlugin(p2)

	fv := &FeatureHubUsageValue{ID: "1", Key: "f", Value: "off"}
	repo.emit(NewUsageEventWithFeature(fv, nil, ""))

	p1.wait(t, 1, time.Second)
	p2.wait(t, 1, time.Second)
}

func TestAdapterPanicInPluginDoesNotStopOthers(t *testing.T) {
	repo := newMockRepo()
	adapter := NewAdapter(repo, newTestLogger())

	panicPlugin := &panickyPlugin{}
	goodPlugin := newMockPlugin()
	adapter.RegisterPlugin(panicPlugin)
	adapter.RegisterPlugin(goodPlugin)

	fv := &FeatureHubUsageValue{ID: "1", Key: "f", Value: "on"}
	repo.emit(NewUsageEventWithFeature(fv, nil, ""))

	goodPlugin.wait(t, 1, time.Second)
}

func TestAdapterCloseRemovesHandler(t *testing.T) {
	repo := newMockRepo()
	adapter := NewAdapter(repo, newTestLogger())
	plugin := newMockPlugin()
	adapter.RegisterPlugin(plugin)

	adapter.Close()

	fv := &FeatureHubUsageValue{ID: "1", Key: "f", Value: "on"}
	repo.emit(NewUsageEventWithFeature(fv, nil, ""))

	select {
	case <-plugin.ch:
		t.Fatal("plugin should not receive events after Close")
	case <-time.After(50 * time.Millisecond):
		// expected: nothing arrived
	}
}

// panickyPlugin panics on Send.
type panickyPlugin struct{}

func (*panickyPlugin) DefaultPluginAttributes() ContextRecord               { return nil }
func (*panickyPlugin) CanSendAsync() bool                                   { return true }
func (*panickyPlugin) Close()                                               {}
func (*panickyPlugin) Send(_ context.Context, _ UsageEvent) context.Context { panic("plugin error") }

// syncPlugin is a synchronous plugin that records calls and optionally enriches the context.
type syncPlugin struct {
	called bool
	ctxKey string
	ctxVal interface{}
}

func (p *syncPlugin) DefaultPluginAttributes() ContextRecord { return nil }
func (p *syncPlugin) CanSendAsync() bool                     { return false }
func (p *syncPlugin) Close()                                 {}
func (p *syncPlugin) Send(ctx context.Context, _ UsageEvent) context.Context {
	p.called = true
	if p.ctxKey != "" {
		return context.WithValue(ctx, p.ctxKey, p.ctxVal)
	}
	return ctx
}

// syncPanickyPlugin is a synchronous plugin that panics on Send.
type syncPanickyPlugin struct{}

func (*syncPanickyPlugin) DefaultPluginAttributes() ContextRecord               { return nil }
func (*syncPanickyPlugin) CanSendAsync() bool                                   { return false }
func (*syncPanickyPlugin) Close()                                               {}
func (*syncPanickyPlugin) Send(_ context.Context, _ UsageEvent) context.Context { panic("sync panic") }

func TestAdapterSyncPluginIsCalledSynchronously(t *testing.T) {
	repo := newMockRepo()
	adapter := NewAdapter(repo, newTestLogger())
	plugin := &syncPlugin{}
	adapter.RegisterPlugin(plugin)

	fv := &FeatureHubUsageValue{ID: "1", Key: "f", Value: "on"}
	repo.emit(NewUsageEventWithFeature(fv, nil, ""))

	// No goroutine — plugin.called must be true immediately after emit returns.
	assert.True(t, plugin.called)
}

func TestAdapterSyncPluginContextThreadedToNextPlugin(t *testing.T) {
	repo := newMockRepo()
	adapter := NewAdapter(repo, newTestLogger())

	first := &syncPlugin{ctxKey: "key", ctxVal: "value"}
	second := &syncPlugin{}
	adapter.RegisterPlugin(first)
	adapter.RegisterPlugin(second)

	// Manually call dispatch to inspect the returned context.
	fv := &FeatureHubUsageValue{ID: "1", Key: "f", Value: "on"}
	event := NewUsageEventWithFeature(fv, nil, "")
	resultCtx := adapter.dispatch(context.Background(), event)

	assert.True(t, first.called)
	assert.True(t, second.called)
	assert.Equal(t, "value", resultCtx.Value("key"))
}

func TestAdapterSyncPanicDoesNotStopSubsequentPlugins(t *testing.T) {
	repo := newMockRepo()
	adapter := NewAdapter(repo, newTestLogger())

	panicPlugin := &syncPanickyPlugin{}
	goodPlugin := &syncPlugin{}
	adapter.RegisterPlugin(panicPlugin)
	adapter.RegisterPlugin(goodPlugin)

	fv := &FeatureHubUsageValue{ID: "1", Key: "f", Value: "on"}
	repo.emit(NewUsageEventWithFeature(fv, nil, ""))

	assert.True(t, goodPlugin.called)
}

func TestAdapterDispatchReturnsFinalContext(t *testing.T) {
	repo := newMockRepo()
	adapter := NewAdapter(repo, newTestLogger())
	adapter.RegisterPlugin(&syncPlugin{ctxKey: "step", ctxVal: 1})

	fv := &FeatureHubUsageValue{ID: "1", Key: "f", Value: "on"}
	ctx := adapter.dispatch(context.Background(), NewUsageEventWithFeature(fv, nil, ""))

	assert.Equal(t, 1, ctx.Value("step"))
}
