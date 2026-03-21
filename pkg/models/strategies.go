package models

import (
	"fmt"
	"math"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/strategies"
	"github.com/google/uuid"
	"github.com/spaolacci/murmur3"
)

var (
	maxMurmur32Hash = math.Pow(2, 32)
)

// Strategies so we can attach methods:
type Strategies []Strategy

type Applied struct {
	Matched bool
	Value   interface{}
}

// Strategy defines model for Strategy.
type Strategy struct {
	Attributes           []*StrategyAttribute `json:"attributes"`
	ID                   string               `json:"id"`
	Name                 string               `json:"name"`
	Percentage           int32                `json:"percentage"` // percentage is at most 1 million
	PercentageAttributes *[]string            `json:"percentageAttributes"`
	Value                interface{}          `json:"value,omitempty"` // this value is used if it is a simple attribute or percentage. If it is more complex then the pairs are passed
}

// StrategyAttribute defines a more complex strategy than simple percentages:
type StrategyAttribute struct {
	ID          string        `json:"id"`
	Conditional string        `json:"conditional"`
	FieldName   string        `json:"fieldName"`
	Values      []interface{} `json:"values"`
	Type        string        `json:"type"`
}

func (ss Strategies) getPercentageAttributes(cc *Context, percentageAttributes *[]string) string {
	if percentageAttributes == nil || len(*percentageAttributes) == 0 {
		if key, ok := cc.UniqueKey(); !ok {
			cc.Session = uuid.New().String()
			println(fmt.Sprintf("no percent attributes, using session and assigning `%s`", cc.Session))
			return cc.Session
		} else {
			println(fmt.Sprintf("no percentage attributes, so using unique key `%s`", key))
			return key
		}
	}

	var pa = ""

	for _, key := range *percentageAttributes {
		if len(pa) > 0 {
			pa += "$"
		}

		pa += cc.ForPercentage(key)
	}

	println(fmt.Sprintf("decoded percentage attribute string is `%s`", pa))

	return pa
}

// Calculate contains the logic to check each strategy and decide which one applies (if any):
func (ss Strategies) Calculate(clientContext *Context, featureId string) (interface{}, bool) {
	if clientContext == nil || len(ss) == 0 {
		println("no context or strategies")
		return nil, false
	}

	var percentage *int32 = nil
	var percentageKey = ""
	basePercentage := map[string]int32{}
	_, hasDefaultPercentageKey := clientContext.UniqueKey()

	// Go through the available strategies
	// remember each strategy can have a different set of percentage attributes, but the total percentage accumulated
	// must be kept as we walk through.
	for _, strategy := range ss {
		logger.Tracef("Checking strategy (%s)", strategy.ID)

		hasNoAttributes := strategy.Attributes == nil || len(strategy.Attributes) == 0

		if strategy.Percentage != 0 {
			hasPercentageKeys := strategy.PercentageAttributes != nil && len(*strategy.PercentageAttributes) > 0

			if hasDefaultPercentageKey || hasPercentageKeys {
				newPercentageKey := ss.getPercentageAttributes(clientContext, strategy.PercentageAttributes) + featureId

				if _, ok := basePercentage[newPercentageKey]; !ok {
					logger.Tracef("strategy: not seen percentage key `%s`, storing new record", newPercentageKey)
					basePercentage[newPercentageKey] = 0
				}

				basePercentageVal := basePercentage[newPercentageKey]

				if percentage == nil || newPercentageKey != percentageKey {
					percentageKey = newPercentageKey
					percentage = new(int32(float64(murmur3.Sum32([]byte(newPercentageKey))) / maxMurmur32Hash * 1000000))

				}

				var useBasePercentage = int32(0)

				if hasNoAttributes {
					useBasePercentage = basePercentageVal
				}

				logger.Tracef("strategy: percentage %v needs to be below %v", *percentage, useBasePercentage+strategy.Percentage)
				if *percentage <= (useBasePercentage + strategy.Percentage) {
					// if we have no attributes, simply being a percentage matched so we return the strategy value
					if hasNoAttributes {
						logger.Trace("matched because no attributes")
						return strategy.Value, true
					}

					// there are attributes, so they have to match as well
					if strategy.proceedWithAttributes(clientContext) {
						logger.Tracef("Matched strategy with percentage (%s:%s) (%v) with matching attributes", strategy.ID, strategy.Name, *percentage)
						return strategy.Value, true
					} else {
						logger.Tracef("Matched percentage but did not match any attributes")
					}
				}

				// if we had attributes, and they matched then we would have returned, but as they don't and the whole
				// criteria didn't match, it didn't increase the percentage value. So we only increase the unconditional
				// percentage value when there are no attributes
				if hasNoAttributes {
					basePercentage[percentageKey] += strategy.Percentage
				}
			}
		}

		// even if it has a percentage, we could match on attributes
		if !hasNoAttributes {
			if strategy.proceedWithAttributes(clientContext) {
				logger.Tracef("Matched strategy (%s:%s)", strategy.ID, strategy.Name)
				return strategy.Value, true
			}
		}
	}

	return nil, false
}

