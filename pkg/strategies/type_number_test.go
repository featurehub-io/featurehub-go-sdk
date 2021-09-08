package strategies

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNumberTypeAssertion(t *testing.T) {

	// Try with some unsupported options values:
	_, err := Number(ConditionalEquals, []interface{}{"string", false}, 123)
	assert.Error(t, err)

	// Try with an unsupported comparison value:
	_, err = Number(ConditionalEquals, []interface{}{55.123}, "string")
	assert.Error(t, err)

	// Test all valid numeric types:
	for numericValue := range []interface{}{
		float32(44.5), float64(44.5),
		uint(475), uint8(75), uint16(7125), uint32(712312315), uint64(7511111131232331231),
		int(475), int8(75), int16(7125), int32(712312315), int64(7511111131232331231),
	} {
		_, err = Number(ConditionalEquals, []interface{}{55.123}, numericValue)
		assert.NoError(t, err)
	}
}

func TestNumberEquals(t *testing.T) {
	assert.True(t, evaluateNumber(ConditionalEquals, []float64{1.2, 1.4}, 1.2))
	assert.False(t, evaluateNumber(ConditionalEquals, []float64{1.2, 1.4}, 1.3))
}

func TestNumberNotEquals(t *testing.T) {
	assert.False(t, evaluateNumber(ConditionalNotEquals, []float64{1.2, 1.4}, 1.2))
	assert.True(t, evaluateNumber(ConditionalNotEquals, []float64{1.2, 1.4}, 1.3))
}

func TestNumberLess(t *testing.T) {
	assert.True(t, evaluateNumber(ConditionalLess, []float64{1.2, 1.4}, 1.1))
	assert.False(t, evaluateNumber(ConditionalLess, []float64{1.2, 1.4}, 1.2))
	assert.False(t, evaluateNumber(ConditionalLess, []float64{1.2, 1.4}, 1.3))
}

func TestNumberLessEquals(t *testing.T) {
	assert.True(t, evaluateNumber(ConditionalLessEquals, []float64{1.2, 1.4}, 1.1))
	assert.True(t, evaluateNumber(ConditionalLessEquals, []float64{1.2, 1.4}, 1.2))
	assert.False(t, evaluateNumber(ConditionalLessEquals, []float64{1.2, 1.4}, 1.3))
}

func TestNumberGreater(t *testing.T) {
	assert.False(t, evaluateNumber(ConditionalGreater, []float64{1.2, 1.4}, 1.1))
	assert.False(t, evaluateNumber(ConditionalGreater, []float64{1.2, 1.4}, 1.4))
	assert.True(t, evaluateNumber(ConditionalGreater, []float64{1.2, 1.4}, 1.5))
}

func TestNumberGreaterEquals(t *testing.T) {
	assert.False(t, evaluateNumber(ConditionalGreaterEquals, []float64{1.2, 1.4}, 1.1))
	assert.True(t, evaluateNumber(ConditionalGreaterEquals, []float64{1.2, 1.4}, 1.4))
	assert.True(t, evaluateNumber(ConditionalGreaterEquals, []float64{1.2, 1.4}, 1.5))
}

func TestNumberExcludes(t *testing.T) {
	assert.False(t, evaluateNumber(ConditionalExcludes, []float64{1.2, 1.4}, 1.2))
	assert.True(t, evaluateNumber(ConditionalExcludes, []float64{1.2, 1.4}, 1.3))
}

func TestNumberIncludes(t *testing.T) {
	assert.True(t, evaluateNumber(ConditionalIncludes, []float64{1.2, 1.4}, 1.2))
	assert.False(t, evaluateNumber(ConditionalIncludes, []float64{1.2, 1.4}, 1.3))
}
