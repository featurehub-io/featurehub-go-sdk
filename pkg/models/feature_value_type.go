package models

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
