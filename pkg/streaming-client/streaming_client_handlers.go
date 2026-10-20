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

	if subscriptionErr, stop := stopStatus(err); stop {
		c.stopForStatus(subscriptionErr, true)
		return closeNow
	}

	c.logger.WithError(err).Debug("Error from SSE stream, reconnecting")
	return eventsource.StreamErrorHandlerResult{}
}

// stopStatus reports whether err is an HTTP status that means the client must stop for good: 236 (the server
// told us to stop), or 400/401/403/404 (the same statuses the polling client stops on - the key is invalid or
// does not exist, or the server address is wrong, so retrying will not help):
func stopStatus(err error) (eventsource.SubscriptionError, bool) {
	var subscriptionErr eventsource.SubscriptionError
	if !goerrors.As(err, &subscriptionErr) {
		return subscriptionErr, false
	}
	switch subscriptionErr.Code {
	case statusStale, http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return subscriptionErr, true
	}
	return subscriptionErr, false
}

// stopForStatus stops the client for good after a stopStatus response. A rejected request is passed to the fatal
// error handler if reportToHandler is set, and otherwise logged:
func (c *StreamingClient) stopForStatus(subscriptionErr eventsource.SubscriptionError, reportToHandler bool) {
	c.Close()

	if subscriptionErr.Code == statusStale {
		c.logger.Warn("The FeatureHub server has requested that we stop (236)! No further updates will be received - existing data will continue to be served")
		return
	}

	const message = "The FeatureHub server rejected the SDK key or server address, stopping"
	details := map[string]interface{}{"status": subscriptionErr.Code, "message": subscriptionErr.Message}
	if reportToHandler {
		c.reportFatalError(&errors.ErrFromAPI{}, message, details)
	} else {
		c.logger.WithError(&errors.ErrFromAPI{}).WithFields(details).Error(message)
	}
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
			c.stopForEvent(event, "Failure from FeatureHub server, stopping")

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
	c.stopForEvent(event, "Error from API client, stopping")
}

// stopForEvent stops the client for good after an error or failure event from the server - it disconnects and
// never reconnects - and passes the event to the fatal error handler:
func (c *StreamingClient) stopForEvent(event eventsource.Event, message string) {
	c.Close()

	details := map[string]interface{}{
		"event":   event.Event(),
		"message": event.Data(),
	}
	c.reportFatalError(&errors.ErrFromAPI{}, message, details)
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
