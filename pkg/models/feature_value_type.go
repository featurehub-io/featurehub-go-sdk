package models

import (
	"fmt"
	"strconv"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
)

const (
	// TypeBoolean is a basic boolean:
	TypeBoolean FeatureValueType = "BOOLEAN"
	// TypeString is a basic string:
	TypeString FeatureValueType = "STRING"
	// TypeNumber is a basic number (float64):
	TypeNumber FeatureValueType = "NUMBER"
	// TypeJSON is a serialised JSON string:
	TypeJSON FeatureValueType = "JSON"

	// TypeFeature is not real, its simply an internal signal to allow Notifiers for whole features
	TypeFeature FeatureValueType = "__internal__feature"
)

// FeatureValueType defines model for FeatureValueType.
type FeatureValueType string

func ConvertToString(typeName FeatureValueType, raw interface{}) (string, error) {
	val, err := ConvertValue(typeName, raw)

	if err != nil {
		return "", err
	}

	switch typeName {
	case TypeBoolean:
		if val.(bool) {
			return "t", err
		} else {
			return "f", err
		}
	case TypeString, TypeJSON:
		return val.(string), err
	case TypeNumber:
		return fmt.Sprintf("%v", val), err
	default:
		return "", errors.NewErrInvalidType("don't know what to do with this value")
	}
}

// ConvertValue converts a raw value (as decoded from YAML, JSON, or similar) to the Go type
// expected for the given FeatureValueType. Scalars may arrive as bool, int, int64, float64, or
// string; ConvertValue normalises them to bool, float64, or string as appropriate.
func ConvertValue(typeName FeatureValueType, raw interface{}) (interface{}, error) {
	switch typeName {
	case TypeBoolean:
		switch v := raw.(type) {
		case bool:
			return v, nil
		case string:
			// copy with reversing usage plugin values
			if v == "on" || v == "yes" || v == "y" || v == "t" {
				return true, nil
			}
			if v == "off" || v == "no" || v == "n" || v == "f" {
				return false, nil
			}
			return strconv.ParseBool(v)
		default:
			return strconv.ParseBool(fmt.Sprintf("%v", raw))
		}

	case TypeNumber:
		switch v := raw.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		case int64:
			return float64(v), nil
		case string:
			return strconv.ParseFloat(v, 64)
		default:
			return strconv.ParseFloat(fmt.Sprintf("%v", raw), 64)
		}

	case TypeString, TypeJSON:
		switch v := raw.(type) {
		case string:
			return v, nil
		default:
			return fmt.Sprintf("%v", raw), nil
		}

	default:
		return nil, fmt.Errorf("unknown feature value type %q", typeName)
	}
}