// proceedWithPercentage contains the logic to match attribute-based rules on the rest of the client context:
func (s Strategy) proceedWithAttributes(clientContext *Context) bool {
	for _, sa := range s.Attributes {

		// Handle each different client-context attribute:
		switch sa.FieldName {

		// Match by country name:
		case strategies.FieldNameCountry:
			matched, err := sa.matchType(sa.Values, fmt.Sprintf("%s", clientContext.Country))
			if err != nil {
				logger.WithError(err).Error("Unable to match type")
			}
			if matched {
				continue
			}
			logger.Tracef("Didn't match attribute strategy (%s:%s = %v) for country: %v\n", sa.ID, sa.FieldName, sa.Values, clientContext.Country)
			return false

		// Match by device type:
		case strategies.FieldNameDevice:
			matched, err := sa.matchType(sa.Values, fmt.Sprintf("%s", clientContext.Device))
			if err != nil {
				logger.WithError(err).Error("Unable to match type")
			}
			if matched {
				continue
			}
			logger.Tracef("Didn't match attribute strategy (%s:%s = %v) for device: %v\n", sa.ID, sa.FieldName, sa.Values, clientContext.Device)
			return false

		// Match by platform:
		case strategies.FieldNamePlatform:
			matched, err := sa.matchType(sa.Values, fmt.Sprintf("%s", clientContext.Platform))
			if err != nil {
				logger.WithError(err).Error("Unable to match type")
			}
			if matched {
				continue
			}
			logger.Tracef("Didn't match attribute strategy (%s:%s = %v) for platform: %v\n", sa.ID, sa.FieldName, sa.Values, clientContext.Platform)
			return false

		// Match by userkey:
		case strategies.FieldNameUserkey:
			logger.Trace("Trying userkey")
			matched, err := sa.matchType(sa.Values, fmt.Sprintf("%s", clientContext.Userkey))
			if err != nil {
				logger.WithError(err).Error("Unable to match type")
			}
			if matched {
				continue
			}
			logger.Tracef("Didn't match attribute strategy (%s:%s = %v) for userkey: %v\n", sa.ID, sa.FieldName, sa.Values, clientContext.Userkey)
			return false

		// Match by version:
		case strategies.FieldNameVersion:
			logger.Trace("Trying version")
			matched, err := sa.matchType(sa.Values, fmt.Sprintf("%s", clientContext.Version))
			if err != nil {
				logger.WithError(err).Error("Unable to match type")
			}
			if matched {
				continue
			}
			logger.Tracef("Didn't match attribute strategy (%s:%s = %v) for version: %v\n", sa.ID, sa.FieldName, sa.Values, clientContext.Version)
			return false

		// Custom field:
		default:

			logger.Tracef("Unsupported strategy field (%s), will now try custom strategies", sa.FieldName)

			// Look up the field by name in the clientContext.Custom attribute:
			customContextValue, ok := clientContext.Custom[sa.FieldName]
			if ok {
				matched, err := sa.matchType(sa.Values, customContextValue)
				if err != nil {
					logger.WithError(err).Error("Unable to match type")
				}
				if matched {
					continue
				}
				logger.Tracef("Didn't match custom strategy (%s:%s = %v) for version: %v\n", sa.ID, sa.FieldName, sa.Values, clientContext.Version)
				return false
			}

			return false
		}
	}

	return true
}

// matchType checks the given value against the given slice of options with the attribute's conditional logic:
func (sa *StrategyAttribute) matchType(options []interface{}, value interface{}) (bool, error) {

	// Handle the different conditionals available to us:
	logger.Tracef("Looking for %v within %v", value, options)
	switch sa.Type {

	case strategies.TypeBoolean:
		return strategies.Boolean(sa.Conditional, options, value)

	case strategies.TypeDate:
		return strategies.Date(sa.Conditional, options, value)

	case strategies.TypeDateTime:
		return strategies.DateTime(sa.Conditional, options, value)

	case strategies.TypeIPAddress:
		return strategies.IPAddress(sa.Conditional, options, value)

	case strategies.TypeNumber:
		return strategies.Number(sa.Conditional, options, value)

	case strategies.TypeSemanticVersion:
		return strategies.SemanticVersion(sa.Conditional, options, value)

	case strategies.TypeString:
		return strategies.String(sa.Conditional, options, value)
	}

	// We didn't find it:
	return false, nil
}
