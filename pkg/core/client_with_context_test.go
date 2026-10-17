package core

import (
	"context"
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/strategies"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		ID:    "p'Korn",
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
	repository := createRepository()

	// Make a repository context:
	testContext := &models.Context{
		Userkey: "TestClientWithContext",
	}

	// Make sure our repository and context are present:
	clientWithContext := repository.WithContext(testContext)
	// the repository in the context is the same as the client
	assert.Equal(t, testContext, clientWithContext.Attributes())

	assert.Implements(t, new(interfaces.RepositoryContext), repository)

	// Try getting a new repository with a replaced context:
	replacementContext := &models.Context{
		Userkey: "TestClientWithContext",
		Country: "New Zealand",
	}
	replacementClient := clientWithContext.WithContext(replacementContext)
	assert.Equal(t, replacementContext, replacementClient.Attributes())

	repository.ProcessFeatures(TestFeature1States)

	// derefString unwraps a (*string, error) return: asserts no error, non-nil, and returns the string value.
	derefString := func(s *string, err error) string {
		t.Helper()
		assert.NoError(t, err)
		if s == nil {
			t.Fatal("unexpected nil *string")
		}
		return *s
	}

	// derefNumber unwraps a (*float64, error) return.
	derefNumber := func(n *float64, err error) float64 {
		t.Helper()
		assert.NoError(t, err)
		if n == nil {
			t.Fatal("unexpected nil *float64")
		}
		return *n
	}

	// First make sure that we get the default value before repository-context is added:
	assert.Equal(t, "this is the default value", derefString(repository.GetString(context.TODO(), "TestFeature1")))

	// See if we can match the "country-thailand" attribute:
	assert.Equal(t, "this is for the thais",
		derefString(repository.WithContext(&models.Context{Country: models.ContextCountryThailand}).GetString(context.TODO(), "TestFeature1")))

	// See if we can match the "platform-unix" attribute:
	assert.Equal(t, "this is for unix users",
		derefString(repository.WithContext(&models.Context{Platform: models.ContextPlatformMacos}).GetString(context.TODO(), "TestFeature1")))

	// See if we can match the "device-notmobile" attribute:
	assert.Equal(t, "this is not for mobile users",
		derefString(repository.WithContext(&models.Context{Device: models.ContextDeviceServer}).GetString(context.TODO(), "TestFeature1")))

	// See if we can match the "userkey" attribute:
	assert.Equal(t, "this is for userkey ออม",
		derefString(repository.WithContext(&models.Context{Userkey: "ออม"}).GetString(context.TODO(), "TestFeature1")))

	// See if we can match the "version-less" attribute:
	assert.Equal(t, "version less than 15.23.4",
		derefString(repository.WithContext(&models.Context{Version: "5.6.7"}).GetString(context.TODO(), "TestFeature1")))

	// See if we can match the "version-lessequal" attribute:
	assert.Equal(t, "version less than or equal to 15.23.4",
		derefString(repository.WithContext(&models.Context{Version: "15.23.4"}).GetString(context.TODO(), "TestFeature1")))

	// See if we can match the "version-greater" attribute:
	assert.Equal(t, "version greater than 16.0.0",
		derefString(repository.WithContext(&models.Context{Version: "16.0.1"}).GetString(context.TODO(), "TestFeature1")))

	// See if we can match the "version-greaterequals" attribute:
	assert.Equal(t, "version greater than or equal to 16.0.0",
		derefString(repository.WithContext(&models.Context{Version: "16.0.0"}).GetString(context.TODO(), "TestFeature1")))

	// Look for a 33% rule (based on a pre-calculated hash):
	assert.Equal(t, "this is for the 33 percent",
		derefString(repository.WithContext(&models.Context{Userkey: "อ้วม"}).GetString(context.TODO(), "TestFeature2")))

	// Look for a 66% rule (based on a pre-calculated hash):
	assert.Equal(t, "this is for the 66 percent",
		derefString(repository.WithContext(&models.Context{Userkey: "1111111111", Session: "ศิริลักษณ์"}).GetString(context.TODO(), "TestFeature2")))

	// Get a default boolean value:
	booleanValue, err := repository.
		WithContext(&models.Context{Userkey: time.Now().String()}).
		GetBoolean(context.TODO(), "TestBoolean")
	assert.NoError(t, err)
	assert.Equal(t, true, booleanValue)

	// Get a default json value:
	assert.Equal(t, `{"test": "something"}`,
		derefString(repository.WithContext(&models.Context{Userkey: time.Now().String()}).GetRawJSON(context.TODO(), "TestJSON")))

	// Get a default number value:
	assert.Equal(t, float64(54321),
		derefNumber(repository.WithContext(&models.Context{Userkey: time.Now().String()}).GetNumber(context.TODO(), "TestNumber")))

	// Get a default string value:
	assert.Equal(t, "this is another string",
		derefString(repository.WithContext(&models.Context{Userkey: time.Now().String()}).GetString(context.TODO(), "TestString")))

	// See if we can match the "custom-bool" attribute:
	assert.Equal(t, "you have the custom bool",
		derefString(repository.WithContext(&models.Context{Custom: map[string]interface{}{"custom-bool": true}}).GetString(context.TODO(), "TestFeature1")))

	// See if we can match the "custom-string" attribute:
	assert.Equal(t, "you have the custom string",
		derefString(repository.WithContext(&models.Context{Custom: map[string]interface{}{"custom-string": "this is it"}}).GetString(context.TODO(), "TestFeature1")))
}

