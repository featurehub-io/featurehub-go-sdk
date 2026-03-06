# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run all tests with coverage
make test
# or: go test ./... -cover

# Run a single test
go test ./pkg/core/ -run TestName -v

# Generate mocks (requires counterfeiter)
make mocks

# Build everything
go build ./...
```

## Architecture

This is a Go client SDK for FeatureHub, a feature management platform. The SDK connects to a FeatureHub server via SSE or HTTP polling and maintains a local cache of feature flags that can be evaluated with client-side rollout strategies.

### Package Layout

- **`pkg/core/`**: Primary SDK logic.
  - `Config` — builder/entry point. `NewConfig(serverAddress, sdkKey, edgeProvider)` returns a `*Config`.
  - `ClientFeatureHubRepository` — local feature cache with notifiers, readiness state, and mutex protection.
  - `ClientWithContext` — bundles a `*models.Context` with a `Repository` for strategy-aware feature evaluation.
  - `EdgeProviderFunc` — pluggable factory: `func(config *Config, internalRepository interfaces.InternalRepository) (interfaces.EdgeClient, error)`
  - `EnvOrDefaultStr` / `EnvOrDefaultDuration` — helpers for reading config from environment variables.
- **`pkg/interfaces/`**: Public API contracts.
  - `Repository` — feature reads, notifiers, readiness, `WithContext`.
  - `InternalRepository` — write side: `ProcessFeature`, `ProcessFeatures`, `ProcessDeleteFeature`, `IsReady`.
  - `EdgeClient` — `Connect()` only.
  - `ErrorFunc` — `func(error, string, map[string]interface{})` for fatal async errors.
- **`pkg/models/`**: Domain objects — `FeatureState`, `FeatureEnvironmentCollection`, `Context`, strategy types, SSE event types, callback func types.
- **`pkg/streaming-client/`**: SSE-based `EdgeClient` implementation. `StreamingClient` manages the SSE connection and delegates all feature storage to a `ClientFeatureHubRepository` (from `pkg/core`).
- **`pkg/polling-client/`**: HTTP polling `EdgeClient` implementation. `FeatureHubPollingClient` supports active (timer-based) and passive (cache-expiry-based) modes. `PollingBase` handles the low-level HTTP GET mechanics (etag, cache-control, SHA-256 context header hashing, concurrent-caller coalescing).
- **`pkg/strategies/`**: Client-side rollout strategy matchers for boolean, number, string, semver, date, datetime, and IP address attribute types.
- **`pkg/errors/`**: Typed errors: `ErrBadConfig`, `ErrFeatureNotFound`, `ErrInvalidType`, `ErrNotifierNotFound`, `ErrFromAPI`, `ErrFeatureIsWrongType`, `ErrInvalidNotifierCallback`.
- **`pkg/mocks/`**: Generated mocks (via `make mocks` using `counterfeiter` from `pkg/interfaces/repository.go`). Do not edit manually.

### Connection Flow

```
client.New(serverAddress, sdkKey)           // root package wires all edge types
  → core.NewConfig(..., edgeProviderFunc)
  → .Streaming()  /  .ActiveRest(interval)  /  .PassiveRest(interval)
  → .WithLogLevel() / .WithWaitForData() / .WithFatalErrorHandler()
  → .Connect()
      → EdgeProviderFunc → StreamingClient | FeatureHubPollingClient
      → client.Connect()
  → .NewContext() → *ClientWithContext  (strategy-aware reads)
  → .Repository() → interfaces.Repository (context-free reads)
```

`NewConfig` reads environment variables on startup to set a default edge type:
- `FEATUREHUB_POLLING_INTERVAL` (e.g. `"30s"`) → active REST polling at that interval
- `FEATUREHUB_POLLING_PASSIVE` (any non-empty value) → passive REST polling
- Neither set → SSE streaming (default)
- `FEATUREHUB_POLLING_REQ_TIMEOUT` — overrides the HTTP request timeout (default `8s`) in the polling client

`WithWaitForData(duration)` blocks `Connect()` until the first feature batch arrives (or the duration elapses).

### Repository Split

`ClientFeatureHubRepository` implements both `interfaces.Repository` (public read API) and `interfaces.InternalRepository` (write API for edge providers). Edge providers only receive the `InternalRepository` side, keeping the write path internal.

The `Config` manages a single `*ClientFeatureHubRepository` by default. External repository implementations can be injected via `Config.SetRepository()` + `Config.SetInternalRepository()`.

### Feature Evaluation with Rollout Strategies

`ClientWithContext` delegates reads to its `interfaces.Repository` but applies strategy evaluation on top. When getting a typed value through `ClientWithContext`:
1. Check percentage rule (Murmur3 hash of userKey/sessionKey)
2. Check attribute rules (device, platform, country, version, custom)
3. Return first matching strategy value, or default feature value if none match

### Edge Types

**SSE streaming** (`pkg/streaming-client`): long-lived SSE connection. Events dispatched to `InternalRepository` as they arrive. Status 236 / `edge.stale` closes the connection.

**Active REST polling** (`pkg/polling-client`): polls on a timer at `config.Timeout()` interval. The server may shorten the interval via `Cache-Control: max-age`. HTTP 404/400 → fatal (stops polling). Other errors → logged and retried on the next timer tick. HTTP 236 → stops polling.

**Passive REST polling** (`pkg/polling-client`): polls only when the cache has expired (driven by `Cache-Control: max-age` from the server). Subsequent `Poll()` calls before expiry are no-ops. Useful when the host environment controls when polling occurs.

Both REST modes use the polling endpoint (`/features?apiKey=…`), which returns `[]FeatureEnvironmentCollection`. Features are flattened and pushed via `ProcessFeatures`. The `contextSha` query parameter (SHA-256/base64-URL of the `x-featurehub` context header) is always appended; it is `"0"` when no context header is set.

### SSE Event Types

Handled in `pkg/streaming-client/streaming_client_handlers.go`:
- `FHFeatures` — replace entire feature cache (`ProcessFeatures`)
- `FHFeature` — update single feature, version-aware (`ProcessFeature`)
- `FHDeleteFeature` — remove feature from cache (`ProcessDeleteFeature`)
- `FHConfig` — `edge.stale` signals the SDK to close the connection
- `FHFailure` / `FHError` — trigger the fatal error handler

### Thread Safety

`ClientFeatureHubRepository` uses two separate `sync.Mutex` instances: one for `features` and one for `notifiers`. Feature writes (SSE events) and reads (user code) are both mutex-guarded.

### SDK Key Format

The SDKKey passed to `NewConfig()` must follow the format: `{namedCache}/environmentID/APIKey`. Validated in `pkg/core/config.go`.

### Notifier System

Callbacks are registered per feature key and identified by UUID. Multiple notifiers per key are supported. Notifiers can be registered before the server sends data and will fire on the first update.

```go
uuid, err := fhClient.AddNotifierBoolean(key, func(value bool) { ... })
fhClient.DeleteNotifier(key, uuid)
```