package interfaces

import (
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
)

// AnalyticsCollector allows the user to generate analytics events:
type AnalyticsCollector interface {
	LogEvent(action string, other map[string]string, featureStateAtCurrentTime map[string]*models.FeatureState) error
}
