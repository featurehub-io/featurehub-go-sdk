package errors

// ErrConfigClosed is returned when an operation is attempted on a Config that has been closed.
type ErrConfigClosed struct{}

func NewErrConfigClosed() *ErrConfigClosed {
	return &ErrConfigClosed{}
}

func (e *ErrConfigClosed) Error() string {
	return "config is closed"
}
