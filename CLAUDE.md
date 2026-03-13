# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run all tests with coverage
make test
# or: go test ./... -cover

# Run a single test
go test ./pkg/core/ -run TestName -v

# Build everything
go build ./...
```

## Architecture

This is a Go client SDK for FeatureHub, a feature management platform. The SDK connects to a FeatureHub server via SSE or HTTP polling and maintains a local cache of feature flags that can be evaluated with client-side rollout strategies.

### Package Layout

- **`pkg/core/`**: Primary SDK logic.
  - `Config` — builder/entry point. `NewConfig(serverAddress, sdkKey, edgeProvider)` returns a `*Config`. Implements `interfaces.FeatureHubConfig`.
  - `ClientFeatureHubRepository` — local feature cache with notifiers, readiness state, and mutex protection. Also implements `usage.StreamableRepository` (`RegisterUsageStream`, `RemoveUsageStream`, `EmitUsageEvent`).
  - `ClientWithContext` — bundles a `*models.Context` with a repository for strategy-aware feature evaluation. Emits `BaseWithFeature` usage events on every feature read. Implements `interfaces.Context`.
  - `ContextFeatureHub` — implements `interfaces.FeatureHubContext` by wrapping an `interfaces.Context` retrieved from a Go `context.Context`. All method calls delegate to the wrapped context using the stored Go context as the first argument.
    - `NewFromContext(ctx context.Context) (*ContextFeatureHub, error)` — extracts the `interfaces.Context` stored under the `"featurehub"` key.
    - `StoreInContext(ctx context.Context, fhCtx interfaces.Context) context.Context` — stores an `interfaces.Context` into a Go context for later retrieval.
    - `ContextMiddleware(fhConfig interfaces.FeatureHubConfig) func(http.Handler) http.Handler` — HTTP middleware that calls `fhConfig.NewContext()` and stores it in the request context on every request.
  - `EdgeProviderFunc` — pluggable factory: `func(config *Config, internalRepository interfaces.InternalRepository) (interfaces.EdgeClient, error)`
  - `EnvOrDefaultStr` / `EnvOrDefaultDuration` — helpers for reading config from environment variables.
- **`pkg/interfaces/`**: Public API contracts.
  - `RepositoryContext` — feature reads (`GetBoolean`, `GetNumber`, `GetString`, `GetRawJSON`), convenience reads (`Boolean`, `Number`, `String`, `JSON` with default values), notifiers, `Properties`, `AllKeys`. All methods take `context.Context` as first parameter.
  - `FeatureRepository` — internal read API used by `ClientWithContext`: `GetFeature`, `GetInternalString`, `GetInternalNumber`, `GetInternalBoolean`, `GetFeatures`, `UsageProvider`, `EmitUsageEvent(context.Context, UsageEvent)`.
  - `Context` — extends `RepositoryContext` with `Attributes()`, `WithContext()`, `RecordUsageEvent(ctx, event)`, `GetContextUsage(ctx)`, `RecordNamedUsage(ctx, name, params)`, `AsConvertibleString(ctx, key)`.
  - `FeatureHubContext` — mirrors `Context` but omits `context.Context` from every method. Implemented by `ContextFeatureHub` for use in HTTP handlers and other call sites that hold their own `context.Context`.
  - `FeatureHubConfig` — interface for `*Config`, covering the full lifecycle: `Connect()`, `Build()`, `Close()`, `NewContext()`, `WithContext()`, `IsReady()`, `ReadinessListener()`, `EdgeType()`, `Validate()`, `EnvironmentID()`, `ClientEvaluated()`, `FeaturesURL()`, `PollingFeaturesURL()`, `Timeout()`, and fluent builder methods (`WithSDKKey`, `WithLogLevel`, `WithWaitForData`, `WithFatalErrorHandler`, `RegisterUsagePlugin`, `AddValueInterceptor`). Fluent methods and `Connect`/`Build` return `FeatureHubConfig`; `NewContext`/`WithContext` return `Context`.
  - `InternalRepository` — write side: `ProcessFeature`, `ProcessFeatures`, `ProcessDeleteFeature`, `IsReady`, `AddValueInterceptor`, `WithContext`, `ReadinessListener(context.Context, func(context.Context))`.
  - `EdgeClient` — `Connect()`, `Poll()`, `ContextChange()`, `Close()`.
  - `FeatureValueInterceptor` — `func(ctx context.Context, key string, feature *models.FeatureState) (value interface{}, matched bool)`. Note: value is returned first, matched bool second.
  - `ErrorFunc` — `func(error, string, map[string]interface{})` for fatal async errors.
- **`pkg/models/`**: Domain objects.
  - `FeatureState` (including `Properties map[string]string` field serialised as `"fp"`), `FeatureEnvironmentCollection`, `Context` (with `GenerateHeader()` for sorted URL-encoded header strings), strategy types, SSE event types, callback func types.
  - `EdgeType string` — edge connection strategy type. Constants: `EdgeStreaming`, `EdgeActiveRest`, `EdgePassiveRest`.
  - `FeatureValueType string` — feature value type. Constants: `TypeBoolean`, `TypeString`, `TypeNumber`, `TypeJSON`.
  - `ConvertValue(typeName FeatureValueType, raw interface{}) (interface{}, error)` — normalises raw values (bool, int, int64, float64, string) to the correct Go type for the given feature type. Recognises `"on"`/`"yes"`/`"y"`/`"t"` as boolean true and their inverses as false.
  - `ConvertToString(typeName FeatureValueType, raw interface{}) (string, error)` — converts a raw value to the string representation used in usage events (`"t"`/`"f"` for booleans, `%v` for numbers, passthrough for strings/JSON).
  - Callback types all take `context.Context` as first parameter: `CallbackFuncBoolean func(context.Context, bool)`, `CallbackFuncString func(context.Context, string)`, `CallbackFuncNumber func(context.Context, float64)`, `CallbackFuncJSON func(context.Context, string)`, `CallbackFuncFeature func(context.Context, *FeatureState)`.
- **`pkg/streaming-client/`**: SSE-based `EdgeClient` implementation. `StreamingClient` manages the SSE connection and delegates all feature storage to a `ClientFeatureHubRepository`. Has a `Close()` method that sets `stopped=true`, preventing reconnection.
- **`pkg/polling-client/`**: HTTP polling `EdgeClient` implementation. `FeatureHubPollingClient` supports active (timer-based) and passive (cache-expiry-based) modes. `PollingBase` handles the low-level HTTP GET mechanics (etag, cache-control, SHA-256 context header hashing, concurrent-caller coalescing). `Poll()` and `ContextChange()` dispatch asynchronously.
- **`pkg/strategies/`**: Client-side rollout strategy matchers for boolean, number, string, semver, date, datetime, and IP address attribute types.
- **`pkg/errors/`**: Typed errors: `ErrBadConfig`, `ErrFeatureNotFound`, `ErrInvalidType`, `ErrNotifierNotFound`, `ErrFromAPI`, `ErrFeatureIsWrongType`, `ErrInvalidNotifierCallback`.
- **`pkg/interceptors/`**: Built-in `FeatureValueInterceptor` implementations.
  - `LocalYamlValueInterceptor` — reads feature overrides from a YAML file specified by `FEATUREHUB_OVERRIDES` env var (defaults to `featurehub-overrides.yaml`). YAML is a list of `{key, type, value}` entries. Values are converted via `models.ConvertValue` at initialisation time.
- **`pkg/usage/`**: Usage/analytics subsystem.
  - `UsageEvent` interface + concrete types: `BaseWithFeature`, `BaseFeaturesCollection`, `BaseCollectionContext`, `UsageNamedFeaturesCollection`.
  - `Plugin` interface: `DefaultPluginAttributes() ContextRecord`, `Send(context.Context, UsageEvent) context.Context`.
  - `StreamHandler` — `func(context.Context, UsageEvent)` — called when a usage event is emitted by the repository.
  - `Adapter` — subscribes to a `StreamableRepository` and fans events out to registered `Plugin`s. Each plugin's `Send` call runs in its own goroutine (panics are caught and logged).
  - `ProviderFactory` / `Provider` — factory for constructing usage event objects; injectable via `ClientFeatureHubRepository.UsageProvider()`.
  - `ConvertFunc` — global hook for customising how feature values are stringified in usage events.
  - `ContextRecord` — `map[string]interface{}` alias for context attributes and additional event data.

### Connection Flow

```
client.New(serverAddress, sdkKey)           // root package wires all edge types; returns *core.Config
  → core.NewConfig(..., edgeProviderFunc)
  → .Streaming()  /  .ActiveRest(interval)  /  .PassiveRest(interval)
  → .WithSDKKey(key)                         // optional: add extra SDK keys for multi-env polling
  → .WithLogLevel() / .WithWaitForData() / .WithFatalErrorHandler()
  → .RegisterUsagePlugin(plugin)            // optional: attach usage analytics
  → .Connect() → (interfaces.FeatureHubConfig, error)
      → EdgeProviderFunc → StreamingClient | FeatureHubPollingClient
      → client.Connect()
  → .Build(context) → (interfaces.FeatureHubConfig, error)
                                            // optional: server-evaluated mode — connects + sends header
  → .NewContext() → interfaces.Context      (strategy-aware reads + usage emission)
  → .WithContext(ctx) → interfaces.Context  (attach a pre-built models.Context)
  → .Close()                                // shut down the edge client