// --- Usage event emission ---

// captureUsageEvents registers a stream handler on repo and returns a slice that
// accumulates every BaseWithFeature event emitted synchronously during the test.
func captureUsageEvents(repo *ClientFeatureHubRepository) *[]*usage.BaseWithFeature {
	var events []*usage.BaseWithFeature
	repo.RegisterUsageStream(func(_ context.Context, event usage.UsageEvent) {
		if e, ok := event.(*usage.BaseWithFeature); ok {
			events = append(events, e)
		}
	})
	return &events
}

func TestGetBooleanEmitsUsageEventWithDefaultValue(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeatures([]*models.FeatureState{
		ffs(`{"key":"flag","type":"BOOLEAN","value":true,"id":"id-flag","version":1}`),
	})
	events := captureUsageEvents(repo)

	ctx := repo.WithContext(&models.Context{Userkey: "alice"})
	val, err := ctx.GetBoolean(context.TODO(), "flag")

	require.NoError(t, err)
	assert.True(t, val)
	require.Len(t, *events, 1)
	assert.Equal(t, "flag", (*events)[0].Feature().Key)
	assert.Equal(t, "on", (*events)[0].Feature().Value)
	assert.Equal(t, "alice", (*events)[0].UserKey())
}

func TestGetNumberEmitsUsageEventWithDefaultValue(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeatures([]*models.FeatureState{
		ffs(`{"key":"count","type":"NUMBER","value":42,"id":"id-count","version":1}`),
	})
	events := captureUsageEvents(repo)

	ctx := repo.WithContext(&models.Context{Userkey: "bob"})
	val, err := ctx.GetNumber(context.TODO(), "count")

	require.NoError(t, err)
	require.NotNil(t, val)
	assert.Equal(t, float64(42), *val)
	require.Len(t, *events, 1)
	assert.Equal(t, "count", (*events)[0].Feature().Key)
	assert.Equal(t, "bob", (*events)[0].UserKey())
}

func TestGetStringEmitsUsageEventWithDefaultValue(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeatures([]*models.FeatureState{
		ffs(`{"key":"label","type":"STRING","value":"hello","id":"id-label","version":1}`),
	})
	events := captureUsageEvents(repo)

	ctx := repo.WithContext(&models.Context{Userkey: "carol"})
	val, err := ctx.GetString(context.TODO(), "label")

	require.NoError(t, err)
	require.NotNil(t, val)
	assert.Equal(t, "hello", *val)
	require.Len(t, *events, 1)
	assert.Equal(t, "label", (*events)[0].Feature().Key)
	assert.Equal(t, "carol", (*events)[0].UserKey())
}

func TestGetRawJSONEmitsUsageEvent(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeatures([]*models.FeatureState{
		ffs(`{"key":"cfg","type":"JSON","value":"{\"x\":1}","id":"id-cfg","version":1}`),
	})
	events := captureUsageEvents(repo)

	ctx := repo.WithContext(&models.Context{Userkey: "dave"})
	_, err := ctx.GetRawJSON(context.TODO(), "cfg")

	require.NoError(t, err)
	require.Len(t, *events, 1)
	assert.Equal(t, "cfg", (*events)[0].Feature().Key)
}

