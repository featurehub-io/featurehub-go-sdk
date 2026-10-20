package errors

// ErrEdgeStopped is returned when connecting with an edge client that has stopped for good: it was closed, the
// server told it to stop (edge.stale or a 236 status), the server rejected the SDK key or server address, or the
// server sent an error. Cause (also available with errors.Unwrap) says which.
type ErrEdgeStopped struct {
	Cause error
}

func NewErrEdgeStopped(cause error) *ErrEdgeStopped {
	return &ErrEdgeStopped{Cause: cause}
}

func (e *ErrEdgeStopped) Error() string {
	if e.Cause == nil {
		return "the edge client has stopped and will not reconnect"
	}
	return "the edge client has stopped and will not reconnect: " + e.Cause.Error()
}

func (e *ErrEdgeStopped) Unwrap() error {
	return e.Cause
}
