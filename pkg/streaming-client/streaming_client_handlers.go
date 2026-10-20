package streamingclient

import (
	"encoding/json"
	goerrors "errors"
	"net/http"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/launchdarkly/eventsource"
)

// statusStale is the HTTP status the server uses to tell the client to stop (the same as edge.stale):
const statusStale = 236

// handleStreamError is called by the eventsource library for connection errors (including read timeouts and
// non-200 responses to a reconnect). The stream reconnects (sending the Last-Event-ID, after a backoff delay)
// unless the client has been closed, the server has told us to stop (236), or the request was rejected
// (400/401/403/404 - a bad SDK key, or the client pointed at the wrong server):
func (c *StreamingClient) handleStreamError(err error) eventsource.StreamErrorHandlerResult {
	closeNow := eventsource.StreamErrorHandlerResult{CloseNow: true}

	if c.stopped.Load() {
		return closeNow
	}

	var subscriptionErr eventsource.SubscriptionError
	if goerrors.As(err, &subscriptionErr) {
		switch subscriptionErr.Code {
		case statusStale:
			c.logger.Warn("The FeatureHub server has requested that we stop (236)! No further updates will be received - existing data will continue to be served")
			c.Close()
			return closeNow

		case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
			// the same statuses the polling client treats as fatal: the key is invalid or does not exist, or the
			// server address is wrong. Retrying will not help, so stop for good
			c.Close()
			details := map[string]interface{}{"status": subscriptionErr.Code, "message": subscriptionErr.Message}
			c.reportFatalError(&errors.ErrFromAPI{}, "The FeatureHub server rejected the SDK key or server address, stopping", details)
			return closeNow
		}
	}

	c.logger.WithError(err).Debug("Error from SSE stream, reconnecting")
	return eventsource.StreamErrorHandlerResult{}
}

// handleEvents deals with incoming server-side events:
func (c *StreamingClient) handleEvents() {

	// Run until the stream is closed (blocks on receiving events from the client channel):
	for event := range c.events {

		// We may have been shut down by some external process:
		if !c.isRunning.Load() {
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

			feature.EnvironmentID = c.config.EnvironmentID()

			c.repository.ProcessDeleteFeature(feature)

		// Failures (from the FeatureHub server):
		case models.FHFailure:
			details := map[string]interface{}{
				"event":   event.Event(),
				"message": event.Data(),
			}
			c.reportFatalError(&errors.ErrFromAPI{}, "Failure from FeatureHub server", details)

		// One specific feature (replaces the previous version):
		case models.FHFeature:
			feature := &models.FeatureState{}
			if err := json.Unmarshal([]byte(event.Data()), feature); err != nil {
				c.logger.WithError(err).WithField("event", "feature").Error("Error unmarshaling SSE payload")
				continue
			}

			feature.EnvironmentID = c.config.EnvironmentID()

			c.repository.ProcessFeature(feature)

		// An entire feature set (replaces what we currently have):
		case models.FHFeatures:
			var features []*models.FeatureState
			if err := json.Unmarshal([]byte(event.Data()), &features); err != nil {
				c.logger.WithError(err).WithField("event", "features").Error("Error unmarshaling SSE payload")
				continue
			}

			// set their EnvironmentID
			for _, feature := range features {
				feature.EnvironmentID = c.config.EnvironmentID()
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
		// Use the fatal error handler for this one:
		details := map[string]interface{}{
			"event":   event.Event(),
			"message": event.Data(),
		}
		c.reportFatalError(&errors.ErrFromAPI{}, "Error from API client", details)
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

		// Close the client for good - it will not reconnect, and cannot be connected again:
		c.logger.Warn("The FeatureHub server has requested that we close our connection (edge.stale)! No further updates will be received - existing data will continue to be served")
		c.Close()
	}
}
