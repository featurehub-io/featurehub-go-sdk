package errors

import "fmt"

// ErrInvalidNotifierCallback is returned when the user attempts to delete a non-existent notifier:
type ErrInvalidNotifierCallback struct {
	message string
}

// NewErrInvalidNotifierCallback returns a ErrInvalidNotifierCallback with a user-provided message:
func NewErrInvalidNotifierCallback(message string) *ErrInvalidNotifierCallback {
	return &ErrInvalidNotifierCallback{message: message}
}

func (e *ErrInvalidNotifierCallback) Error() string {
	if e.message != "" {
		return fmt.Sprintf("Notifier not found: %s", e.message)
	}
	return "Notifier not found"
}
