package streamingclient

import (
	"encoding/json"

	"github.com/donovanhide/eventsource"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
)

// handleErrors deals with incoming server-side errors:
func (c *StreamingClient) handleErrors() {

	// Run forever (blocks on receiving events from the client channel):
	for {
		event := <-c.apiClient.Errors

		// We may have been shut down by some external process:
		if !c.isRunning {
			c.logger.Info("No longer handling SSE errors")
			break
		}

		c.logger.WithError(event).Trace("Error from API client")
	}
}

// handleEvents deals with incoming server-side events:
func (c *StreamingClient) handleEvents() {

	// Run forever (blocks on receiving events from the client channel):
	for {
		event := <-c.apiClient.Events

		// We may have been shut down by some external process:
		if !c.isRunning {
			c.logger.Info("No longer handling SSE events")
			break
		}

		// Handle the different types of events that can be received on this channel:
		switch models.Event(event.Event()) {

		// Control messages:
		case models.SSEAck, models.SSEBye:
			c.logger.WithField("event", event.Event()).Trace("Received SSE control event")

		// Errors (from the SSE client):
		case models.SSEError:
			c.handleSSEError(event)

		// FeatureHub configuration events:
		case models.FHConfig:
			c.handleFHConfigEvent(event)

		// Delete a feature from our list:
		case models.FHDeleteFeature:
			c.handleFHDeleteFeature(event)

		// Failures (from the FeatureHub server):
		case models.FHFailure:
			details := map[string]interface{}{
				"event":   event.Event(),
				"message": event.Data(),
			}
			c.fatalErrorHandler(&errors.ErrFromAPI{}, "Failure from FeatureHub server", details)

		// One specific feature (replaces the previous version):
		case models.FHFeature:
			c.handleFHFeature(event)

		// An entire feature set (replaces what we currently have):
		case models.FHFeatures:
			c.handleFHFeatures(event)

		// Everything else just gets logged:
		default:
			c.logger.WithField("event", event.Event()).Trace("Received SSE event")
		}
	}
}

func (c *StreamingClient) handleSSEError(event eventsource.Event) {
	// If we're already running then just log an error, otherwise panic:
	if c.hasData {
		c.logger.WithError(&errors.ErrFromAPI{}).WithField("event", event.Event()).WithField("message", event.Data()).Error("Error from API client")
	} else {
		// Use the fatailErrorFunc for this one:
		details := map[string]interface{}{
			"event":   event.Event(),
			"message": event.Data(),
		}
		c.fatalErrorFunc(&errors.ErrFromAPI{}, "Error from API client", details)
	}
}

func (c *StreamingClient) handleFHConfigEvent(event eventsource.Event) {

	// Unmarshal the event payload:
	configEvent := new(models.ConfigEvent)
	if err := json.Unmarshal([]byte(event.Data()), configEvent); err != nil {
		c.logger.WithError(err).WithField("event", "config").Error("Error unmarshaling SSE payload")
	}

	// Handle "edge.stale" config:
	if configEvent.EdgeStale {

		// Close the SSE client connection:
		c.logger.Warn("The FeatureHub server has requested that we close our connection (edge.stale)! No further updates will be received - existing data will continue to be served")
		c.isRunning = false
		c.apiClient.Close()
	}
}

func (c *StreamingClient) handleFHDeleteFeature(event eventsource.Event) {

	// Unmarshal the event payload:
	feature := &models.FeatureState{}
	if err := json.Unmarshal([]byte(event.Data()), feature); err != nil {
		c.logger.WithError(err).WithField("event", "feature").Error("Error unmarshaling SSE payload")
	}

	// Delete the feature:
	c.featuresMutex.Lock()
	defer c.featuresMutex.Unlock()
	delete(c.features, feature.Key)

	c.logger.WithField("key", feature.Key).Debug("Deleted a feature")
}

func (c *StreamingClient) handleFHFeature(event eventsource.Event) {

	// Unmarshal the event payload:
	feature := &models.FeatureState{}
	if err := json.Unmarshal([]byte(event.Data()), feature); err != nil {
		c.logger.WithError(err).WithField("event", "feature").Error("Error unmarshaling SSE payload")
	}

	// Take the new feature (or ignore if the version is not newer):
	c.featuresMutex.Lock()
	defer c.featuresMutex.Unlock()
	if currentFeature, ok := c.features[feature.Key]; ok {
		if feature.Version <= currentFeature.Version {
			c.logger.WithField("key", feature.Key).Debug("Received an old feature from server")
			return
		}
	}

	// Otherwise this is a new feature, so we just take it:
	c.logger.WithField("key", feature.Key).Debug("Received a new feature from server")
	c.features[feature.Key] = feature
	c.notify(feature)
	c.isReady()
}

func (c *StreamingClient) handleFHFeatures(event eventsource.Event) {

	// Unmarshal the event payload:
	features := []*models.FeatureState{}
	if err := json.Unmarshal([]byte(event.Data()), &features); err != nil {
		c.logger.WithError(err).WithField("event", "features").Error("Error unmarshaling SSE payload")
	}

	// Create a new map of features:
	newFeatures := make(map[string]*models.FeatureState)
	for _, newFeature := range features {
		newFeatures[newFeature.Key] = newFeature
	}

	// Take the new features:
	c.featuresMutex.Lock()
	oldFeatures := c.features
	c.features = newFeatures
	c.isReady()
	c.featuresMutex.Unlock()

	// Compare versions to see who should be notified:
	for _, newFeature := range newFeatures {
		if oldFeature, ok := oldFeatures[newFeature.Key]; ok {
			if newFeature.Version <= oldFeature.Version {
				continue
			}
		}
		c.notify(newFeature)
	}

	c.logger.Debugf("Received %d features from server", len(features))
}