func TestStrategyMatchEmitsUsageEventWithStrategyValue(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeatures([]*models.FeatureState{
		{
			ID:    "id-f1",
			Key:   "TestFeature1",
			Type:  models.TypeString,
			Value: "default",
			Strategies: []models.Strategy{
				{
					ID:    "s1",
					Value: "for-thailand",
					Attributes: []*models.StrategyAttribute{
						{
							Conditional: strategies.ConditionalEquals,
							FieldName:   strategies.FieldNameCountry,
							Values:      []interface{}{"thailand"},
							Type:        strategies.TypeString,
						},
					},
				},
			},
		},
	})
	events := captureUsageEvents(repo)

	ctx := repo.WithContext(&models.Context{Userkey: "eve", Country: models.ContextCountryThailand})
	val, err := ctx.GetString(context.TODO(), "TestFeature1")

	require.NoError(t, err)
	require.NotNil(t, val)
	assert.Equal(t, "for-thailand", *val)
	require.Len(t, *events, 1)
	assert.Equal(t, "for-thailand", (*events)[0].Feature().Value)
	assert.Equal(t, "eve", (*events)[0].UserKey())
}

func TestFeatureNotFoundDoesNotEmitUsageEvent(t *testing.T) {
	repo := createRepository()
	events := captureUsageEvents(repo)

	ctx := repo.WithContext(&models.Context{Userkey: "frank"})
	_, err := ctx.GetBoolean(context.TODO(), "absent")

	assert.Error(t, err)
	assert.Empty(t, *events)
}

func TestInterceptorMatchEmitsUsageEventWhenFeatureExists(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeatures([]*models.FeatureState{
		ffs(`{"key":"flag","type":"BOOLEAN","value":false,"id":"id-flag","version":1}`),
	})
	repo.AddValueInterceptor(interceptorThatMatches(true))
	events := captureUsageEvents(repo)

	ctx := repo.WithContext(&models.Context{Userkey: "grace"})
	val, err := ctx.GetBoolean(context.TODO(), "flag")

	require.NoError(t, err)
	assert.True(t, val)
	require.Len(t, *events, 1, "interceptor match on an existing feature must emit a usage event")
	assert.Equal(t, "flag", (*events)[0].Feature().Key)
	assert.Equal(t, "grace", (*events)[0].UserKey())
}

func TestInterceptorMatchOnUnknownFeatureDoesNotEmitUsageEvent(t *testing.T) {
	repo := createRepository()
	// No features loaded — interceptor provides a value for an unknown key.
	repo.AddValueInterceptor(interceptorThatMatches(true))
	events := captureUsageEvents(repo)

	ctx := repo.WithContext(&models.Context{Userkey: "heidi"})
	val, err := ctx.GetBoolean(context.TODO(), "absent")

	require.NoError(t, err)
	assert.True(t, val)
	assert.Empty(t, *events, "interceptor match on an unknown feature must not emit a usage event")
}

// --- Properties ---

func TestClientWithContextPropertiesReturnsNilForUnknownFeature(t *testing.T) {
	repo := createRepository()
	ctx := repo.WithContext(&models.Context{})

	assert.Nil(t, ctx.Properties(context.TODO(), "does-not-exist"))
}

func TestClientWithContextPropertiesReturnsNilWhenFeatureHasNoProperties(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeature(ffs(`{"id":"id-myfeature","key":"myfeature","type":"BOOLEAN","value":true,"version":1}`))
	ctx := repo.WithContext(&models.Context{})

	assert.Nil(t, ctx.Properties(context.TODO(), "myfeature"))
}

func TestClientWithContextPropertiesReturnsMapFromUnderlyingFeature(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeature(ffs(`{"id":"id-myfeature","key":"myfeature","type":"STRING","value":"hello","version":1,"fp":{"env":"prod","tier":"gold"}}`))
	ctx := repo.WithContext(&models.Context{})

	result := ctx.Properties(context.TODO(), "myfeature")
	assert.Equal(t, map[string]string{"env": "prod", "tier": "gold"}, result)
}

func TestClientWithContextPropertiesIsConsistentAcrossContextSwitch(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeature(ffs(`{"id":"id-myfeature","key":"myfeature","type":"STRING","value":"v","version":1,"fp":{"k":"v"}}`))

	ctx1 := repo.WithContext(&models.Context{Userkey: "user1"})
	ctx2 := ctx1.WithContext(&models.Context{Userkey: "user2"})

	assert.Equal(t, ctx1.Properties(context.TODO(), "myfeature"), ctx2.Properties(context.TODO(), "myfeature"))
}
