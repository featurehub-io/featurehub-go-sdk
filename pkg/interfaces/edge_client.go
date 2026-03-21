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
}
