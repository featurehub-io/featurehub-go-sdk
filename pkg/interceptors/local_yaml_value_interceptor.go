package interceptors

import (
	"context"
	"encoding/json"
	"os"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const (
	localYamlEnvVar      = "FEATUREHUB_LOCAL_YAML"
	defaultLocalYamlFile = "featurehub-features.yaml"
)

// localYamlFile is the top-level structure expected in the YAML file.
type localYamlFile struct {
	FlagValues map[string]interface{} `yaml:"flagValues"`
}

// NewLocalYamlValueInterceptor returns a FeatureValueInterceptor backed by a YAML file.
// An explicit file path may be supplied; if omitted, the path is taken from the
// FEATUREHUB_LOCAL_YAML environment variable, defaulting to "featurehub-features.yaml".
//
// The YAML file must contain a single top-level "flagValues" map. Types are inferred
// from the value:
//   - bool          → BOOLEAN
//   - integer/float → NUMBER (float64)
//   - string        → STRING
//   - map or slice  → JSON (serialised to a JSON string)
//
// Example:
//
//	flagValues:
//	  darkMode: true
//	  maxRetries: 5
//	  greeting: "Hello, world!"
//	  config:
//	    timeout: 30
//	    retries: 3
func NewLocalYamlValueInterceptor(logger *logrus.Logger, path ...string) interfaces.FeatureValueInterceptor {
	overrides := loadLocalYaml(logger, path...)
	return interfaces.NewInterceptor(func(_ context.Context, key string, _ interfaces.FeatureRepository, _ *models.FeatureState) (interface{}, bool) {
		if value, ok := overrides[key]; ok {
			return value, true
		}
		return nil, false
	})
}

func loadLocalYaml(logger *logrus.Logger, paths ...string) map[string]interface{} {
	filePath := ""
	if len(paths) > 0 && paths[0] != "" {
		filePath = paths[0]
	} else {
		filePath = os.Getenv(localYamlEnvVar)
		if filePath == "" {
			filePath = defaultLocalYamlFile
		}
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.WithError(err).WithField("file", filePath).Error("Failed to read local YAML feature file")
		}
		return map[string]interface{}{}
	}

	var contents localYamlFile
	if err := yaml.Unmarshal(data, &contents); err != nil {
		logger.WithError(err).WithField("file", filePath).Error("Failed to parse local YAML feature file")
		return map[string]interface{}{}
	}

	if contents.FlagValues == nil {
		return map[string]interface{}{}
	}

	overrides := make(map[string]interface{}, len(contents.FlagValues))
	for key, raw := range contents.FlagValues {
		converted, err := convertLocalValue(raw)
		if err != nil {
			logger.WithError(err).WithField("key", key).Error("Failed to convert local YAML value, skipping entry")
			continue
		}
		overrides[key] = converted
	}

	return overrides
}

// convertLocalValue maps a raw YAML value to a Go type suitable for feature evaluation.
// Booleans stay bool, numbers become float64, strings stay string, and complex
// structures (maps, slices) are serialised to a JSON string.
func convertLocalValue(raw interface{}) (interface{}, error) {
	switch v := raw.(type) {
	case bool:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return v, nil
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return string(jsonBytes), nil
	}
}
