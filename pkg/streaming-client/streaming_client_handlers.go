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
			feature := &models.FeatureState{}
			if err := json.Unmarshal([]byte(event.Data()), feature); err != nil {
				c.logger.WithError(err).WithField("event", "feature").Error("Error unmarshaling SSE payload")
				continue
			}

			c.repository.ProcessDeleteFeature(feature)

		// Failures (from the FeatureHub server):
		case models.FHFailure:
			details := map[string]interface{}{
				"event":   event.Event(),
				"message": event.Data(),
			}
			c.fatalErrorHandler(&errors.ErrFromAPI{}, "Failure from FeatureHub server", details)

		// One specific feature (replaces the previous version):
		case models.FHFeature:
			feature := &models.FeatureState{}
			if err := json.Unmarshal([]byte(event.Data()), feature); err != nil {
				c.logger.WithError(err).WithField("event", "feature").Error("Error unmarshaling SSE payload")
				continue
			}

			c.repository.ProcessFeature(feature)

		// An entire feature set (replaces what we currently have):
		case models.FHFeatures:
			var features []*models.FeatureState
			if err := json.Unmarshal([]byte(event.Data()), &features); err != nil {
				c.logger.WithError(err).WithField("event", "features").Error("Error unmarshaling SSE payload")
				return
			}

			c.repository.ProcessFeatures(features)

		// Everything else just gets logged:
		default:
			c.logger.WithField("event", event.Event()).Trace("Received SSE event")
		}
	}
}

func (c *StreamingClient) handleSSEError(event eventsource.Event) {
	// If we're already running then just log an error, otherwise panic:
	if c.repository.IsReady() {
		c.logger.WithError(&errors.ErrFromAPI{}).WithField("event", event.Event()).WithField("message", event.Data()).Error("Error from API client")
	} else {
		// Use the fatalErrorFunc for this one:
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
