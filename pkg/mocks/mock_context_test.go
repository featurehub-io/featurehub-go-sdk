package mocks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func features() map[string]interface{} {
	return map[string]interface{}{
		"boolFlag": true,
		"numFlag":  float64(42),
		"strFlag":  "hello",
		"jsonFlag": `{"x":1}`,
	}
}

// --- GetBoolean ---

func TestMockGetBooleanFound(t *testing.T) {
	m := NewMockContext(features())
	v, err := m.GetBoolean(context.TODO(), "boolFlag")
	require.NoError(t, err)
	assert.True(t, v)
}

func TestMockGetBooleanNotFound(t *testing.T) {
	m := NewMockContext(features())
	_, err := m.GetBoolean(context.TODO(), "missing")
	assert.Error(t, err)
}

func TestMockGetBooleanWrongType(t *testing.T) {
	m := NewMockContext(features())
	_, err := m.GetBoolean(context.TODO(), "strFlag")
	assert.Error(t, err)
}

// --- GetNumber ---

func TestMockGetNumberFound(t *testing.T) {
	m := NewMockContext(features())
	v, err := m.GetNumber(context.TODO(), "numFlag")
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, float64(42), *v)
}

func TestMockGetNumberNotFound(t *testing.T) {
	m := NewMockContext(features())
	_, err := m.GetNumber(context.TODO(), "missing")
	assert.Error(t, err)
}

func TestMockGetNumberWrongType(t *testing.T) {
	m := NewMockContext(features())
	_, err := m.GetNumber(context.TODO(), "boolFlag")
	assert.Error(t, err)
}

// --- GetString ---

func TestMockGetStringFound(t *testing.T) {
	m := NewMockContext(features())
	v, err := m.GetString(context.TODO(), "strFlag")
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, "hello", *v)
}

func TestMockGetStringNotFound(t *testing.T) {
	m := NewMockContext(features())
	_, err := m.GetString(context.TODO(), "missing")
	assert.Error(t, err)
}

// --- GetRawJSON ---

func TestMockGetRawJSONFound(t *testing.T) {
	m := NewMockContext(features())
	v, err := m.GetRawJSON(context.TODO(), "jsonFlag")
	require.NoError(t, err)
	require.NotNil(t, v)
	assert.Equal(t, `{"x":1}`, *v)
}

func TestMockGetRawJSONNotFound(t *testing.T) {
	m := NewMockContext(features())
	_, err := m.GetRawJSON(context.TODO(), "missing")
	assert.Error(t, err)
}

// --- Convenience methods ---

func TestMockBooleanFound(t *testing.T) {
	m := NewMockContext(features())
	assert.True(t, m.Boolean(context.TODO(), "boolFlag", false))
}

func TestMockBooleanDefault(t *testing.T) {
	m := NewMockContext(features())
	assert.True(t, m.Boolean(context.TODO(), "missing", true))
}

func TestMockNumberFound(t *testing.T) {
	m := NewMockContext(features())
	assert.Equal(t, float64(42), m.Number(context.TODO(), "numFlag", 0))
}

func TestMockNumberDefault(t *testing.T) {
	m := NewMockContext(features())
	assert.Equal(t, float64(99), m.Number(context.TODO(), "missing", 99))
}

func TestMockStringFound(t *testing.T) {
	m := NewMockContext(features())
	assert.Equal(t, "hello", m.String(context.TODO(), "strFlag", "default"))
}

func TestMockStringDefault(t *testing.T) {
	m := NewMockContext(features())
	assert.Equal(t, "default", m.String(context.TODO(), "missing", "default"))
}

func TestMockJSONFound(t *testing.T) {
	m := NewMockContext(features())
	assert.Equal(t, `{"x":1}`, m.JSON(context.TODO(), "jsonFlag", "{}"))
}

func TestMockJSONDefault(t *testing.T) {
	m := NewMockContext(features())
	assert.Equal(t, "{}", m.JSON(context.TODO(), "missing", "{}"))
}

// --- AllKeys ---

func TestMockAllKeys(t *testing.T) {
	m := NewMockContext(features())
	keys := m.AllKeys()
	assert.ElementsMatch(t, []string{"boolFlag", "numFlag", "strFlag", "jsonFlag"}, keys)
}

// --- WithContext returns self ---

func TestMockWithContextReturnsSelf(t *testing.T) {
	m := NewMockContext(features())
	result := m.WithContext(nil)
	assert.Same(t, m, result)
}

// --- AsConvertibleString ---

func TestMockAsConvertibleStringFound(t *testing.T) {
	m := NewMockContext(features())
	s, err := m.AsConvertibleString(context.TODO(), "numFlag")
	require.NoError(t, err)
	assert.Equal(t, "42", s)
}

func TestMockAsConvertibleStringNotFound(t *testing.T) {
	m := NewMockContext(features())
	_, err := m.AsConvertibleString(context.TODO(), "missing")
	assert.Error(t, err)
}
