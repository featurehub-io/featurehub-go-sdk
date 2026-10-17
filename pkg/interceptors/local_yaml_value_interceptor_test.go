package interceptors

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newLogger returns a discarding logger so test output is clean.
func newLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

// writeYAML writes content to a temp file and returns its path.
func writeYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "features-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// setLocalYamlFile points FEATUREHUB_LOCAL_YAML at path for the duration of the test.
func setLocalYamlFile(t *testing.T, path string) {
	t.Helper()
	t.Setenv(localYamlEnvVar, path)
}

// --- file not found ---

func TestNoFileReturnsNonMatchingInterceptor(t *testing.T) {
	t.Setenv(localYamlEnvVar, filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor.Intercept(context.TODO(), "anyKey", nil, nil)

	assert.False(t, matched)
	assert.Nil(t, value)
}

// --- missing flagValues field ---

func TestMissingFlagValuesFieldReturnsNoMatch(t *testing.T) {
	path := writeYAML(t, "someOtherField: true\n")
	setLocalYamlFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	_, matched := interceptor.Intercept(context.TODO(), "anyKey", nil, nil)
	assert.False(t, matched)
}

// --- key not in overrides ---

func TestUnknownKeyReturnsNoMatch(t *testing.T) {
	path := writeYAML(t, "flagValues:\n  known: true\n")
	setLocalYamlFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor.Intercept(context.TODO(), "unknown", nil, nil)

	assert.False(t, matched)
	assert.Nil(t, value)
}

// --- BOOLEAN ---

func TestBooleanTrue(t *testing.T) {
	path := writeYAML(t, "flagValues:\n  flag: true\n")
	setLocalYamlFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor.Intercept(context.TODO(), "flag", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, true, value)
}

func TestBooleanFalse(t *testing.T) {
	path := writeYAML(t, "flagValues:\n  flag: false\n")
	setLocalYamlFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor.Intercept(context.TODO(), "flag", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, false, value)
}

// --- NUMBER ---

func TestNumberInteger(t *testing.T) {
	path := writeYAML(t, "flagValues:\n  count: 42\n")
	setLocalYamlFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor.Intercept(context.TODO(), "count", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, float64(42), value)
}

func TestNumberFloat(t *testing.T) {
	path := writeYAML(t, "flagValues:\n  ratio: 3.14\n")
	setLocalYamlFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor.Intercept(context.TODO(), "ratio", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, float64(3.14), value)
}

// --- STRING ---

func TestStringValue(t *testing.T) {
	path := writeYAML(t, "flagValues:\n  greeting: \"hello world\"\n")
	setLocalYamlFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor.Intercept(context.TODO(), "greeting", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, "hello world", value)
}

func TestStringEmptyValue(t *testing.T) {
	path := writeYAML(t, "flagValues:\n  label: \"\"\n")
	setLocalYamlFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor.Intercept(context.TODO(), "label", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, "", value)
}

// --- JSON (complex structures) ---

func TestComplexMapBecomesJSONString(t *testing.T) {
	yaml := "flagValues:\n  cfg:\n    timeout: 30\n    retries: 3\n"
	path := writeYAML(t, yaml)
	setLocalYamlFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor.Intercept(context.TODO(), "cfg", nil, nil)

	assert.True(t, matched)
	str, ok := value.(string)
	require.True(t, ok, "complex map should be returned as a JSON string")
	assert.JSONEq(t, `{"timeout":30,"retries":3}`, str)
}

func TestComplexSliceBecomesJSONString(t *testing.T) {
	yaml := "flagValues:\n  tags:\n    - alpha\n    - beta\n"
	path := writeYAML(t, yaml)
	setLocalYamlFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor.Intercept(context.TODO(), "tags", nil, nil)

	assert.True(t, matched)
	str, ok := value.(string)
	require.True(t, ok, "slice should be returned as a JSON string")
	assert.JSONEq(t, `["alpha","beta"]`, str)
}

// --- multiple entries ---

func TestMultipleEntriesAllResolved(t *testing.T) {
	yaml := "flagValues:\n  flagA: true\n  flagB: 7\n  flagC: hi\n"
	setLocalYamlFile(t, writeYAML(t, yaml))
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	val, matched := interceptor.Intercept(context.TODO(), "flagA", nil, nil)
	assert.True(t, matched)
	assert.Equal(t, true, val)

	val, matched = interceptor.Intercept(context.TODO(), "flagB", nil, nil)
	assert.True(t, matched)
	assert.Equal(t, float64(7), val)

	val, matched = interceptor.Intercept(context.TODO(), "flagC", nil, nil)
	assert.True(t, matched)
	assert.Equal(t, "hi", val)
}

// --- path passed directly ---

func TestExplicitPathTakesPrecedence(t *testing.T) {
	// env var points at a different file — the explicit path should win
	envPath := writeYAML(t, "flagValues:\n  flag: false\n")
	setLocalYamlFile(t, envPath)

	explicitPath := writeYAML(t, "flagValues:\n  flag: true\n")
	interceptor := NewLocalYamlValueInterceptor(newLogger(), explicitPath)

	value, matched := interceptor.Intercept(context.TODO(), "flag", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, true, value)
}

// --- env var path ---

func TestEnvVarPathIsUsedWhenNoExplicitPath(t *testing.T) {
	path := writeYAML(t, "flagValues:\n  fromEnv: \"yes\"\n")
	setLocalYamlFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor.Intercept(context.TODO(), "fromEnv", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, "yes", value)
}

// --- feature state is ignored ---

func TestFeatureStateArgumentIsIgnored(t *testing.T) {
	path := writeYAML(t, "flagValues:\n  flag: true\n")
	setLocalYamlFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	fs := &models.FeatureState{Key: "flag", Type: models.TypeBoolean, Value: false}
	value, matched := interceptor.Intercept(context.TODO(), "flag", nil, fs)

	assert.True(t, matched)
	assert.Equal(t, true, value, "interceptor value should win over the stored feature state")
}

// --- error handling ---

func TestMalformedYAMLReturnsNonMatchingInterceptor(t *testing.T) {
	path := writeYAML(t, "this: is: not: valid: yaml: [\n")
	setLocalYamlFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	_, matched := interceptor.Intercept(context.TODO(), "anyKey", nil, nil)
	assert.False(t, matched)
}
