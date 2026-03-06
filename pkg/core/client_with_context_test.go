package core

import (
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/strategies"
	"github.com/stretchr/testify/assert"
)

var TestFeature1States = []*models.FeatureState{
	{
		ID:    "TestFeature1",
		Key:   "TestFeature1",
		Type:  models.TypeString,
		Value: "this is the default value",
		Strategies: []models.Strategy{
			{
				ID:    "s1",
				Name:  "country-thailand",
				Value: "this is for the thais",
				Attributes: []*models.StrategyAttribute{
					{
						ID:          "a1",
						Conditional: strategies.ConditionalEquals,
						FieldName:   strategies.FieldNameCountry,
						Values:      []interface{}{"thailand"},
						Type:        strategies.TypeString,
					},
				},
			},
			{
				ID:    "s2",
				Name:  "platform-unix",
				Value: "this is for unix users",
				Attributes: []*models.StrategyAttribute{
					{
						ID:          "a2",
						Conditional: strategies.ConditionalEquals,
						FieldName:   strategies.FieldNamePlatform,
						Values:      []interface{}{string(models.ContextPlatformLinux), string(models.ContextPlatformMacos)},
						Type:        strategies.TypeString,
					},
				},
			},
			{
				ID:    "s3",
				Name:  "device-notmobile",
				Value: "this is not for mobile users",
				Attributes: []*models.StrategyAttribute{
					{
						ID:          "a3",
						Conditional: strategies.ConditionalNotEquals,
						FieldName:   strategies.FieldNameDevice,
						Values:      []interface{}{string(models.ContextDeviceMobile), string(models.ContextDeviceWatch)},
						Type:        strategies.TypeString,
					},
				},
			},
			{
				ID:    "s3.1",
				Name:  "userkey",
				Value: "this is for userkey ออม",
				Attributes: []*models.StrategyAttribute{
					{
						ID:          "a3.1",
						Conditional: strategies.ConditionalEquals,
						FieldName:   strategies.FieldNameUserkey,
						Values:      []interface{}{"ออม"}, // ensure international characters work
						Type:        strategies.TypeString,
					},
				},
			},
			{
				ID:    "s4",
				Name:  "version-less",
				Value: "version less than 15.23.4",
				Attributes: []*models.StrategyAttribute{
					{
						ID:          "a4",
						Conditional: strategies.ConditionalLess,
						FieldName:   strategies.FieldNameVersion,
						Values:      []interface{}{"15.23.4"},
						Type:        strategies.TypeSemanticVersion,
					},
				},
			},
			{
				ID:    "s5",
				Name:  "version-lessequals",
				Value: "version less than or equal to 15.23.4",
				Attributes: []*models.StrategyAttribute{
					{
						ID:          "a5",
						Conditional: strategies.ConditionalLessEquals,
						FieldName:   strategies.FieldNameVersion,
						Values:      []interface{}{"15.23.4"},
						Type:        strategies.TypeSemanticVersion,
					},
				},
			},
			{
				ID:    "s6",
				Name:  "version-greater",
				Value: "version greater than 16.0.0",
				Attributes: []*models.StrategyAttribute{
					{
						ID:          "a6",
						Conditional: strategies.ConditionalGreater,
						FieldName:   strategies.FieldNameVersion,
						Values:      []interface{}{"16.0.0"},
						Type:        strategies.TypeSemanticVersion,
					},
				},
			},
			{
				ID:    "s7",
				Name:  "version-greaterequals",
				Value: "version greater than or equal to 16.0.0",
				Attributes: []*models.StrategyAttribute{
					{
						ID:          "a7",
						Conditional: strategies.ConditionalGreaterEquals,
						FieldName:   strategies.FieldNameVersion,
						Values:      []interface{}{"16.0.0"},
						Type:        strategies.TypeSemanticVersion,
					},
				},
			},
			{
				ID:    "s8",
				Name:  "custom-bool",
				Value: "you have the custom bool",
				Attributes: []*models.StrategyAttribute{
					{
						ID:          "a8",
						Conditional: strategies.ConditionalEquals,
						FieldName:   "custom-bool",
						Values:      []interface{}{true},
						Type:        strategies.TypeBoolean,
					},
				},
			},
			{
				ID:    "s9",
				Name:  "custom-string",
				Value: "you have the custom string",
				Attributes: []*models.StrategyAttribute{
					{
						ID:          "a9",
						Conditional: strategies.ConditionalEquals,
						FieldName:   "custom-string",
						Values:      []interface{}{"this is it"},
						Type:        strategies.TypeString,
					},
				},
			},
		},
	},
	{
		ID:    "TestFeature2",
		Key:   "TestFeature2",
		Type:  models.TypeString,
		Value: "this is the default value",
		Strategies: []models.Strategy{
			{
				ID:         "33",
				Name:       "33Percent",
				Percentage: 330000,
				Value:      "this is for the 33 percent",
			},
			{
				ID:         "66",
				Name:       "66Percent",
				Percentage: 660000,
				Value:      "this is for the 66 percent",
			},
		},
	},
	{
		ID:    "TestFeature3",
		Key:   "TestBoolean",
		Type:  models.TypeBoolean,
		Value: true,
	},
	{
		ID:    "TestFeature4",
		Key:   "TestJSON",
		Type:  models.TypeJSON,
		Value: `{"test": "something"}`,
	},
	{
		ID:    "TestFeature5",
		Key:   "TestNumber",
		Type:  models.TypeNumber,
		Value: float64(54321),
	},
	{
		ID:    "TestFeature6",
		Key:   "TestString",
		Type:  models.TypeString,
		Value: "this is another string",
	},
}

