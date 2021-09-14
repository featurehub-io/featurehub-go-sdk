package handler

import (
	streamingclient "github.com/featurehub-io/featurehub-go-sdk/pkg/streaming-client"
	"github.com/sirupsen/logrus"
)

// Handler provides basic mocks of the Turn webhook API:
type Handler struct {
	logger   *logrus.Logger
	fhClient *streamingclient.ClientWithContext
}

// New returns a new Handler:
func New(logger *logrus.Logger, fhClient *streamingclient.ClientWithContext) *Handler {
	return &Handler{
		logger:   logger,
		fhClient: fhClient,
	}
}
