package usage

import (
	"github.com/sirupsen/logrus"
)

// StreamHandler is called whenever a usage event is emitted by the repository.
type StreamHandler func(event UsageEvent)

// StreamableRepository is a repository that supports usage-event streaming.
type StreamableRepository interface {
	RegisterUsageStream(handler StreamHandler) int
	RemoveUsageStream(id int)
}

// Adapter fans usage events from a StreamableRepository out to registered Plugins.
type Adapter struct {
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
	a.handlerID = repository.RegisterUsageStream(a.dispatch)
	return a
}

// RegisterPlugin adds a plugin to receive usage events.
func (a *Adapter) RegisterPlugin(plugin Plugin) {
	a.plugins = append(a.plugins, plugin)
}

// Close unregisters the adapter from the repository's usage stream.
func (a *Adapter) Close() {
	a.repository.RemoveUsageStream(a.handlerID)
}

func (a *Adapter) dispatch(event UsageEvent) {
	for _, p := range a.plugins {
		go func(p Plugin) {
			defer func() {
				if r := recover(); r != nil {
					a.logger.WithField("panic", r).Error("usage plugin panicked during Send")
				}
			}()
			p.Send(event)
		}(p)
	}
}
