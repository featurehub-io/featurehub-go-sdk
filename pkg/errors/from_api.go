package errors

import "fmt"

// ErrFromAPI is returned when we have trouble making API requests:
type ErrFromAPI struct {
	message string
	cause   error // one of the stop reasons, for errors.Is, if made with NewStopReason
}

// NewErrFromAPI returns a ErrFromAPI with a user-provided message:
func NewErrFromAPI(message string) *ErrFromAPI {
	return &ErrFromAPI{message: message}
}

func (e *ErrFromAPI) Error() string {
	if e.message != "" {
		return fmt.Sprintf("API error: %s", e.message)
	}
	return "API error"
}

// Unwrap returns the stop reason this error was made with (see NewStopReason), or nil:
func (e *ErrFromAPI) Unwrap() error {
	return e.cause
}
