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

This is a Go client SDK for FeatureHub, a feature management platform. The SDK connects to a FeatureHub server via Server-Sent Events (SSE) and maintains a local cache of feature flags that can be evaluated with client-side rollout strategies.

### Package Layout

- **`pkg/core/`**: Primary SDK logic.
  - `Config` — builder/entry point. `NewConfig(serverAddress, sdkKey, edgeProvider)` returns a `*Config`.
  - `ClientFeatureHubRepository` — local feature cache with notifiers, readiness state, and mutex protection.
  - `ClientWithContext` — bundles a `*models.Context` with a `Repository` for strategy-aware feature evaluation.
  - `EdgeProviderFunc` — pluggable factory: `func(config *Config, internalRepository interfaces.InternalRepository) (interfaces.EdgeClient, error)`
- **`pkg/interfaces/`**: Public API contracts.
  - `Repository` — feature reads, notifiers, readiness, `WithContext`.
  - `InternalRepository` — write side: `ProcessFeature`, `ProcessFeatures`, `ProcessDeleteFeature`, `IsReady`.
  - `EdgeClient` — `Connect()` only.
  - `ErrorFunc` — `func(error, string, map[string]interface{})` for fatal async errors.
- **`pkg/models/`**: Domain objects — `FeatureState`, `Context`, strategy types, SSE event types, callback func types.
- **`pkg/streaming-client/`**: SSE-based `EdgeClient` implementation. `StreamingClient` manages the SSE connection and delegates all feature storage to a `ClientFeatureHubRepository` (from `pkg/core`). Registered as an `EdgeProviderFunc`.
- **`pkg/strategies/`**: Client-side rollout strategy matchers for boolean, number, string, semver, date, datetime, and IP address attribute types.
- **`pkg/errors/`**: Typed errors: `ErrBadConfig`, `ErrFeatureNotFound`, `ErrInvalidType`, `ErrNotifierNotFound`, `ErrFromAPI`, `ErrFeatureIsWrongType`, `ErrInvalidNotifierCallback`.
- **`pkg/mocks/`**: Generated mocks (via `make mocks` using `counterfeiter` from `pkg/interfaces/repository.go`). Do not edit manually.

### Connection Flow

```
core.NewConfig(serverAddress, sdkKey, edgeProvider)
  → Config (builder)
  → .WithLogLevel() / .WithWaitForData() / .WithFatalErrorHandler()
  → .Connect()
      → EdgeProviderFunc(config, internalRepository) → interfaces.EdgeClient
      → client.Connect() — e.g. StreamingClient spawns SSE goroutines
  → .NewContext() → *ClientWithContext  (for strategy-aware reads)
  → .Repository() → interfaces.Repository (for context-free reads)
```

`WithWaitForData(duration)` blocks `Connect()` until the first feature batch arrives.

### Repository Split

`ClientFeatureHubRepository` implements both `interfaces.Repository` (public read API) and `interfaces.InternalRepository` (write API for edge providers). Edge providers only receive the `InternalRepository` side, keeping the write path internal.

The `Config` manages a single `*ClientFeatureHubRepository` by default. External repository implementations can be injected via `Config.SetRepository()` + `Config.SetInternalRepository()`.

### Feature Evaluation with Rollout Strategies

`ClientWithContext` delegates reads to its `interfaces.Repository` but applies strategy evaluation on top. When getting a typed value through `ClientWithContext`:
1. Check percentage rule (Murmur3 hash of userKey/sessionKey)
2. Check attribute rules (device, platform, country, version, custom)
3. Return first matching strategy value, or default feature value if none match

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