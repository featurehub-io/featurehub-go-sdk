package models

import (
	"testing"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/strategies"
	"github.com/sirupsen/logrus"
	"github.com/spaolacci/murmur3"
	"github.com/stretchr/testify/assert"
)

func TestStrategies(t *testing.T) {
	assert.NotEqual(t, "Strategies are tested from the root module", "/sdks/client-go/streaming_client_with_context_test.go")
}

var TestPercentStrategies = Strategies{
	{
		ID:                   "33",
		Name:                 "33Percent",
		Percentage:           330000,
		Value:                "this is for the 33 percent",
		PercentageAttributes: &[]string{"person-name"},
		Attributes: []*StrategyAttribute{
			{
				ID:          "a9",
				Conditional: strategies.ConditionalEquals,
				FieldName:   "custom-string",
				Values:      []interface{}{"this is a9"},
				Type:        strategies.TypeString,
			},
		},
	},
	{
		ID:         "75",
		Name:       "75Percent",
		Percentage: 750000,
		Value:      "this is for the 75 percent as well but different attributes",
		Attributes: []*StrategyAttribute{
			{
				ID:          "a1",
				Conditional: strategies.ConditionalEquals,
				FieldName:   "other-string",
				Values:      []interface{}{"this is a1"},
				Type:        strategies.TypeString,
			},
		},
	},
	{
		ID:                   "332nd",
		Name:                 "332ndPercent",
		Percentage:           330000,
		PercentageAttributes: &[]string{"person-name"},
		Value:                "this is for the 33 percent as well but different attributes",
		Attributes: []*StrategyAttribute{
			{
				ID:          "a3",
				Conditional: strategies.ConditionalEquals,
				FieldName:   "other-string",
				Values:      []interface{}{"this is a3"},
				Type:        strategies.TypeString,
			},
		},
	},
	{
		ID:         "75",
		Name:       "75Percent",
		Percentage: 750000,
		Value:      "this is the 75 percent fallthrough",
	},
}

// important note for this test, because 33% uses SessionId/UserId, and 75% uses "person-name", they are completely
// different percentage comparisons. 33% DOES NOT add to 75% because their percentage calculations cannot be compared.
func TestCalculateForPercentages(t *testing.T) {
	logger.Level = logrus.TraceLevel
	custom := make(map[string]interface{})
	custom["person-name"] = "sirilak" // field required for 33%

	ctx := &Context{Custom: custom, Session: "ศิริลักษณ์"}
	s := &TestPercentStrategies

	featureId := "p'Korn"

	matched, ok := s.Calculate(ctx, featureId)
	assert.True(t, ok)
	assert.Equal(t, "this is the 75 percent fallthrough", matched)

	custom["other-string"] = "this is a1"
	println("---------------------")
	matched, ok = s.Calculate(ctx, featureId)
	assert.True(t, ok)
	assert.Equal(t, "this is for the 75 percent as well but different attributes", matched)

	custom["other-string"] = "this is a3"
	println("---------------------")
	matched, ok = s.Calculate(ctx, featureId)
	assert.True(t, ok)
	assert.Equal(t, "this is for the 33 percent as well but different attributes", matched)

	custom["custom-string"] = "this is a9"
	println("---------------------")
	matched, ok = s.Calculate(ctx, featureId)
	assert.True(t, ok)
	assert.Equal(t, "this is for the 33 percent", matched)

}

func TestPercentageConversion(t *testing.T) {
	percent := func(text string, id string) int64 {
		return int64(float64(murmur3.Sum32([]byte(text+id))) / maxMurmur32Hash * 1000000)
	}

	assert.Equal(t, int64(212628), percent("fred", "abcde"))
	assert.Equal(t, int64(931882), percent("zappo-food",
		"172765e02-2-1-1-2-2-1"))

	print("%v", percent("ศิริลักษณ์", "p'Korn"))
}

func TestPercentageWithAttributesDoesNotMatchOnAttributesAlone(t *testing.T) {
	SetLogger(logrus.New())

	// a 0.0001% rollout restricted to nz: almost no-one is inside the percentage
	ss := Strategies{
		{
			ID:         "tiny-nz",
			Percentage: 1,
			Value:      "rolled-out",
			Attributes: []*StrategyAttribute{
				{
					ID:          "country",
					Conditional: strategies.ConditionalEquals,
					FieldName:   strategies.FieldNameCountry,
					Values:      []interface{}{"new_zealand"},
					Type:        strategies.TypeString,
				},
			},
		},
	}

	// a user outside the percentage, but in nz
	_, matched := ss.Calculate(&Context{Userkey: "user-outside-percentage", Country: "new_zealand"}, "feature")
	assert.False(t, matched)

	// no user key or session means the percentage can't be evaluated, so it must not match either
	_, matched = ss.Calculate(&Context{Country: "new_zealand"}, "feature")
	assert.False(t, matched)
}
