package errors

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- ErrBadConfig ---

func TestErrBadConfigWithMessage(t *testing.T) {
	err := NewErrBadConfig("missing SDK key")
	assert.EqualError(t, err, "Invalid config: missing SDK key")
}

func TestErrBadConfigWithoutMessage(t *testing.T) {
	err := &ErrBadConfig{}
	assert.EqualError(t, err, "Invalid config")
}

func TestErrBadConfigImplementsError(t *testing.T) {
	var err error = NewErrBadConfig("x")
	assert.NotNil(t, err)
}

func TestErrBadConfigErrorsAs(t *testing.T) {
	err := NewErrBadConfig("oops")
	var target *ErrBadConfig
	assert.True(t, stderrors.As(err, &target))
	assert.Equal(t, "oops", target.message)
}

// --- ErrFeatureNotFound ---

func TestErrFeatureNotFoundWithMessage(t *testing.T) {
	err := NewErrFeatureNotFound("my-flag")
	assert.EqualError(t, err, "Feature not found: my-flag")
}

func TestErrFeatureNotFoundWithoutMessage(t *testing.T) {
	err := &ErrFeatureNotFound{}
	assert.EqualError(t, err, "Feature not found")
}

func TestErrFeatureNotFoundImplementsError(t *testing.T) {
	var err error = NewErrFeatureNotFound("x")
	assert.NotNil(t, err)
}

func TestErrFeatureNotFoundErrorsAs(t *testing.T) {
	err := NewErrFeatureNotFound("flag-x")
	var target *ErrFeatureNotFound
	assert.True(t, stderrors.As(err, &target))
}

// --- ErrFromAPI ---

func TestErrFromAPIWithMessage(t *testing.T) {
	err := NewErrFromAPI("timeout")
	assert.EqualError(t, err, "API error: timeout")
}

func TestErrFromAPIWithoutMessage(t *testing.T) {
	err := &ErrFromAPI{}
	assert.EqualError(t, err, "API error")
}

func TestErrFromAPIImplementsError(t *testing.T) {
	var err error = NewErrFromAPI("x")
	assert.NotNil(t, err)
}

func TestErrFromAPIErrorsAs(t *testing.T) {
	err := NewErrFromAPI("bad gateway")
	var target *ErrFromAPI
	assert.True(t, stderrors.As(err, &target))
}

// --- ErrInvalidType ---

func TestErrInvalidTypeWithMessage(t *testing.T) {
	err := NewErrInvalidType("STRING")
	assert.EqualError(t, err, "Invalid type: STRING")
}

func TestErrInvalidTypeWithoutMessage(t *testing.T) {
	err := &ErrInvalidType{}
	assert.EqualError(t, err, "Invalid type")
}

func TestErrInvalidTypeImplementsError(t *testing.T) {
	var err error = NewErrInvalidType("x")
	assert.NotNil(t, err)
}

func TestErrInvalidTypeErrorsAs(t *testing.T) {
	err := NewErrInvalidType("BOOLEAN")
	var target *ErrInvalidType
	assert.True(t, stderrors.As(err, &target))
}

// --- ErrNotifierNotFound ---

func TestErrNotifierNotFoundWithMessage(t *testing.T) {
	err := NewErrNotifierNotFound("my-flag/uuid-123")
	assert.EqualError(t, err, "Notifier not found: my-flag/uuid-123")
}

func TestErrNotifierNotFoundWithoutMessage(t *testing.T) {
	err := &ErrNotifierNotFound{}
	assert.EqualError(t, err, "Notifier not found")
}

func TestErrNotifierNotFoundImplementsError(t *testing.T) {
	var err error = NewErrNotifierNotFound("x")
	assert.NotNil(t, err)
}

func TestErrNotifierNotFoundErrorsAs(t *testing.T) {
	err := NewErrNotifierNotFound("flag/uuid")
	var target *ErrNotifierNotFound
	assert.True(t, stderrors.As(err, &target))
}

// --- ErrInvalidNotifierCallback ---

func TestErrInvalidNotifierCallbackWithMessage(t *testing.T) {
	err := NewErrInvalidNotifierCallback("callback is nil")
	assert.NotEmpty(t, err.Error())
	assert.Contains(t, err.Error(), "callback is nil")
}

func TestErrInvalidNotifierCallbackWithoutMessage(t *testing.T) {
	err := &ErrInvalidNotifierCallback{}
	assert.NotEmpty(t, err.Error())
}

func TestErrInvalidNotifierCallbackImplementsError(t *testing.T) {
	var err error = NewErrInvalidNotifierCallback("x")
	assert.NotNil(t, err)
}

func TestErrInvalidNotifierCallbackErrorsAs(t *testing.T) {
	err := NewErrInvalidNotifierCallback("nil func")
	var target *ErrInvalidNotifierCallback
	assert.True(t, stderrors.As(err, &target))
}
