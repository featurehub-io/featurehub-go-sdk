package models

import (
	"fmt"
	"math"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/strategies"
	"github.com/spaolacci/murmur3"
)

var (
	maxMurmur32Hash = math.Pow(2, 32)
)

// Strategies so we can attach methods:
type Strategies []Strategy

// Strategy defines model for Strategy.
type Strategy struct {
	Attributes []*StrategyAttribute `json:"attributes"`
	ID         string               `json:"id"`
	Name       string               `json:"name"`
	Percentage float64              `json:"percentage"`
	Value      interface{}          `json:"value,omitempty"` // this value is used if it is a simple attribute or percentage. If it is more complex then the pairs are passed
}

// StrategyAttribute defines a more complex strategy than simple percentages:
type StrategyAttribute struct {
	ID          string        `json:"id"`
	Conditional string        `json:"conditional"`
	FieldName   string        `json:"fieldName"`
	Values      []interface{} `json:"values"`
	Type        string        `json:"type"`
}

// Calculate contains the logic to check each strategy and decide which one applies (if any):
func (ss Strategies) Calculate(clientContext *Context) interface{} {

	// Pre-calculate our hashKey:
	hashKey, _ := clientContext.UniqueKey()

	// Go through the available strategies:
	for _, strategy := range ss {
		logger.Tracef("Checking strategy (%s)", strategy.ID)

		// Check if we match any percentage-based rule:
		if !strategy.proceedWithPercentage(hashKey) {
			logger.Tracef("Failed strategy (%s) percentage - trying next strategy", strategy.ID)
			continue
		}

		// Check if we match the attribute-based rules:
		if !strategy.proceedWithAttributes(clientContext) {
			logger.Tracef("Failed strategy (%s) attributes - trying next strategy", strategy.ID)
			continue
		}

		// If we got this far then we matched this strategy, so we return its value:
		logger.Debugf("Matched strategy (%s:%s)", strategy.ID, strategy.Name)
		return strategy.Value
	}

	// Otherwise just return nil:
	return nil
}

// proceedWithPercentage contains the logic to match percentage-based rules on a user-key / session-key hash:
func (s Strategy) proceedWithPercentage(hashKey string) bool {

	// Make sure we have a percentage rule:
	if s.Percentage == 0 {
		return true
	}

	// If we do have a rule, but don't have a hash-key then we can't continue with this strategy:
	if len(hashKey) == 0 {
		return false
	}

	// Murmur32 sum on the key gives us a consistent number:
	hashedPercentage := float64(murmur3.Sum32([]byte(hashKey))) / maxMurmur32Hash * 1000000

	// If our calculated percentage is less than the strategy percentage then we matched!
	if hashedPercentage <= s.Percentage {
		logger.Tracef("Matched percentage strategy (%s:%f = %v) for calculated percentage: %v\n", s.ID, s.Percentage, s.Value, hashedPercentage)
		return true
	}

	logger.Debugf("Didn't match percentage strategy (%s:%f = %v) for calculated percentage: %v\n", s.ID, s.Percentage, s.Value, hashedPercentage)
	return false
}

// proceedWithPercentage contains the logic to match attribute-based rules on the rest of the client context:
func (s Strategy) proceedWithAttributes(clientContext *Context) bool {

	// We can't continue without a clientContext:
	if clientContext == nil {
		logger.Trace("proceedWithAttributes() Received nil clientContext")
		return false
	}

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
			} else {
				return false
			}
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
