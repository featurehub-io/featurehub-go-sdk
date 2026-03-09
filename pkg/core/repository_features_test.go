package core

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func createRepository() *ClientFeatureHubRepository {
	// Make a Logger:
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logBuffer := new(bytes.Buffer)
	logger.SetOutput(logBuffer)

	// Use the config to make a new StreamingClient with a mock apiClient::
	return NewClientFeatureHubRepository(logger)
}

func featuresFromString(data string) ([]*models.FeatureState, error) {
	var features []*models.FeatureState
	err := json.Unmarshal([]byte(data), &features)
	return features, err
}

func featureFromString(data string) (*models.FeatureState, error) {
	var features *models.FeatureState
	err := json.Unmarshal([]byte(data), &features)
	return features, err
}

func ffs(data string) *models.FeatureState {
	var features *models.FeatureState
	json.Unmarshal([]byte(data), &features)
	return features
}

func TestRepositoryFeatures(t *testing.T) {
	repo := createRepository()

	var data = `[{"key":"booleanfeature","type":"BOOLEAN","value":true, "version":1},{"key":"jsonfeature","type":"JSON","value":"{\"is_crufty\": true}", "version":1},{"key":"numberfeature","type":"NUMBER","value":123456789, "version":1},{"key":"stringfeature","type":"STRING","value":"this is a string", "version":1}]`
	features, err := featuresFromString(data)
	assert.NoError(t, err)

	repo.ProcessFeatures(features)

	// Look for a feature that doesn't exist:
	_, _, _, err = repo.GetFeature("something-that-does-not-exist", false)
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrFeatureNotFound{}, err)

	// Look for a feature that DOES exist:
	feature, matched, value, err := repo.GetFeature("stringfeature", false)
	assert.NoError(t, err)
	assert.Equal(t, models.FeatureValueType("STRING"), feature.Type)
	assert.Equal(t, false, matched)
	assert.Equal(t, "this is a string", value)

	// Look for a boolean feature that is NOT a boolean:
	booleanFeature, err := repo.GetBoolean("stringfeature")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrInvalidType{}, err)

	// Look for a boolean feature that IS a boolean:
	booleanFeature, err = repo.GetBoolean("booleanfeature")
	assert.NoError(t, err)
	assert.Equal(t, true, booleanFeature)

	// Look for a JSON feature that is NOT JSON:
	jsonFeature, err := repo.GetRawJSON("numberfeature")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrInvalidType{}, err)

	// Look for a JSON feature that IS JSON:
	jsonFeature, err = repo.GetRawJSON("jsonfeature")
	assert.NoError(t, err)
	if assert.NotNil(t, jsonFeature) {
		assert.Equal(t, `{"is_crufty": true}`, *jsonFeature)
	}

	// Look for a number feature that is NOT a number:
	numberFeature, err := repo.GetNumber("stringfeature")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrInvalidType{}, err)

	// Look for a number feature that IS a number:
	numberFeature, err = repo.GetNumber("numberfeature")
	assert.NoError(t, err)
	if assert.NotNil(t, numberFeature) {
		assert.Equal(t, float64(123456789), *numberFeature)
	}

	// Look for a string feature that is NOT a string:
	stringFeature, err := repo.GetString("numberfeature")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrInvalidType{}, err)

	// Look for a string feature that DOES exist:
	stringFeature, err = repo.GetString("stringfeature")
	assert.NoError(t, err)
	if assert.NotNil(t, stringFeature) {
		assert.Equal(t, "this is a string", *stringFeature)
	}

	data = `{"key":"booleanfeature","type":"BOOLEAN","value":false,"version":3}`
	anotherFeature, err := featureFromString(data)

	repo.ProcessFeature(anotherFeature)

	booleanFeature, err = repo.GetBoolean("booleanfeature")
	assert.NoError(t, err)
	assert.Equal(t, false, booleanFeature)

}
