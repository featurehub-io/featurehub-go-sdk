package interceptors

import (
	"context"
	"os"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const (
	overridesEnvVar      = "FEATUREHUB_OVERRIDES"
	defaultOverridesFile = "featurehub-overrides.yaml"
)

// yamlOverrideEntry is a single entry in the overrides YAML file.
type yamlOverrideEntry struct {
	Key   string      `yaml:"key"`
	Type  string      `yaml:"type"`
	Value interface{} `yaml:"value"`
}

// NewLocalYamlValueInterceptor returns a FeatureValueInterceptor backed by a YAML overrides
// file. The file path is taken from the FEATUREHUB_OVERRIDES environment variable, defaulting
// to "featurehub-overrides.yaml". Values are converted to their native Go types at
// initialisation time; any conversion errors are logged and that entry is skipped.
//
// The YAML file format is a list of entries:
//
//   - key: myBoolFlag
//     type: BOOLEAN
//     value: true
//   - key: myNumber
//     type: NUMBER
//     value: 42.5
//   - key: myString
//     type: STRING
//     value: "hello world"
//   - key: myJson
//     type: JSON
//     value: '{"enabled": true}'
func NewLocalYamlValueInterceptor(logger *logrus.Logger) interfaces.FeatureValueInterceptor {
	overrides := loadOverrides(logger)
	return func(context context.Context, key string, _ *models.FeatureState) (interface{}, bool) {
		if value, ok := overrides[key]; ok {
			return value, true
		}
		return nil, false
	}
}

func loadOverrides(logger *logrus.Logger) map[string]interface{} {
	path := os.Getenv(overridesEnvVar)
	if path == "" {
		path = defaultOverridesFile
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.WithError(err).WithField("file", path).Error("Failed to read feature overrides file")
		}
		return map[string]interface{}{}
	}

	var entries []yamlOverrideEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		logger.WithError(err).WithField("file", path).Error("Failed to parse feature overrides file")
		return map[string]interface{}{}
	}

	overrides := make(map[string]interface{}, len(entries))
	for _, entry := range entries {
		converted, err := models.ConvertValue(models.FeatureValueType(entry.Type), entry.Value)
		if err != nil {
			logger.WithError(err).
				WithField("key", entry.Key).
				WithField("type", entry.Type).
				Error("Failed to convert override value, skipping entry")
			continue
		}
		overrides[entry.Key] = converted
	}

	return overrides
}
