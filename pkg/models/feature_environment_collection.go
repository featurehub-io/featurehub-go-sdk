package models

// FeatureEnvironmentCollection represents a collection of features for a single environment,
// as returned by the FeatureHub REST polling endpoint.
type FeatureEnvironmentCollection struct {
	ID       string          `json:"id"`
	Features []*FeatureState `json:"features,omitempty"`
}
