package usage

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
)

// StreamHandler is called whenever a usage event is emitted by the repository.
type StreamHandler func(context context.Context, event UsageEvent)

// StreamableRepository is a repository that supports usage-event streaming.
type StreamableRepository interface {
	RegisterUsageStream(handler StreamHandler) int
	RemoveUsageStream(id int)
}

// Adapter fans usage events from a StreamableRepository out to registered Plugins.
type Adapter struct {
	pluginsMu  sync.Mutex // guards plugins, which may be registered while events are being sent
	plugins    []Plugin
	repository StreamableRepository
	handlerID  int
	logger     *logrus.Logger
}

// NewAdapter creates an Adapter and immediately subscribes to the repository's usage stream.
func NewAdapter(repository StreamableRepository, logger *logrus.Logger) *Adapter {
	a := &Adapter{
		repository: repository,
		logger:     logger,
	}
	a.handlerID = repository.RegisterUsageStream(func(ctx context.Context, event UsageEvent) {
		a.dispatch(ctx, event)
	})
	return a
}

// RegisterPlugin adds a plugin to receive usage events.
func (a *Adapter) RegisterPlugin(plugin Plugin) {
	if _, scoped := plugin.(RequestScopedPlugin); scoped && plugin.CanSendAsync() && a.logger != nil {
		a.logger.Warn("A request-scoped usage plugin sends asynchronously, so it may miss features read just before an outgoing call")
	}
	a.pluginsMu.Lock()
	defer a.pluginsMu.Unlock()
	a.plugins = append(a.plugins, plugin)
}

// currentPlugins returns a copy of the registered plugins:
func (a *Adapter) currentPlugins() []Plugin {
	a.pluginsMu.Lock()
	defer a.pluginsMu.Unlock()
	return append([]Plugin(nil), a.plugins...)
}

// StartRequest gives each RequestScopedPlugin the chance to add its state for a new request to ctx, and returns
// the resulting context. A plugin that panics or returns nil leaves the context as it was.
func (a *Adapter) StartRequest(ctx context.Context) context.Context {
	for _, p := range a.currentPlugins() {
		scoped, ok := p.(RequestScopedPlugin)
		if !ok {
			continue
		}
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

// Close unregisters the adapter from the repository's usage stream and closes all plugins.
func (a *Adapter) Close() {
	a.repository.RemoveUsageStream(a.handlerID)
	for _, p := range a.currentPlugins() {
		p.Close()
	}
}

func (a *Adapter) dispatch(ctx context.Context, event UsageEvent) context.Context {
	for _, p := range a.currentPlugins() {
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
				ctx = p.Send(ctx, event)
			}()
		}
	}
	return ctx
}
