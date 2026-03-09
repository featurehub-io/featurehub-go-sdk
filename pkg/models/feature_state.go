package models

import (
	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
)

// FeatureState defines model for FeatureState.
type FeatureState struct {
	ID            string            `json:"id,omitempty"`            // ID, this field cannot be empty or nil
	Key           string            `json:"key,omitempty"`           // Name of the feature, this field cannot be empty or nil
	Strategies    Strategies        `json:"strategies,omitempty"`    // Rollout strategy, this can be nil or an empty array
	Type          FeatureValueType  `json:"type,omitempty"`          // Data type, this field cannot be empty or nil
	Value         interface{}       `json:"value,omitempty"`         // the current value, this field can be nil unless the Type is TypeBoolean in which case it will ALWAYS be true or false
	Version       int64             `json:"version,omitempty"`       // Version, this field cannot be empty or nil
	Properties    map[string]string `json:"fp,omitempty"`            // Properties from the server. Not supported in SaaS, but can be nil
	EnvironmentID string            `json:"environmentId,omitempty"` // Environment this feature is set client side application.
}

// AsBoolean returns a boolean value for this feature:
func (fs *FeatureState) AsBoolean() (bool, error) {

	// Make sure the feature is the correct type:
	if fs.Type != TypeBoolean {
		return false, errors.NewErrInvalidType(string(fs.Type))
	}

	// Assert the value:
	defaultValue, ok := fs.Value.(bool)
	if !ok {
		return false, errors.NewErrInvalidType("Unable to assert value as a bool")
	}

	// Return the default value as a fall-back:
	return defaultValue, nil
}

// AsNumber returns a number value for this feature:
func (fs *FeatureState) AsNumber() (float64, error) {

	// Make sure the feature is the correct type:
	if fs.Type != TypeNumber {
		return 0, errors.NewErrInvalidType(string(fs.Type))
	}

	// Assert the value:
	defaultValue, ok := fs.Value.(float64)
	if !ok {
		return 0, errors.NewErrInvalidType("Unable to assert value as a float64")
	}

	// Return the default value as a fall-back:
	return defaultValue, nil
}

// AsRawJSON returns a raw JSON value for this feature:
func (fs *FeatureState) AsRawJSON() (string, error) {

	// Make sure the feature is the correct type:
	if fs.Type != TypeJSON {
		return "{}", errors.NewErrInvalidType(string(fs.Type))
	}

	// Assert the value:
	defaultValue, ok := fs.Value.(string)
	if !ok {
		return "{}", errors.NewErrInvalidType("Unable to assert value as a string")
	}

	// Return the default value as a fall-back:
	return defaultValue, nil
}

// AsString returns a string value for this feature:
func (fs *FeatureState) AsString() (string, error) {

	// Make sure the feature is the correct type:
	if fs.Type != TypeString {
		return "", errors.NewErrInvalidType(string(fs.Type))
	}

	// Assert the value:
	defaultValue, ok := fs.Value.(string)
	if !ok {
		return "", errors.NewErrInvalidType("Unable to assert value as a string")
	}

	// Return the default value as a fall-back:
	return defaultValue, nil
}
