# OpenTelemetry usage plugin

This describes an OpenTelemetry plugin for the FeatureHub Go SDK. It records the features evaluated during a
request, and their values, as:

* attributes or events on the request's current span, and
* [W3C baggage](https://www.w3.org/TR/baggage/) on every outgoing call the request makes, so downstream services
  (and their traces and logs) know which features were active for the request.

The plugin is not part of the SDK, so the SDK does not depend on OpenTelemetry. It should live in its own module
(for example `github.com/featurehub-io/featurehub-go-sdk/otel`). The SDK provides the hooks it needs, described
below.

## Why the plugin cannot simply add baggage

OpenTelemetry baggage is stored in a `context.Context`, and a context cannot be changed once it is made: adding
baggage makes a *new* context. A usage plugin's `Send` is called with the request's context and can return a new
one, but that context never reaches the code that read the feature:

* the usage stream (`usage.StreamHandler`) and `EmitUsageEvent` return nothing, so the context returned by
  synchronous plugins is only passed from one plugin to the next;
* `GetBoolean`, `Boolean` and the other reads return only the value;
* `r.Context()` cannot be replaced in place for the rest of the request.

Changing every read to return a context would be a large API change, and would only work if every caller
remembered to use that context for its outgoing calls.

Instead, the plugin keeps a **mutable record for each request** in the request's context, set up once when the
request starts. `Send` adds each evaluated feature to it. The record is turned into baggage only when the request
makes an outgoing call, by an OpenTelemetry propagator. Span attributes need no such step, because a span can be
changed in place.

## SDK hooks the plugin uses

| Hook | Purpose |
|---|---|
| `usage.RequestScopedPlugin` | A `usage.Plugin` with `StartRequest(ctx context.Context) context.Context`, called once per request to add the plugin's record to the request context. |
| `core.ContextMiddleware(cfg, initialisers...)` | HTTP middleware: makes the FeatureHub context, stores it in the request, calls `StartRequest` on request-scoped plugins, then runs the initialisers (e.g. to set the user key). |
| `core.NewRequestContext(ctx, cfg)` | The same for gRPC, queue consumers and other non-HTTP requests. Returns the new Go context and the FeatureHub context. |
| `FeatureHubConfig.StartRequest(ctx)` | What the two above call; passes the context through every registered `RequestScopedPlugin`. |
| `core.FromContext(ctx)` | Returns the FeatureHub context stored in a Go context (under a private key type). |
| `CanSendAsync() == false` | `Send` runs before the feature read returns, so a read immediately followed by an outgoing call is never missed. The SDK logs a warning if a request-scoped plugin is asynchronous. |
| `usage.EventWithFeature` | The event for a single feature read: `GetFeature()` gives the key, the converted `Value` and the `RawValue`. Other events (`GetContextUsage`, `RecordNamedUsage`) are collections and can be ignored. |

## The plugin

```go
package fhotel

import (
	"context"
	"strings"
	"sync"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// evaluations is the record kept for each request. A request can read features from several goroutines, so it
// is guarded by a mutex:
type evaluations struct {
	mu     sync.Mutex
	values map[string]string // feature key -> value, the last value read wins
}

type evaluationsKey struct{}

// Options controls what the plugin records.
type Options struct {
	// Features is the allow-list of feature keys added to baggage. Baggage is sent to every downstream service,
	// including third parties, so only features that are safe to share should be listed. Empty means none.
	Features []string
	// Prefix is put before each feature key in baggage and span attributes. Default "fh.".
	Prefix string
	// SpanAttributes records each evaluation on the current span as well.
	SpanAttributes bool
}

// Plugin is a usage.RequestScopedPlugin that records evaluated features for OpenTelemetry.
type Plugin struct {
	allowed map[string]bool
	prefix  string
	span    bool
}

func New(opts Options) *Plugin {
	p := &Plugin{allowed: map[string]bool{}, prefix: opts.Prefix, span: opts.SpanAttributes}
	if p.prefix == "" {
		p.prefix = "fh."
	}
	for _, key := range opts.Features {
		p.allowed[key] = true
	}
	return p
}

var _ usage.RequestScopedPlugin = (*Plugin)(nil)

func (p *Plugin) DefaultPluginAttributes() usage.ContextRecord { return nil }

// CanSendAsync is false: each read must be recorded before it returns.
func (p *Plugin) CanSendAsync() bool { return false }
func (p *Plugin) Close()             {}

// StartRequest adds an empty record to the request's context:
func (p *Plugin) StartRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, evaluationsKey{}, &evaluations{values: map[string]string{}})
}

// Send records a feature read. It runs on every read, so it must be cheap:
func (p *Plugin) Send(ctx context.Context, event usage.UsageEvent) context.Context {
	withFeature, ok := event.(usage.EventWithFeature)
	if !ok || withFeature.GetFeature() == nil {
		return ctx
	}
	feature := withFeature.GetFeature()

	if p.span {
		trace.SpanFromContext(ctx).SetAttributes(attribute.String(p.prefix+feature.Key, feature.Value))
	}

	if p.allowed[feature.Key] {
		if evals, ok := ctx.Value(evaluationsKey{}).(*evaluations); ok {
			evals.mu.Lock()
			evals.values[feature.Key] = feature.Value
			evals.mu.Unlock()
		}
	}
	return ctx
}

// ContextWithBaggage returns ctx with the features evaluated so far added to its baggage, for code that needs a
// baggage.Baggage itself (e.g. logging). Outgoing calls get them through the Propagator instead.
func (p *Plugin) ContextWithBaggage(ctx context.Context) context.Context {
	evals, ok := ctx.Value(evaluationsKey{}).(*evaluations)
	if !ok {
		return ctx
	}
	bag := baggage.FromContext(ctx)

	evals.mu.Lock()
	defer evals.mu.Unlock()
	for key, value := range evals.values {
		// NewMemberRaw percent-encodes the value. A key that is not a valid baggage token is skipped:
		member, err := baggage.NewMemberRaw(p.prefix+key, value)
		if err != nil {
			continue
		}
		// SetMember fails if the baggage would exceed the W3C limits (8192 bytes, 180 members); keep what fits:
		if next, err := bag.SetMember(member); err == nil {
			bag = next
		}
	}
	return baggage.ContextWithBaggage(ctx, bag)
}

// Propagator wraps a propagator (normally propagation.Baggage{}) so that the features evaluated during the
// request are added to the baggage of every outgoing call:
func (p *Plugin) Propagator(next propagation.TextMapPropagator) propagation.TextMapPropagator {
	return &propagator{plugin: p, next: next}
}

type propagator struct {
	plugin *Plugin
	next   propagation.TextMapPropagator
}

func (pr *propagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	pr.next.Inject(pr.plugin.ContextWithBaggage(ctx), carrier)
}

func (pr *propagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return pr.next.Extract(ctx, carrier)
}

func (pr *propagator) Fields() []string { return pr.next.Fields() }

// FromBaggage returns the FeatureHub features in ctx's baggage (e.g. in a downstream service), without the prefix:
func (p *Plugin) FromBaggage(ctx context.Context) map[string]string {
	found := map[string]string{}
	for _, member := range baggage.FromContext(ctx).Members() {
		if key, ok := strings.CutPrefix(member.Key(), p.prefix); ok {
			found[key] = member.Value()
		}
	}
	return found
}
```

## Wiring it up

```go
fhConfig := client.New(serverAddress, sdkKey)

otelPlugin := fhotel.New(fhotel.Options{
	Features:       []string{"new-checkout", "pricing-model"},
	SpanAttributes: true,
})
fhConfig.RegisterUsagePlugin(otelPlugin)

// Outgoing calls made with otelhttp/otelgrpc now carry the evaluated features in their baggage:
otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
	propagation.TraceContext{},
	otelPlugin.Propagator(propagation.Baggage{}),
))

if _, err := fhConfig.Connect(); err != nil {
	log.Fatal(err)
}

handler := core.ContextMiddleware(fhConfig, func(r *http.Request, fhCtx interfaces.Context) {
	fhCtx.Attributes().Userkey = userFromRequest(r)
})(mux)

// otelhttp.NewHandler must wrap the FeatureHub middleware, so the span exists before features are read:
http.ListenAndServe(":8080", otelhttp.NewHandler(handler, "server"))
```

In a handler:

```go
func checkout(w http.ResponseWriter, r *http.Request) {
	hub, _ := core.NewFromContext(r.Context())
	if hub.Boolean("new-checkout", false) { // recorded on the span, and in the request's record
		// ...
	}

	// The outgoing request's baggage includes fh.new-checkout=on:
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, paymentsURL, body)
	client.Do(req)
}
```

For gRPC or a queue consumer, start each request with `core.NewRequestContext` instead of the middleware:

```go
ctx, fhCtx := core.NewRequestContext(ctx, fhConfig)
fhCtx.Attributes().Userkey = message.UserID
```

## Things to be careful of

* **Only listed features go into baggage.** Baggage is sent to every downstream service, including third
  parties, and feature values can reveal plans or customer data. Span attributes stay in your own tracing
  backend, so they can record more.
* **Baggage limits.** The W3C limit is 8192 bytes and 180 members; `SetMember` refuses anything beyond that, and
  the plugin keeps what fits. Keys must be valid tokens - feature keys of letters, digits, `_`, `-` and `.` are
  fine.
* **Values are as the SDK reports them.** `Value` comes from `usage.SetConvertFunc` (by default `on`/`off` for
  booleans), which is global - another plugin changing it changes the baggage too. Use `RawValue` if the plugin
  needs its own format.
* **The plugin must be synchronous.** With `CanSendAsync() == true`, a read immediately followed by an outgoing
  call could miss the feature. The SDK logs a warning if a request-scoped plugin is registered as asynchronous.
* **Keep `Send` cheap.** It runs on every feature read, inside the read.
* **Only requests started by the middleware or `NewRequestContext` have a record.** Reads with any other
  context (e.g. `context.Background()` in a background job) still set span attributes if there is a span, but
  nothing goes into baggage.
* **Set the context's attributes before reading features.** Use an initialiser (or set them straight after
  `NewRequestContext`). A FeatureHub context can be read from several goroutines of a request, but its attributes
  should not be changed while that happens.
