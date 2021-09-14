package handler

import (
	"fmt"
	"net/http"
)

// Static returns a greeting according to a static feature:
// - Test this by setting a boolean feature called "goodbye" either on or off
func (h *Handler) Static(w http.ResponseWriter, r *http.Request) {

	// Get the "name" parameter (from the query string):
	name := r.FormValue("name")

	// Log an analytics event:
	tags := map[string]string{"name": name}
	h.fhClient.LogAnalyticsEvent("Static", tags)

	// Look up a boolean feature called "goodbye":
	sayGoodbye, err := h.fhClient.GetBoolean("goodbye")
	if err != nil {
		h.logger.WithError(err).Warn("Error retrieving feature")
	}

	// Respond:
	if sayGoodbye {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(fmt.Sprintf("Goodbye, %s", name)))
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Hello, %s", name)))
	}
}
