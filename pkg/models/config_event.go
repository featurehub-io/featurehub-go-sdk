package models

// ConfigEvent defines model for a FeatureHub config event.
type ConfigEvent struct {
	EdgeStale bool `json:"edge.stale,omitempty"` // The edge server has become stale
}
