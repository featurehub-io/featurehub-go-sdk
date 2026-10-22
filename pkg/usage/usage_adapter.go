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
	pluginsMu  sync.Mutex                // makes RegisterPlugin and Close take turns
	plugins    atomic.Pointer[pluginSet] // replaced (never changed) by RegisterPlugin, so every read is lock-free
	closed     atomic.Bool               // set (under pluginsMu) by Close; read without a lock by dispatch
	active     atomic.Int64              // dispatches, async sends and StartRequests in progress
	idleMu     sync.Mutex
	idle       *sync.Cond // signalled when active reaches zero once closed
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
	a.idle = sync.NewCond(&a.idleMu)
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
	if a.closed.Load() {
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
	if !a.enter() {
		return ctx
	}
	defer a.leave()
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

// enter records a dispatch, send or StartRequest starting, and reports false (recording nothing) if the adapter
// is closed. Every true return must be matched by a call to leave:
func (a *Adapter) enter() bool {
	a.active.Add(1)
	// Close sets closed before it waits for active to reach zero, so either it waits for this, or this sees closed
	if a.closed.Load() {
		a.leave()
		return false
	}
	return true
}

func (a *Adapter) leave() {
	if a.active.Add(-1) == 0 && a.closed.Load() {
		a.idleMu.Lock()
		a.idle.Broadcast()
		a.idleMu.Unlock()
	}
}

// Close unregisters the adapter from the repository's usage stream, waits for the dispatches already in progress
// (including asynchronous sends) to finish, and then closes all plugins, so no plugin is sent an event during or
// after its Close. Plugins registered afterwards are closed straight away. Idempotent. A plugin's Send must not
// close the adapter (or the config that owns it), as Close would wait for that Send.
func (a *Adapter) Close() {
	a.pluginsMu.Lock()
	if a.closed.Load() {
		a.pluginsMu.Unlock()
		return
	}
	a.closed.Store(true)
	a.pluginsMu.Unlock()

	a.repository.RemoveUsageStream(a.handlerID)

	a.idleMu.Lock()
	for a.active.Load() > 0 {
		a.idle.Wait()
	}
	a.idleMu.Unlock()

	for _, p := range a.currentPlugins().all {
		p.Close()
	}
}

func (a *Adapter) dispatch(ctx context.Context, event UsageEvent) context.Context {
	if !a.enter() {
		return ctx
	}
	defer a.leave()
	for _, p := range a.currentPlugins().all {
		if p.CanSendAsync() {
			// recorded before the goroutine starts, so Close waits for it
			a.active.Add(1)
			// ctx is passed in, as a later synchronous plugin replaces the loop's ctx while this runs
			go func(p Plugin, ctx context.Context) {
				defer a.leave()
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
