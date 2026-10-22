package usage

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

// StreamHandler is called whenever a usage event is emitted by the repository.
type StreamHandler func(context context.Context, event UsageEvent)

// StreamableRepository is a repository that supports usage-event streaming.
type StreamableRepository interface {
	RegisterUsageStream(handler StreamHandler) int
	RemoveUsageStream(id int)
}

// pluginSet is the registered plugins, and those that are request scoped. It is never changed once published, so
// it is read without a lock:
type pluginSet struct {
	all    []Plugin
	scoped []RequestScopedPlugin
}

// Adapter fans usage events from a StreamableRepository out to registered Plugins.
type Adapter struct {
	pluginsMu  sync.Mutex                // makes RegisterPlugin and Close take turns, and guards closed
	plugins    atomic.Pointer[pluginSet] // replaced (never changed) by RegisterPlugin, so every read is lock-free
	closed     bool
	repository StreamableRepository
	handlerID  int
	logger     *logrus.Logger
}

// NewAdapter creates an Adapter and immediately subscribes to the repository's usage stream. A nil logger uses
// the standard logrus logger.
func NewAdapter(repository StreamableRepository, logger *logrus.Logger) *Adapter {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	a := &Adapter{
		repository: repository,
		logger:     logger,
	}
	a.plugins.Store(&pluginSet{})
	a.handlerID = repository.RegisterUsageStream(func(ctx context.Context, event UsageEvent) {
		a.dispatch(ctx, event)
	})
	return a
}

// RegisterPlugin adds a plugin to receive usage events, and reports whether it was added. A closed adapter closes
// the plugin instead, so it is never left open.
func (a *Adapter) RegisterPlugin(plugin Plugin) bool {
	scoped, isScoped := plugin.(RequestScopedPlugin)
	if isScoped && plugin.CanSendAsync() {
		a.logger.Warn("A request-scoped usage plugin sends asynchronously, so it may miss features read just before an outgoing call")
	}
	a.pluginsMu.Lock()
	if a.closed {
		a.pluginsMu.Unlock()
		plugin.Close()
		return false
	}
	defer a.pluginsMu.Unlock()

	// the published slices are never changed, so they are copied when added to, and otherwise shared
	current := a.currentPlugins()
	next := &pluginSet{
		all:    append(append(make([]Plugin, 0, len(current.all)+1), current.all...), plugin),
		scoped: current.scoped,
	}
	if isScoped {
		next.scoped = append(append(make([]RequestScopedPlugin, 0, len(current.scoped)+1), current.scoped...), scoped)
	}
	a.plugins.Store(next)
	return true
}

// currentPlugins returns the registered plugins, which must not be changed:
func (a *Adapter) currentPlugins() *pluginSet {
	return a.plugins.Load()
}

// StartRequest gives each RequestScopedPlugin the chance to add its state for a new request to ctx, and returns
// the resulting context. A plugin that panics or returns nil leaves the context as it was.
func (a *Adapter) StartRequest(ctx context.Context) context.Context {
	for _, scoped := range a.currentPlugins().scoped {
		func() {
			defer func() {
				if r := recover(); r != nil {
					a.logger.WithField("panic", r).Error("usage plugin panicked during StartRequest")
				}
			}()
			if next := scoped.StartRequest(ctx); next != nil {
				ctx = next
			}
		}()
	}
	return ctx
}

// Close unregisters the adapter from the repository's usage stream and closes all plugins. Plugins registered
// afterwards are closed straight away. Idempotent.
func (a *Adapter) Close() {
	a.pluginsMu.Lock()
	if a.closed {
		a.pluginsMu.Unlock()
		return
	}
	a.closed = true
	a.pluginsMu.Unlock()

	a.repository.RemoveUsageStream(a.handlerID)
	for _, p := range a.currentPlugins().all {
		p.Close()
	}
}

func (a *Adapter) dispatch(ctx context.Context, event UsageEvent) context.Context {
	for _, p := range a.currentPlugins().all {
		if p.CanSendAsync() {
			// ctx is passed in, as a later synchronous plugin replaces the loop's ctx while this runs
			go func(p Plugin, ctx context.Context) {
				defer func() {
					if r := recover(); r != nil {
						a.logger.WithField("panic", r).Error("usage plugin panicked during Send")
					}
				}()
				p.Send(ctx, event)
			}(p, ctx)
		} else {
			func() {
				defer func() {
					if r := recover(); r != nil {
						a.logger.WithField("panic", r).Error("usage plugin panicked during Send")
					}
				}()
				// a plugin returning nil leaves the context as it was, as in StartRequest
				if next := p.Send(ctx, event); next != nil {
					ctx = next
				}
			}()
		}
	}
	return ctx
}
