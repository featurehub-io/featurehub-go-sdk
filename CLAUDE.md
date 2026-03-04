# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run all tests with coverage
make test
# or: go test ./... -cover

# Run a single test
go test ./pkg/streaming-client/ -run TestName -v

# Generate mocks (requires counterfeiter)
make mocks

# Build everything
go build ./...
```

## Architecture

This is a Go client SDK for FeatureHub, a feature management platform. The SDK connects to a FeatureHub server via Server-Sent Events (SSE) and maintains a local cache of feature flags that can be evaluated with client-side rollout strategies.

### Package Layout

- **Root package** (`client.go`): Entry point facade. `New(serverAddress, sdkKey)` returns a `Config` builder.
- **`pkg/interfaces/`**: Public API contracts (`Client`, `AnalyticsCollector`).
- **`pkg/models/`**: Domain objects — `FeatureState`, `Context`, strategy types, SSE event types.
- **`pkg/streaming-client/`**: Core implementation. The `StreamingClient` manages the SSE connection, feature cache (protected by mutex), notifier callbacks, and analytics delegation.
- **`pkg/strategies/`**: Client-side rollout strategy matchers for boolean, number, string, semver, date, datetime, and IP address attribute types.
- **`pkg/errors/`**: Typed errors: `ErrBadConfig`, `ErrFeatureNotFound`, `ErrInvalidType`, `ErrNotifierNotFound`, `ErrFromAPI`.
- **`pkg/analytics/`**: Built-in `AnalyticsCollector` implementation: `LoggingAnalyticsCollector`.
- **`pkg/mocks/`**: Generated mocks (via `make mocks` using `counterfeiter`). Do not edit manually.

### Connection Flow

```
client.New(serverAddress, sdkKey)
  → Config (builder)
  → .WithLogLevel() / .WithWaitForData() / .WithFatalErrorHandler()
  → .Connect()
      → NewStreamingClient(config)
      → client.Start() — spawns handleEvents() and handleErrors() goroutines
```

`WithWaitForData(true)` blocks `Connect()` until the first feature batch arrives from the server.

### Feature Evaluation with Rollout Strategies

`ClientWithContext` wraps a `StreamingClient` with a `Context`. When getting a feature value through `ClientWithContext`, it evaluates each strategy on `FeatureState.Strategies` in order:
1. Check percentage rule (Murmur3 hash of userKey/sessionKey)
2. Check attribute rules (device, platform, country, version, custom)
3. Return first matching strategy value, or default feature value if none match

### SSE Event Types

Handled in `pkg/streaming-client/streaming_client_handlers.go`:
- `FHFeatures` — replace entire feature cache
- `FHFeature` — update single feature (version-aware: ignores if incoming version ≤ current)
- `FHDeleteFeature` — remove feature from cache
- `FHConfig` — `edge.stale` signals the SDK to close the connection
- `FHFailure` / `FHError` — trigger the fatal error handler

### Thread Safety

The feature cache, notifiers map, and analytics collectors list are each protected by their own `sync.Mutex`. Feature writes (from SSE events) and reads (user code) are both mutex-guarded.

### SDK Key Format

The SDKKey passed to `New()` must follow the format: `{namedCache}/environmentID/APIKey`. This is validated in `pkg/streaming-client/config.go`.

### Notifier System

Callbacks are registered per feature key and identified by UUID. Multiple notifiers per key are supported. Notifiers can be registered before the server sends data and will fire on the first update.

```go
uuid := fhClient.AddNotifierBoolean(key, func(value bool) { ... })
fhClient.DeleteNotifier(key, uuid)
```