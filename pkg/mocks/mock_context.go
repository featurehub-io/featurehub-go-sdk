package mocks

import (
	"context"
	"fmt"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
)

// MockContext implements interfaces.Context using a map of feature key → value.
// Values should be the native Go type for the feature (bool, float64, string).
// Keys absent from the map cause Get* methods to return an error and convenience
// methods (Boolean, Number, String, JSON) to return their supplied default value.
type MockContext struct {
	features map[string]interface{}
}

// NewMockContext returns a MockContext populated with the provided feature values.
func NewMockContext(features map[string]interface{}) *MockContext {
	return &MockContext{features: features}
}

var _ interfaces.Context = (*MockContext)(nil)

// --- Get* methods ---

func (m *MockContext) GetBoolean(_ context.Context, featureKey string) (bool, error) {
	v, ok := m.features[featureKey]
	if !ok {
		return false, errors.NewErrFeatureNotFound(featureKey)
	}
	b, ok := v.(bool)
	if !ok {
		return false, errors.NewErrInvalidType(featureKey)
	}
	return b, nil
}

func (m *MockContext) GetNumber(_ context.Context, featureKey string) (*float64, error) {
	v, ok := m.features[featureKey]
	if !ok {
		return nil, errors.NewErrFeatureNotFound(featureKey)
	}
	f, ok := v.(float64)
	if !ok {
		return nil, errors.NewErrInvalidType(featureKey)
	}
	return &f, nil
}

func (m *MockContext) GetRawJSON(_ context.Context, featureKey string) (*string, error) {
	v, ok := m.features[featureKey]
	if !ok {
		return nil, errors.NewErrFeatureNotFound(featureKey)
	}
	s, ok := v.(string)
	if !ok {
		return nil, errors.NewErrInvalidType(featureKey)
	}
	return &s, nil
}

func (m *MockContext) GetString(_ context.Context, featureKey string) (*string, error) {
	v, ok := m.features[featureKey]
	if !ok {
		return nil, errors.NewErrFeatureNotFound(featureKey)
	}
	s, ok := v.(string)
	if !ok {
		return nil, errors.NewErrInvalidType(featureKey)
	}
	return &s, nil
}

// --- Convenience methods ---

func (m *MockContext) Boolean(_ context.Context, featureKey string, defaultValue bool) bool {
	v, ok := m.features[featureKey]
	if !ok {
		return defaultValue
	}
	b, ok := v.(bool)
	if !ok {
		return defaultValue
	}
	return b
}

func (m *MockContext) Number(_ context.Context, featureKey string, defaultValue float64) float64 {
	v, ok := m.features[featureKey]
	if !ok {
		return defaultValue
	}
	f, ok := v.(float64)
	if !ok {
		return defaultValue
	}
	return f
}

func (m *MockContext) String(_ context.Context, featureKey string, defaultValue string) string {
	v, ok := m.features[featureKey]
	if !ok {
		return defaultValue
	}
	s, ok := v.(string)
	if !ok {
		return defaultValue
	}
	return s
}

func (m *MockContext) JSON(_ context.Context, featureKey string, defaultValue string) string {
	v, ok := m.features[featureKey]
	if !ok {
		return defaultValue
	}
	s, ok := v.(string)
	if !ok {
		return defaultValue
	}
	return s
}

// --- AllKeys ---

func (m *MockContext) AllKeys() []string {
	keys := make([]string, 0, len(m.features))
	for k := range m.features {
		keys = append(keys, k)
	}
	return keys
}

// --- Properties ---

func (m *MockContext) Properties(_ context.Context, _ string) map[string]string {
	return nil
}

// --- Notifiers (no-ops) ---

func (m *MockContext) AddNotifierBoolean(_ context.Context, _ string, _ models.CallbackFuncBoolean) (string, error) {
	return "", nil
}

func (m *MockContext) AddNotifierJSON(_ context.Context, _ string, _ models.CallbackFuncJSON) (string, error) {
	return "", nil
}

func (m *MockContext) AddNotifierNumber(_ context.Context, _ string, _ models.CallbackFuncNumber) (string, error) {
	return "", nil
}

func (m *MockContext) AddNotifierString(_ context.Context, _ string, _ models.CallbackFuncString) (string, error) {
	return "", nil
}

func (m *MockContext) AddNotifierFeature(_ context.Context, _ string, _ models.CallbackFuncFeature) (string, error) {
	return "", nil
}

func (m *MockContext) DeleteNotifier(_, _ string) error {
	return nil
}

// --- Context identity ---

func (m *MockContext) Attributes() *models.Context {
	return nil
}

func (m *MockContext) WithContext(_ *models.Context) interfaces.Context {
	return m
}

// --- Usage (no-ops) ---

func (m *MockContext) RecordUsageEvent(_ context.Context, _ usage.UsageEvent) {}

func (m *MockContext) GetContextUsage(_ context.Context) usage.UsageEvent {
	return nil
}

func (m *MockContext) RecordNamedUsage(_ context.Context, _ string, _ usage.ContextRecord) {}

// --- AsConvertibleString ---

func (m *MockContext) AsConvertibleString(_ context.Context, featureKey string) (string, error) {
	v, ok := m.features[featureKey]
	if !ok {
		return "", errors.NewErrFeatureNotFound(featureKey)
	}
	return fmt.Sprintf("%v", v), nil
}
