package interceptors

import (
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

	matched, value := interceptor("anyKey", nil)

	assert.False(t, matched)
	assert.Nil(t, value)
}

// --- key not in overrides ---

func TestUnknownKeyReturnsNoMatch(t *testing.T) {
	path := writeYAML(t, "- key: known\n  type: BOOLEAN\n  value: true\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	matched, value := interceptor("unknown", nil)

	assert.False(t, matched)
	assert.Nil(t, value)
}

// --- BOOLEAN ---

func TestBooleanNativeTrue(t *testing.T) {
	path := writeYAML(t, "- key: flag\n  type: BOOLEAN\n  value: true\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	matched, value := interceptor("flag", nil)

	assert.True(t, matched)
	assert.Equal(t, true, value)
}

func TestBooleanNativeFalse(t *testing.T) {
	path := writeYAML(t, "- key: flag\n  type: BOOLEAN\n  value: false\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	matched, value := interceptor("flag", nil)

	assert.True(t, matched)
	assert.Equal(t, false, value)
}

func TestBooleanStringTrue(t *testing.T) {
	path := writeYAML(t, "- key: flag\n  type: BOOLEAN\n  value: \"true\"\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	matched, value := interceptor("flag", nil)

	assert.True(t, matched)
	assert.Equal(t, true, value)
}

func TestBooleanStringFalse(t *testing.T) {
	path := writeYAML(t, "- key: flag\n  type: BOOLEAN\n  value: \"false\"\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	matched, value := interceptor("flag", nil)

	assert.True(t, matched)
	assert.Equal(t, false, value)
}

// --- NUMBER ---

func TestNumberNativeFloat(t *testing.T) {
	path := writeYAML(t, "- key: count\n  type: NUMBER\n  value: 3.14\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	matched, value := interceptor("count", nil)

	assert.True(t, matched)
	assert.Equal(t, float64(3.14), value)
}

func TestNumberNativeInt(t *testing.T) {
	path := writeYAML(t, "- key: count\n  type: NUMBER\n  value: 42\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	matched, value := interceptor("count", nil)

	assert.True(t, matched)
	assert.Equal(t, float64(42), value)
}

func TestNumberStringValue(t *testing.T) {
	path := writeYAML(t, "- key: count\n  type: NUMBER\n  value: \"99.5\"\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	matched, value := interceptor("count", nil)

	assert.True(t, matched)
	assert.Equal(t, float64(99.5), value)
}

// --- STRING ---

func TestStringValue(t *testing.T) {
	path := writeYAML(t, "- key: label\n  type: STRING\n  value: hello world\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	matched, value := interceptor("label", nil)

	assert.True(t, matched)
	assert.Equal(t, "hello world", value)
}

func TestStringEmptyValue(t *testing.T) {
	path := writeYAML(t, "- key: label\n  type: STRING\n  value: \"\"\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	matched, value := interceptor("label", nil)

	assert.True(t, matched)
	assert.Equal(t, "", value)
}

// --- JSON ---

func TestJSONValue(t *testing.T) {
	path := writeYAML(t, "- key: cfg\n  type: JSON\n  value: '{\"enabled\": true}'\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	matched, value := interceptor("cfg", nil)

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

	matched, val := interceptor("flagA", nil)
	assert.True(t, matched)
	assert.Equal(t, true, val)

	matched, val = interceptor("flagB", nil)
	assert.True(t, matched)
	assert.Equal(t, float64(7), val)

	matched, val = interceptor("flagC", nil)
	assert.True(t, matched)
	assert.Equal(t, "hi", val)
}

// --- feature state is ignored ---

func TestFeatureStateArgumentIsIgnored(t *testing.T) {
	path := writeYAML(t, "- key: flag\n  type: BOOLEAN\n  value: true\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	fs := &models.FeatureState{Key: "flag", Type: models.TypeBoolean, Value: false}
	matched, value := interceptor("flag", fs)

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

	matched, _ := interceptor("bad", nil)
	assert.False(t, matched, "entry with unknown type should be skipped")

	matched, val := interceptor("good", nil)
	assert.True(t, matched)
	assert.Equal(t, true, val)
}

func TestMalformedYAMLReturnsNonMatchingInterceptor(t *testing.T) {
	path := writeYAML(t, "this: is: not: valid: yaml: [\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	matched, _ := interceptor("anyKey", nil)
	assert.False(t, matched)
}

// --- env var override ---

func TestEnvVarOverridesDefaultPath(t *testing.T) {
	path := writeYAML(t, "- key: fromEnvVar\n  type: STRING\n  value: yes\n")
	setOverridesFile(t, path)
	interceptor := NewLocalYamlValueInterceptor(newLogger())

	matched, value := interceptor("fromEnvVar", nil)

	assert.True(t, matched)
	assert.Equal(t, "yes", value)
}

// --- convertValue unit tests ---

func TestConvertValueBooleanTrue(t *testing.T) {
	v, err := convertValue("BOOLEAN", true)
	require.NoError(t, err)
	assert.Equal(t, true, v)
}

func TestConvertValueBooleanStringFalse(t *testing.T) {
	v, err := convertValue("BOOLEAN", "false")
	require.NoError(t, err)
	assert.Equal(t, false, v)
}

func TestConvertValueNumberInt(t *testing.T) {
	v, err := convertValue("NUMBER", 5)
	require.NoError(t, err)
	assert.Equal(t, float64(5), v)
}

func TestConvertValueNumberFloat(t *testing.T) {
	v, err := convertValue("NUMBER", float64(1.5))
	require.NoError(t, err)
	assert.Equal(t, float64(1.5), v)
}

func TestConvertValueNumberString(t *testing.T) {
	v, err := convertValue("NUMBER", "2.5")
	require.NoError(t, err)
	assert.Equal(t, float64(2.5), v)
}

func TestConvertValueStringPassthrough(t *testing.T) {
	v, err := convertValue("STRING", "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", v)
}

func TestConvertValueJSONPassthrough(t *testing.T) {
	v, err := convertValue("JSON", `{"x":1}`)
	require.NoError(t, err)
	assert.Equal(t, `{"x":1}`, v)
}

func TestConvertValueUnknownTypeReturnsError(t *testing.T) {
	_, err := convertValue("NONSENSE", "value")
	assert.Error(t, err)
}
