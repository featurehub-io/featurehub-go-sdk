package errors

// ErrEdgeStopped is returned when connecting with an edge client that has stopped for good: it was closed, the
// server told it to stop (edge.stale or a 236 status), or the server rejected the SDK key or server address.
type ErrEdgeStopped struct{}

func NewErrEdgeStopped() *ErrEdgeStopped {
	return &ErrEdgeStopped{}
}

func (e *ErrEdgeStopped) Error() string {
	return "the edge client has stopped and will not reconnect (closed, told to stop by the server, or the SDK key or server address was rejected)"
}
