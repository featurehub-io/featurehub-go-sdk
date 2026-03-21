package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertValueBooleanTrue(t *testing.T) {
	v, err := ConvertValue(TypeBoolean, true)
	require.NoError(t, err)
	assert.Equal(t, true, v)
	v, err = ConvertValue(TypeBoolean, "true")
	require.NoError(t, err)
	assert.Equal(t, true, v)
	v, err = ConvertValue(TypeBoolean, "yes")
	require.NoError(t, err)
	assert.Equal(t, true, v)
	v, err = ConvertValue(TypeBoolean, "on")
	require.NoError(t, err)
	assert.Equal(t, true, v)
}

func TestConvertValueBooleanStringFalse(t *testing.T) {
	v, err := ConvertValue(TypeBoolean, "false")
	require.NoError(t, err)
	assert.Equal(t, false, v)
	v, err = ConvertValue(TypeBoolean, "off")
	require.NoError(t, err)
	assert.Equal(t, false, v)
	v, err = ConvertValue(TypeBoolean, "no")
	require.NoError(t, err)
	assert.Equal(t, false, v)
	v, err = ConvertValue(TypeBoolean, "n")
	require.NoError(t, err)
	assert.Equal(t, false, v)
}

func TestConvertValueNumberInt(t *testing.T) {
	v, err := ConvertValue(TypeNumber, 5)
	require.NoError(t, err)
	assert.Equal(t, float64(5), v)
}

func TestConvertValueNumberFloat(t *testing.T) {
	v, err := ConvertValue(TypeNumber, float64(1.5))
	require.NoError(t, err)
	assert.Equal(t, float64(1.5), v)
}

func TestConvertValueNumberString(t *testing.T) {
	v, err := ConvertValue(TypeNumber, "2.5")
	require.NoError(t, err)
	assert.Equal(t, float64(2.5), v)
}

func TestConvertValueStringPassthrough(t *testing.T) {
	v, err := ConvertValue(TypeString, "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", v)
}

func TestConvertValueJSONPassthrough(t *testing.T) {
	v, err := ConvertValue(TypeJSON, `{"x":1}`)
	require.NoError(t, err)
	assert.Equal(t, `{"x":1}`, v)
}

func TestConvertValueUnknownTypeReturnsError(t *testing.T) {
	_, err := ConvertValue(FeatureValueType("NONSENSE"), "value")
	assert.Error(t, err)
}

// --- ConvertToString ---

func TestConvertToStringBooleanTrue(t *testing.T) {
	s, err := ConvertToString(TypeBoolean, true)
	require.NoError(t, err)
	assert.Equal(t, "t", s)
}

func TestConvertToStringBooleanFalse(t *testing.T) {
	s, err := ConvertToString(TypeBoolean, false)
	require.NoError(t, err)
	assert.Equal(t, "f", s)
}

func TestConvertToStringBooleanFromString(t *testing.T) {
	s, err := ConvertToString(TypeBoolean, "on")
	require.NoError(t, err)
	assert.Equal(t, "t", s)

	s, err = ConvertToString(TypeBoolean, "off")
	require.NoError(t, err)
	assert.Equal(t, "f", s)
}

func TestConvertToStringNumber(t *testing.T) {
	s, err := ConvertToString(TypeNumber, float64(42))
	require.NoError(t, err)
	assert.Equal(t, "42", s)
}

func TestConvertToStringNumberFloat(t *testing.T) {
	s, err := ConvertToString(TypeNumber, 3.14)
	require.NoError(t, err)
	assert.Equal(t, "3.14", s)
}

func TestConvertToStringString(t *testing.T) {
	s, err := ConvertToString(TypeString, "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", s)
}

func TestConvertToStringJSON(t *testing.T) {
	s, err := ConvertToString(TypeJSON, `{"a":1}`)
	require.NoError(t, err)
	assert.Equal(t, `{"a":1}`, s)
}

func TestConvertToStringUnknownTypeReturnsError(t *testing.T) {
	_, err := ConvertToString(FeatureValueType("NONSENSE"), "value")
	assert.Error(t, err)
}

func TestConvertToStringInvalidValueReturnsError(t *testing.T) {
	_, err := ConvertToString(TypeBoolean, "notabool")
	assert.Error(t, err)
}
