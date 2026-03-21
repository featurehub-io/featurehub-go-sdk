package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- EnvOrDefaultStr ---

func TestEnvOrDefaultStrReturnsDefaultWhenEnvAbsent(t *testing.T) {
	t.Setenv("TEST_STR_ABSENT", "")

	result := EnvOrDefaultStr("TEST_STR_ABSENT", "fallback")

	assert.Equal(t, "fallback", result)
}

func TestEnvOrDefaultStrReturnsDefaultWhenEnvUnset(t *testing.T) {
	result := EnvOrDefaultStr("TEST_STR_DEFINITELY_NOT_SET_XYZ", "fallback")

	assert.Equal(t, "fallback", result)
}

func TestEnvOrDefaultStrReturnsEnvValue(t *testing.T) {
	t.Setenv("TEST_STR", "hello")

	result := EnvOrDefaultStr("TEST_STR", "fallback")

	assert.Equal(t, "hello", result)
}

func TestEnvOrDefaultStrDefaultCanBeEmpty(t *testing.T) {
	result := EnvOrDefaultStr("TEST_STR_DEFINITELY_NOT_SET_XYZ", "")

	assert.Equal(t, "", result)
}

// --- EnvOrDefaultDuration ---

func TestEnvOrDefaultDurationReturnsDefaultWhenEnvAbsent(t *testing.T) {
	t.Setenv("TEST_DURATION_ABSENT", "")

	result := EnvOrDefaultDuration("TEST_DURATION_ABSENT", 5*time.Second)

	assert.Equal(t, 5*time.Second, result)
}

func TestEnvOrDefaultDurationReturnsDefaultWhenEnvUnset(t *testing.T) {
	// Variable is never set in this process, so Getenv returns "".
	result := EnvOrDefaultDuration("TEST_DURATION_DEFINITELY_NOT_SET_XYZ", 10*time.Minute)

	assert.Equal(t, 10*time.Minute, result)
}

func TestEnvOrDefaultDurationReturnsParsedValue(t *testing.T) {
	t.Setenv("TEST_DURATION", "30s")

	result := EnvOrDefaultDuration("TEST_DURATION", 5*time.Second)

	assert.Equal(t, 30*time.Second, result)
}

func TestEnvOrDefaultDurationParsesMinutes(t *testing.T) {
	t.Setenv("TEST_DURATION_MIN", "2m")

	result := EnvOrDefaultDuration("TEST_DURATION_MIN", 5*time.Second)

	assert.Equal(t, 2*time.Minute, result)
}

func TestEnvOrDefaultDurationParsesMilliseconds(t *testing.T) {
	t.Setenv("TEST_DURATION_MS", "500ms")

	result := EnvOrDefaultDuration("TEST_DURATION_MS", 5*time.Second)

	assert.Equal(t, 500*time.Millisecond, result)
}

func TestEnvOrDefaultDurationReturnsDefaultOnInvalidValue(t *testing.T) {
	t.Setenv("TEST_DURATION_BAD", "not-a-duration")

	result := EnvOrDefaultDuration("TEST_DURATION_BAD", 3*time.Minute)

	assert.Equal(t, 3*time.Minute, result)
}

func TestEnvOrDefaultDurationReturnsDefaultOnNumericOnlyValue(t *testing.T) {
	// time.ParseDuration requires a unit suffix; bare numbers are invalid.
	t.Setenv("TEST_DURATION_NUM", "30")

	result := EnvOrDefaultDuration("TEST_DURATION_NUM", 1*time.Hour)

	assert.Equal(t, 1*time.Hour, result)
}