```

HTTP middleware pattern:

```
core.ContextMiddleware(fhConfig)            // wraps each request: stores NewContext() in r.Context()
core.NewFromContext(r.Context())            // retrieves ContextFeatureHub (interfaces.FeatureHubContext)
  → hub.GetBoolean("myFlag")               // no context.Context param needed
```

`NewConfig` reads environment variables on startup to set a default edge type:
- `FEATUREHUB_POLLING_INTERVAL` (e.g. `"30s"`) → active REST polling at that interval
- `FEATUREHUB_POLLING_PASSIVE` (any non-empty value) → passive REST polling
- Neither set → SSE streaming (default)
- `FEATUREHUB_POLLING_REQ_TIMEOUT` — overrides the HTTP request timeout (default `8s`) in the polling client

`WithWaitForData(duration)` blocks `Connect()` until the first feature batch arrives (or the duration elapses).

### Repository Split

`ClientFeatureHubRepository` implements both `interfaces.RepositoryContext` (public read API) and `interfaces.InternalRepository` (write API for edge providers). Edge providers only receive the `InternalRepository` side, keeping the write path internal.

The `Config` manages a single `*ClientFeatureHubRepository` by default. External repository implementations can be injected via `Config.SetRepository()`. `SetRepository` also creates a new `usage.Adapter` and registers the built-in `passiveRestPollPlugin`.

### Context Propagation

All feature-read methods (`GetBoolean`, `GetNumber`, `GetString`, `GetRawJSON`), convenience methods (`Boolean`, `Number`, `String`, `JSON`), `Properties`, `AllKeys`, notifier registration (`AddNotifier*`), `ReadinessListener`, and usage emission (`EmitUsageEvent`, `RecordUsageEvent`, `GetContextUsage`, `RecordNamedUsage`) take `context.Context` as their first parameter.

The context flows through to usage plugin `Send(context.Context, UsageEvent) context.Context` and `StreamHandler func(context.Context, UsageEvent)` calls, enabling tracing and cancellation propagation.

`ContextFeatureHub` / `FeatureHubContext` provide a context-free façade where the Go context is captured once (at `NewFromContext` time) and reused for every delegated call.

### Usage System

`ClientFeatureHubRepository` implements `usage.StreamableRepository`. Every feature read through `ClientWithContext` emits a `BaseWithFeature` event via `EmitUsageEvent(ctx, event)`.

`Config.SetRepository` creates a `usage.Adapter` and registers a `passiveRestPollPlugin` that calls `client.Poll()` on every usage event when `EdgeType() == models.EdgePassiveRest`. This keeps the feature cache fresh in passive-REST mode without requiring the host to manually poll.

Additional plugins are registered with `Config.RegisterUsagePlugin(plugin)`. Each plugin's `Send` runs in its own goroutine; panics are caught and logged.

```go
config.RegisterUsagePlugin(myAnalyticsPlugin)
```

### Feature Value Interceptors

Interceptors are functions registered with `Config.AddValueInterceptor(fn)` or `repo.AddValueInterceptor(fn)` that can override feature values before evaluation. The interceptor signature is:

```go
type FeatureValueInterceptor func(ctx context.Context, key string, feature *models.FeatureState) (value interface{}, matched bool)
```

**Note:** the return order is `(value, matched)` — value first, bool second.

Return `(value, true)` to supply an override, or `(nil, false)` to pass through. Interceptors are checked before rollout strategies; first match wins.

The built-in `LocalYamlValueInterceptor` (in `pkg/interceptors/`) reads from a YAML file at the path specified by `FEATUREHUB_OVERRIDES`. Values are converted via `models.ConvertValue` at startup.

Interceptors that match against a key with a nil feature state (unknown key) do **not** emit a usage event.

### Feature Evaluation with Rollout Strategies

`ClientWithContext` delegates reads to its `interfaces.FeatureRepository` but applies strategy evaluation on top. When getting a typed value through `ClientWithContext`:
1. Check interceptors — if matched and feature state is non-nil, emit usage and return.
2. Check percentage rule (Murmur3 hash of userKey/sessionKey).
3. Check attribute rules (device, platform, country, version, custom).
4. Return first matching strategy value (with usage event), or default feature value if none match.

### Multi-SDK-Key Support

`Config` supports polling multiple SDK keys simultaneously via `WithSDKKey()`:

```go
config.WithSDKKey("default/env-2/key-2").WithSDKKey("default/env-3/key-3")
```

`PollingFeaturesURL()` produces a URL with each key as a separate `apiKey=` query parameter:
```
/features?apiKey=default/env-1/key-1&apiKey=default/env-2/key-2
```

### Build Method (Server-Evaluated Mode)

`Config.Build(context *models.Context) (interfaces.FeatureHubConfig, error)` is the entry point for server-evaluated mode:
- If the SDK key is client-evaluated (contains `"*"`), returns immediately without connecting.
- If no edge client exists, creates one via the `EdgeProviderFunc` and calls `Connect()`.
- Calls `ContextChange(header)` with the header generated from the provided `*models.Context`.

### Edge Types

Edge types are defined in `pkg/models` as `models.EdgeType`:
- `models.EdgeStreaming` — SSE streaming (default)
- `models.EdgeActiveRest` — active REST polling on a timer
- `models.EdgePassiveRest` — passive REST polling on demand

**SSE streaming** (`pkg/streaming-client`): long-lived SSE connection. Events dispatched to `InternalRepository` as they arrive. Status 236 / `edge.stale` closes the connection. `Close()` sets `stopped=true`; a stopped client will refuse to reconnect.

**Active REST polling** (`pkg/polling-client`): polls on a timer at `config.Timeout()` interval. The server may shorten the interval via `Cache-Control: max-age`. HTTP 404/400 → fatal (stops polling). Other errors → logged and retried on the next timer tick. HTTP 236 → stops polling.

**Passive REST polling** (`pkg/polling-client`): polls only when the cache has expired (driven by `Cache-Control: max-age` from the server). Subsequent `Poll()` calls before expiry are no-ops. Useful when the host environment controls when polling occurs. When usage events are enabled, the `passiveRestPollPlugin` also triggers a poll on each usage event.

Both REST modes use the polling endpoint (`/features?apiKey=…`), which returns `[]FeatureEnvironmentCollection`. Features are flattened and pushed via `ProcessFeatures`. The `contextSha` query parameter (SHA-256/base64-URL of the `x-featurehub` context header) is always appended; it is `"0"` when no context header is set.

`Poll()` and `ContextChange()` on the polling client execute asynchronously.

### SSE Event Types

Handled in `pkg/streaming-client/streaming_client_handlers.go`:
- `FHFeatures` — replace entire feature cache (`ProcessFeatures`)
- `FHFeature` — update single feature, version-aware (`ProcessFeature`)
- `FHDeleteFeature` — remove feature from cache (`ProcessDeleteFeature`)
- `FHConfig` — `edge.stale` signals the SDK to close the connection
- `FHFailure` / `FHError` — trigger the fatal error handler

### Thread Safety

`ClientFeatureHubRepository` uses two separate `sync.Mutex` instances: one for `features` and one for `notifiers`. Feature writes (SSE events) and reads (user code) are both mutex-guarded. Usage stream handlers are stored in a map guarded by a third mutex (`usageStreamsMu`); `EmitUsageEvent` iterates the map without locking (streams change rarely).

### Feature Identity and Key Mutability

Features are stored internally by **ID**, not by key. The `ID` field is the only immutable identifier; the user-visible `Key` can change between server updates. `ClientFeatureHubRepository` maintains two maps:

- `features map[string]*models.FeatureState` — keyed by feature ID
- `keyIndex map[string]string` — maps current key → ID

All three write methods (`ProcessFeature`, `ProcessFeatures`, `ProcessDeleteFeature`) and all read methods (`GetFeature`, `Properties`) operate through this two-level index. When a key rename is detected during `ProcessFeature`, the stale key is removed from `keyIndex` atomically. `ProcessDeleteFeature` looks up by ID first; if the payload carries no ID it falls back to the key index.

The `featureID(feature)` helper returns `feature.ID` when set, or `feature.Key` as a fallback for older server payloads that omit the ID.

**Every `FeatureState` created in tests must include a non-empty `ID` field.** The ID is required; omitting it causes the feature to be indexed under its key, losing the key-mutability guarantee.

### SDK Key Format

The SDKKey passed to `NewConfig()` follows one of two formats:
- `{namedCache}/environmentID/APIKey` (3-part)
- `environmentID/APIKey` (2-part)

Keys containing `"*"` are client-evaluated (`Config.ClientEvaluated() == true`). `Config.EnvironmentID()` extracts the environment ID regardless of which format is used. Validated in `pkg/core/config.go`.

### Notifier System

Callbacks are registered per feature key and identified by UUID. Multiple notifiers per key are supported. Notifiers can be registered before the server sends data and will fire on the first update. All callback types take `context.Context` as their first parameter.

```go
uuid, err := fhClient.AddNotifierBoolean(ctx, key, func(ctx context.Context, value bool) { ... })
fhClient.DeleteNotifier(key, uuid)
```

### Feature Properties

`FeatureState` carries an optional `Properties map[string]string` field (JSON key `"fp"`). Access via:

```go
props := repo.Properties(ctx, featureKey)   // returns nil if feature absent or has no properties
props := ctx.Properties(ctx, featureKey)    // delegates to the underlying repository
```

### Context.GenerateHeader

`models.Context.GenerateHeader()` produces a sorted, URL-encoded `key=value` header string (ampersand-separated) from all non-empty context fields. This string is sent as the `x-featurehub` header for server-evaluated mode and hashed (SHA-256/base64-URL) as the `contextSha` query parameter in polling requests.

### EdgeType

`Config.EdgeType()` returns the currently configured `models.EdgeType`. Calling `Streaming()`, `ActiveRest()`, or `PassiveRest()` closes any existing edge client and updates the type. The type constants (`models.EdgeStreaming`, `models.EdgeActiveRest`, `models.EdgePassiveRest`) live in `pkg/models`.

### Config Lifecycle

```go
config := core.NewConfig(server, key, edgeProvider)  // returns *core.Config
config.ActiveRest(30 * time.Second)                   // still returns *Config (not in interface)
fhCfg, err := config.Connect()                        // returns (interfaces.FeatureHubConfig, error)
// ... use fhCfg ...
fhCfg.Close()                                          // shuts down edge client and nils the reference
```

`Config.Close()` is idempotent — safe to call when no client is connected.
