package interfaces

type EdgeClient interface {
	Connect()
	ContextChange(header string) error
}
