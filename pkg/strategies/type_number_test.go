package strategies

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type numericTest struct {
	err     bool
	options []interface{}
	result  bool
	value   interface{}
}

func TestNumberTypeAssertion(t *testing.T) {

	// Run numeric comparisons against a set of pre-defined tests and expected results:
	for _, numericComparisonTest := range []numericTest{
		{
			// This will fail because we can only compare against float64 (this is how it comes with JSON from the FH API):
			err:     true,
			options: []interface{}{"string", false},
			value:   123,
		},
		{
			// This will fail because we can only compare numeric types (not strings):
			err:     true,
			options: []interface{}{55.123},
			value:   "string",
		},
		{
			// This will fail because of different floating point precision:
			err:     true,
			options: []interface{}{55.123},
			result:  false,
			value:   float32(55.123),
		},
		{
			options: []interface{}{55.123},
			result:  true,
			value:   float64(55.123),
		},
		{
			options: []interface{}{float64(55)},
			result:  true,
			value:   int(55),
		},
		{
			options: []interface{}{float64(55)},
			result:  true,
			value:   int8(55),
		},
		{
			options: []interface{}{float64(5555)},
			result:  true,
			value:   int16(5555),
		},
		{
			options: []interface{}{float64(712312315)},
			result:  true,
			value:   int32(712312315),
		},
		{
			options: []interface{}{float64(7511111131232331231)},
			result:  true,
			value:   int64(7511111131232331231),
		},
		{
			options: []interface{}{float64(55)},
			result:  true,
			value:   uint(55),
		},
		{
			options: []interface{}{float64(55)},
			result:  true,
			value:   uint8(55),
		},
		{
			options: []interface{}{float64(5555)},
			result:  true,
			value:   uint16(5555),
		},
		{
			options: []interface{}{float64(712312315)},
			result:  true,
			value:   uint32(712312315),
		},
		{
			options: []interface{}{float64(7511111131232331231)},
			result:  true,
			value:   uint64(7511111131232331231),
		},
	} {
		ok, err := Number(ConditionalEquals, numericComparisonTest.options, numericComparisonTest.value)
		assert.Equal(t, numericComparisonTest.result, ok, "Value of type %s", reflect.TypeOf(numericComparisonTest.value))
		if numericComparisonTest.err {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
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