func TestClientWithContext(t *testing.T) {
	// Use the config to make a new StreamingClient with a mock apiClient::
	testClient := createClient()

	// Make a repository context:
	testContext := &models.Context{
		Userkey: "TestClientWithContext",
	}

	// Make sure our repository and context are present:
	clientWithContext := testClient.WithContext(testContext)
	// the repository in the context is the same as the client
	assert.Equal(t, testClient, clientWithContext.Repository)

	assert.Equal(t, testContext, clientWithContext.Attributes())
	assert.Equal(t, testClient, clientWithContext.Repository())
	assert.Implements(t, new(interfaces.Repository), testClient)

	// Try getting a new repository with a replaced context:
	replacementContext := &models.Context{
		Userkey: "TestClientWithContext",
		Country: "New Zealand",
	}
	replacementClient := clientWithContext.WithContext(replacementContext)
	assert.Equal(t, replacementContext, replacementClient.Attributes())

	testClient.ProcessFeatures(TestFeature1States)

	// First make sure that we get the default value before repository-context is added:
	stringValue, err := testClient.GetString("TestFeature1")
	assert.NoError(t, err)
	assert.Equal(t, "this is the default value", stringValue)

	// See if we can match the "country-thailand" attribute:
	stringValue, err = testClient.
		WithContext(&models.Context{Country: models.ContextCountryThailand}).
		GetString("TestFeature1")
	assert.Equal(t, "this is for the thais", stringValue)
	assert.NoError(t, err)

	// See if we can match the "platform-unix" attribute:
	stringValue, err = testClient.
		WithContext(&models.Context{Platform: models.ContextPlatformMacos}).
		GetString("TestFeature1")
	assert.Equal(t, "this is for unix users", stringValue)
	assert.NoError(t, err)

	// See if we can match the "device-notmobile" attribute:
	stringValue, err = testClient.
		WithContext(&models.Context{Device: models.ContextDeviceServer}).
		GetString("TestFeature1")
	assert.Equal(t, "this is not for mobile users", stringValue)
	assert.NoError(t, err)

	// See if we can match the "userkey" attribute:
	stringValue, err = testClient.
		WithContext(&models.Context{Userkey: "ออม"}).
		GetString("TestFeature1")
	assert.Equal(t, "this is for userkey ออม", stringValue)
	assert.NoError(t, err)

	// See if we can match the "version-less" attribute:
	stringValue, err = testClient.
		WithContext(&models.Context{Version: "5.6.7"}).
		GetString("TestFeature1")
	assert.Equal(t, "version less than 15.23.4", stringValue)
	assert.NoError(t, err)

	// See if we can match the "version-lessequal" attribute:
	stringValue, err = testClient.
		WithContext(&models.Context{Version: "15.23.4"}).
		GetString("TestFeature1")
	assert.Equal(t, "version less than or equal to 15.23.4", stringValue)
	assert.NoError(t, err)

	// See if we can match the "version-greater" attribute:
	stringValue, err = testClient.
		WithContext(&models.Context{Version: "16.0.1"}).
		GetString("TestFeature1")
	assert.Equal(t, "version greater than 16.0.0", stringValue)
	assert.NoError(t, err)

	// See if we can match the "version-greaterequals" attribute:
	stringValue, err = testClient.
		WithContext(&models.Context{Version: "16.0.0"}).
		GetString("TestFeature1")
	assert.Equal(t, "version greater than or equal to 16.0.0", stringValue)
	assert.NoError(t, err)

	// Look for a 33% rule (based on a pre-calculated hash):
	stringValue, err = testClient.
		WithContext(&models.Context{Userkey: "1111111111"}).
		GetString("TestFeature2")
	assert.Equal(t, "this is for the 33 percent", stringValue)
	assert.NoError(t, err)

	// Look for a 66% rule (based on a pre-calculated hash):
	stringValue, err = testClient.
		WithContext(&models.Context{
			Userkey: "1111111111",
			Session: "4444444444",
		}).
		GetString("TestFeature2")
	assert.Equal(t, "this is for the 66 percent", stringValue)
	assert.NoError(t, err)

	// Get a default boolean value:
	booleanValue, err := testClient.
		WithContext(&models.Context{Userkey: time.Now().String()}).
		GetBoolean("TestBoolean")
	assert.NoError(t, err)
	assert.Equal(t, true, booleanValue)

	// Get a default json value:
	jsonValue, err := testClient.
		WithContext(&models.Context{Userkey: time.Now().String()}).
		GetRawJSON("TestJSON")
	assert.NoError(t, err)
	assert.Equal(t, `{"test": "something"}`, jsonValue)

	// Get a default number value:
	numberValue, err := testClient.
		WithContext(&models.Context{Userkey: time.Now().String()}).
		GetNumber("TestNumber")
	assert.NoError(t, err)
	assert.Equal(t, float64(54321), numberValue)

	// Get a default string value:
	stringValue, err = testClient.
		WithContext(&models.Context{Userkey: time.Now().String()}).
		GetString("TestString")
	assert.NoError(t, err)
	assert.Equal(t, "this is another string", stringValue)

	// See if we can match the "custom-bool" attribute:
	stringValue, err = testClient.
		WithContext(&models.Context{Custom: map[string]interface{}{"custom-bool": true}}).
		GetString("TestFeature1")
	assert.Equal(t, "you have the custom bool", stringValue)
	assert.NoError(t, err)

	// See if we can match the "custom-string" attribute:
	stringValue, err = testClient.
		WithContext(&models.Context{Custom: map[string]interface{}{"custom-string": "this is it"}}).
		GetString("TestFeature1")
	assert.Equal(t, "you have the custom string", stringValue)
	assert.NoError(t, err)
}
