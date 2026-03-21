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
	f, err := os.CreateTemp(t.TempDir(), "overrides-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// setOverridesFile points FEATUREHUB_OVERRIDES at path for the duration of the test.
func setOverridesFile(t *testing.T, path string) {
	t.Helper()
	t.Setenv(overridesEnvVar, path)
}

// --- file not found ---

func TestNoFileReturnsNonMatchingInterceptor(t *testing.T) {
	t.Setenv(overridesEnvVar, filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor(context.TODO(), "anyKey", nil, nil)

	assert.False(t, matched)
	assert.Nil(t, value)
}

// --- key not in overrides ---

func TestUnknownKeyReturnsNoMatch(t *testing.T) {
	path := writeYAML(t, "- key: known\n  type: BOOLEAN\n  value: true\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor(context.TODO(), "unknown", nil, nil)

	assert.False(t, matched)
	assert.Nil(t, value)
}

// --- BOOLEAN ---

func TestBooleanNativeTrue(t *testing.T) {
	path := writeYAML(t, "- key: flag\n  type: BOOLEAN\n  value: true\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor(context.TODO(), "flag", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, true, value)
}

func TestBooleanNativeFalse(t *testing.T) {
	path := writeYAML(t, "- key: flag\n  type: BOOLEAN\n  value: false\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor(context.TODO(), "flag", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, false, value)
}

func TestBooleanStringTrue(t *testing.T) {
	path := writeYAML(t, "- key: flag\n  type: BOOLEAN\n  value: \"true\"\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor(context.TODO(), "flag", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, true, value)
}

func TestBooleanStringFalse(t *testing.T) {
	path := writeYAML(t, "- key: flag\n  type: BOOLEAN\n  value: \"false\"\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor(context.TODO(), "flag", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, false, value)
}

// --- NUMBER ---

func TestNumberNativeFloat(t *testing.T) {
	path := writeYAML(t, "- key: count\n  type: NUMBER\n  value: 3.14\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor(context.TODO(), "count", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, float64(3.14), value)
}

func TestNumberNativeInt(t *testing.T) {
	path := writeYAML(t, "- key: count\n  type: NUMBER\n  value: 42\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor(context.TODO(), "count", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, float64(42), value)
}

func TestNumberStringValue(t *testing.T) {
	path := writeYAML(t, "- key: count\n  type: NUMBER\n  value: \"99.5\"\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor(context.TODO(), "count", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, float64(99.5), value)
}

// --- STRING ---

func TestStringValue(t *testing.T) {
	path := writeYAML(t, "- key: label\n  type: STRING\n  value: hello world\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor(context.TODO(), "label", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, "hello world", value)
}

func TestStringEmptyValue(t *testing.T) {
	path := writeYAML(t, "- key: label\n  type: STRING\n  value: \"\"\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor(context.TODO(), "label", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, "", value)
}

// --- JSON ---

func TestJSONValue(t *testing.T) {
	path := writeYAML(t, "- key: cfg\n  type: JSON\n  value: '{\"enabled\": true}'\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor(context.TODO(), "cfg", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, `{"enabled": true}`, value)
}

// --- multiple entries ---

func TestMultipleEntriesAllResolved(t *testing.T) {
	yaml := `
- key: flagA
  type: BOOLEAN
  value: true
- key: flagB
  type: NUMBER
  value: 7
- key: flagC
  type: STRING
  value: hi
`
	setOverridesFile(t, writeYAML(t, yaml))
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	val, matched := interceptor(context.TODO(), "flagA", nil, nil)
	assert.True(t, matched)
	assert.Equal(t, true, val)

	val, matched = interceptor(context.TODO(), "flagB", nil, nil)
	assert.True(t, matched)
	assert.Equal(t, float64(7), val)

	val, matched = interceptor(context.TODO(), "flagC", nil, nil)
	assert.True(t, matched)
	assert.Equal(t, "hi", val)
}

// --- feature state is ignored ---

func TestFeatureStateArgumentIsIgnored(t *testing.T) {
	path := writeYAML(t, "- key: flag\n  type: BOOLEAN\n  value: true\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	fs := &models.FeatureState{Key: "flag", Type: models.TypeBoolean, Value: false}
	value, matched := interceptor(context.TODO(), "flag", nil, fs)

	assert.True(t, matched)
	assert.Equal(t, true, value, "interceptor value should win over the stored feature state")
}

// --- error handling ---

func TestInvalidTypeSkipsEntryAndContinues(t *testing.T) {
	yaml := `
- key: bad
  type: UNKNOWN_TYPE
  value: whatever
- key: good
  type: BOOLEAN
  value: true
`
	setOverridesFile(t, writeYAML(t, yaml))
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	_, matched := interceptor(context.TODO(), "bad", nil, nil)
	assert.False(t, matched, "entry with unknown type should be skipped")

	val, matched := interceptor(context.TODO(), "good", nil, nil)
	assert.True(t, matched)
	assert.Equal(t, true, val)
}

func TestMalformedYAMLReturnsNonMatchingInterceptor(t *testing.T) {
	path := writeYAML(t, "this: is: not: valid: yaml: [\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	_, matched := interceptor(context.TODO(), "anyKey", nil, nil)
	assert.False(t, matched)
}

// --- env var override ---

func TestEnvVarOverridesDefaultPath(t *testing.T) {
	path := writeYAML(t, "- key: fromEnvVar\n  type: STRING\n  value: yes\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	value, matched := interceptor(context.TODO(), "fromEnvVar", nil, nil)

	assert.True(t, matched)
	assert.Equal(t, "yes", value)
}
