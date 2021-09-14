package handler

import (
	"fmt"
	"net/http"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
)

// Mapped returns a greeting according to a pre-defined list of names:
// - Add a rollout strategy to the goodbye feature to split on userkey, and populate this with some names
func (h *Handler) Mapped(w http.ResponseWriter, r *http.Request) {

	// Get the "name" parameter (from the query string):
	name := r.FormValue("name")

	// Log an analytics event:
	tags := map[string]string{"name": name}
	h.fhClient.LogAnalyticsEvent("Mapped", tags)

	// Get a new context for this session (loaded with a parameter from the request):
	sessionContext := h.fhClient.WithContext(&models.Context{Userkey: name})

	// Look up a boolean feature called "goodbye":
	sayGoodbye, err := sessionContext.GetBoolean("goodbye")
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
