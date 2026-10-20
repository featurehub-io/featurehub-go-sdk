package interfaces

type EdgeClient interface {
	// Initial connect process
	Connect()
	// Poll - triggers the client to check if it needs to poll again
	Poll() error
	// ContextChange - support for Server Side Evaluation, changes the header to match
	ContextChange(header string)
	// Close - stops and shuts down the client
	Close()
	// Stopped - true once the client has stopped for good (closed, told to stop by the server, or its requests
	// were rejected). A stopped client never connects again.
	Stopped() bool
	// StopReason - why the client stopped, or nil while it has not
	StopReason() error
}
