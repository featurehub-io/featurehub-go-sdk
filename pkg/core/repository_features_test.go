package core

import (
	"bytes"
	"context"
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

	var data = `[{"id":"id-bool","key":"booleanfeature","type":"BOOLEAN","value":true,"version":1},{"id":"id-json","key":"jsonfeature","type":"JSON","value":"{\"is_crufty\": true}","version":1},{"id":"id-num","key":"numberfeature","type":"NUMBER","value":123456789,"version":1},{"id":"id-str","key":"stringfeature","type":"STRING","value":"this is a string","version":1}]`
	features, err := featuresFromString(data)
	assert.NoError(t, err)

	repo.ProcessFeatures(features)

	// Look for a feature that doesn't exist:
	_, _, _, err = repo.GetFeature(context.TODO(), "something-that-does-not-exist")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrFeatureNotFound{}, err)

	// Look for a feature that DOES exist:
	feature, matched, value, err := repo.GetFeature(context.TODO(), "stringfeature")
	assert.NoError(t, err)
	assert.Equal(t, models.FeatureValueType("STRING"), feature.Type)
	assert.Equal(t, false, matched)
	assert.Equal(t, "this is a string", value)

	// Look for a boolean feature that is NOT a boolean:
	booleanFeature, err := repo.GetBoolean(context.TODO(), "stringfeature")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrInvalidType{}, err)

	// Look for a boolean feature that IS a boolean:
	booleanFeature, err = repo.GetBoolean(context.TODO(), "booleanfeature")
	assert.NoError(t, err)
	assert.Equal(t, true, booleanFeature)

	// Look for a JSON feature that is NOT JSON:
	jsonFeature, err := repo.GetRawJSON(context.TODO(), "numberfeature")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrInvalidType{}, err)

	// Look for a JSON feature that IS JSON:
	jsonFeature, err = repo.GetRawJSON(context.TODO(), "jsonfeature")
	assert.NoError(t, err)
	if assert.NotNil(t, jsonFeature) {
		assert.Equal(t, `{"is_crufty": true}`, *jsonFeature)
	}

	// Look for a number feature that is NOT a number:
	numberFeature, err := repo.GetNumber(context.TODO(), "stringfeature")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrInvalidType{}, err)

	// Look for a number feature that IS a number:
	numberFeature, err = repo.GetNumber(context.TODO(), "numberfeature")
	assert.NoError(t, err)
	if assert.NotNil(t, numberFeature) {
		assert.Equal(t, float64(123456789), *numberFeature)
	}

	// Look for a string feature that is NOT a string:
	stringFeature, err := repo.GetString(context.TODO(), "numberfeature")
	assert.Error(t, err)
	assert.IsType(t, &errors.ErrInvalidType{}, err)

	// Look for a string feature that DOES exist:
	stringFeature, err = repo.GetString(context.TODO(), "stringfeature")
	assert.NoError(t, err)
	if assert.NotNil(t, stringFeature) {
		assert.Equal(t, "this is a string", *stringFeature)
	}

	data = `{"id":"id-bool","key":"booleanfeature","type":"BOOLEAN","value":false,"version":3}`
	anotherFeature, err := featureFromString(data)

	repo.ProcessFeature(anotherFeature)

	booleanFeature, err = repo.GetBoolean(context.TODO(), "booleanfeature")
	assert.NoError(t, err)
	assert.Equal(t, false, booleanFeature)

}

func TestPropertiesReturnsNilForUnknownFeature(t *testing.T) {
	repo := createRepository()

	result := repo.Properties(context.TODO(), "does-not-exist")
	assert.Nil(t, result)
}

func TestPropertiesReturnsNilWhenFeatureHasNoProperties(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeature(ffs(`{"id":"id-myfeature","key":"myfeature","type":"BOOLEAN","value":true,"version":1}`))

	result := repo.Properties(context.TODO(), "myfeature")
	assert.Nil(t, result)
}

func TestPropertiesReturnsMapWhenFeatureHasProperties(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeature(ffs(`{"id":"id-myfeature","key":"myfeature","type":"STRING","value":"hello","version":1,"fp":{"color":"red","size":"large"}}`))

	result := repo.Properties(context.TODO(), "myfeature")
	assert.Equal(t, map[string]string{"color": "red", "size": "large"}, result)
}

func TestPropertiesReturnsEmptyMapForFeatureWithEmptyProperties(t *testing.T) {
	repo := createRepository()
	feature := &models.FeatureState{
		ID:         "id-myfeature",
		Key:        "myfeature",
		Type:       models.TypeString,
		Value:      "hello",
		Version:    1,
		Properties: map[string]string{},
	}
	repo.ProcessFeature(feature)

	result := repo.Properties(context.TODO(), "myfeature")
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestWhenKeyChangesCorrectPropertyIsStillUpdated(t *testing.T) {
	repo := createRepository()
	feature := &models.FeatureState{
		ID:      "id-myfeature",
		Key:     "myfeature",
		Type:    models.TypeString,
		Value:   "hello",
		Version: 1,
	}
	repo.ProcessFeature(feature)
	// we can find the feature by key
	f, _, _, fErr := repo.GetFeature(context.TODO(), "myfeature")
	assert.NoError(t, fErr)
	assert.NotNil(t, f)
	assert.Equal(t, feature.ID, f.ID)

	featureChangedKey := &models.FeatureState{
		ID:      "id-myfeature",
		Key:     "หลิงหลิง",
		Type:    models.TypeString,
		Value:   "hello",
		Version: 1,
	}

	repo.ProcessFeature(featureChangedKey)
	f, _, _, fErr = repo.GetFeature(context.TODO(), "myfeature")
	assert.Error(t, fErr)
	assert.Nil(t, f)
	f, _, _, fErr = repo.GetFeature(context.TODO(), "หลิงหลิง")
	assert.NoError(t, fErr)
	assert.NotNil(t, f)
	assert.Equal(t, feature.ID, f.ID)

	// lower version number, changed key
	featureChanged2Key := &models.FeatureState{
		ID:      "id-myfeature",
		Key:     "鄺玲玲",
		Type:    models.TypeString,
		Value:   "hello",
		Version: 0,
	}

	repo.ProcessFeature(featureChanged2Key)
	// should fail to find as it has a lower version no
	f, _, _, fErr = repo.GetFeature(context.TODO(), "鄺玲玲")
	assert.Error(t, fErr)
	assert.Nil(t, f)
}

func TestFeatureDeletesWhenKeyHasChanged(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeature(ffs(`{"id":"id-stringfeature","key":"หลิงหลิง","type":"STRING","value":"this is a string","version":1}`))

	f, _, _, fErr := repo.GetFeature(context.TODO(), "หลิงหลิง")
	assert.NoError(t, fErr)
	assert.NotNil(t, f)
	repo.ProcessDeleteFeature(ffs(`{"id":"id-stringfeature","key":"鄺玲玲","type":"STRING","value":"this is a string","version":1}`))
	f, _, _, fErr = repo.GetFeature(context.TODO(), "หลิงหลิง")
	assert.Nil(t, f)
	assert.Error(t, fErr)
	_, _, _, fErr = repo.GetFeature(context.TODO(), "鄺玲玲")
	assert.Error(t, fErr)
}
