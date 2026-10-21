package errors

import stderrors "errors"

// Reasons an edge client stops for good. The client's StopReason() (and so the Cause of ErrEdgeStopped) matches
// one of these with errors.Is, so callers can tell a client they closed from one the server stopped or rejected:
var (
	// ErrClientClosed - the client was closed with Close (or by the config)
	ErrClientClosed = stderrors.New("the client was closed")
	// ErrServerRequestedStop - the server told the client to stop (a 236 status, or edge.stale)
	ErrServerRequestedStop = stderrors.New("the server requested a stop")
	// ErrKeyRejected - the server rejected the SDK key or server address (a 400, 401, 403 or 404 status)
	ErrKeyRejected = stderrors.New("the server rejected the SDK key or server address")
	// ErrServerError - the server sent an SSE error or failure event
	ErrServerError = stderrors.New("the server sent an error")
	// ErrNoEnvironments - the server returned no environments for the SDK keys when polling
	ErrNoEnvironments = stderrors.New("the server returned no environments")
)

// NewStopReason returns an ErrFromAPI for one of the server stop reasons above, with details (e.g. the status).
// errors.Is(err, reason) matches it:
func NewStopReason(reason error, details string) *ErrFromAPI {
	message := reason.Error()
	if details != "" {
		message += " (" + details + ")"
	}
	return &ErrFromAPI{message: message, cause: reason}
}
