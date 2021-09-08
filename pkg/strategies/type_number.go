package strategies

import (
	"fmt"
	"reflect"
)

// TypeNumber is for numerical values:
const TypeNumber = "NUMBER"

// Number asserts the given parameters then passes on for evaluation:
func Number(conditional string, options []interface{}, value interface{}) (bool, error) {
	var assertedValue float64
	var ok bool

	// Type switch on the value (because numbers can come in a bunch of interesting shapes and sizes):
	switch value.(type) {
	// case float32:
	// 	assertedValue = float64(value.(float32))
	case int:
		assertedValue = float64(value.(int))
	case int8:
		assertedValue = float64(value.(int8))
	case int16:
		assertedValue = float64(value.(int16))
	case int32:
		assertedValue = float64(value.(int32))
	case int64:
		assertedValue = float64(value.(int64))
	case uint:
		assertedValue = float64(value.(uint))
	case uint8:
		assertedValue = float64(value.(uint8))
	case uint16:
		assertedValue = float64(value.(uint16))
	case uint32:
		assertedValue = float64(value.(uint32))
	case uint64:
		assertedValue = float64(value.(uint64))
	default:
		// In case new numeric types are invented:
		assertedValue, ok = value.(float64)
		if !ok {
			return false, fmt.Errorf("Unable to assert %s value (%v) as float64", reflect.TypeOf(value), value)
		}
	}

	// Type assert all of the options:
	var assertedOptions []float64
	for _, option := range options {
		assertedOption, ok := option.(float64)
		if !ok {
			return false, fmt.Errorf("Unable to assert value (%v) as float64", option)
		}
		assertedOptions = append(assertedOptions, assertedOption)
	}

	return evaluateNumber(conditional, assertedOptions, assertedValue), nil
}

// evaluateNumber makes evaluations for TypeNumber values:
func evaluateNumber(conditional string, options []float64, value float64) bool {

	switch conditional {

	case ConditionalEquals:
		// Return true if the value is equal to any of the options:
		for _, option := range options {
			if value == option {
				return true
			}
		}
		return false

	case ConditionalNotEquals:
		// Return false if the value is equal to any of the options:
		for _, option := range options {
			if value == option {
				return false
			}
		}
		return true

	case ConditionalLess:
		// Return false if the value is greater than or equal to any of the options:
		for _, option := range options {
			if value >= option {
				return false
			}
		}
		return true

	case ConditionalLessEquals:
		// Return false if the value is greater than any of the options:
		for _, option := range options {
			if value > option {
				return false
			}
		}
		return true

	case ConditionalGreater:
		// Return false if the value is less than or equal to any of the options:
		for _, option := range options {
			if value <= option {
				return false
			}
		}
		return true

	case ConditionalGreaterEquals:
		// Return false if the value is less than any of the options:
		for _, option := range options {
			if value < option {
				return false
			}
		}
		return true

	case ConditionalExcludes:
		// Return false if the value is equal to any of the options:
		for _, option := range options {
			if value == option {
				return false
			}
		}
		return true

	case ConditionalIncludes:
		// Return true if the value is equal to any of the options:
		for _, option := range options {
			if value == option {
				return true
			}
		}
		return false

	default:
		return false
	}
}
