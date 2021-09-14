package handler

import (
	"fmt"
	"net/http"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
)

// Random returns a greeting according to a random percentage feature:
// - Test this by setting a boolean feature called "random" and adding a percentage strategy
// - Try the URL with a number of different names - you should see your percentage split (with consistent results for each name)
func (h *Handler) Random(w http.ResponseWriter, r *http.Request) {

	// Get the "name" parameter (from the query string):
	name := r.FormValue("name")

	// Log an analytics event:
	tags := map[string]string{"name": name}
	h.fhClient.LogAnalyticsEvent("Random", tags)

	// Get a new context for this session (loaded with a parameter from the request):
	sessionContext := h.fhClient.WithContext(&models.Context{Userkey: name})

	// Look up a boolean feature called "random":
	sayGoodbye, err := sessionContext.GetBoolean("random")
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
