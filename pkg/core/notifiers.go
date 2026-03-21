package core

import (
	"context"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/errors"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
)

// Notifier ties together a feature, type and callback function:
type notifier struct {
	callbackFuncBoolean models.CallbackFuncBoolean
	callbackFuncJSON    models.CallbackFuncJSON
	callbackFuncNumber  models.CallbackFuncNumber
	callbackFuncString  models.CallbackFuncString
	callbackFuncFeature models.CallbackFuncFeature
	featureKey          string
	uuid                string
}

func (n *notifier) getKey() string {
	return n.featureKey
}

// notify triggers the appropriate callback function for this notifier type:
func (n *notifier) notify(context context.Context, feature *models.FeatureState) error {
	// ask the feature what type it is
	featureValueType := feature.Type

	// they registered a callback wanting the entire feature, not a specific type, so give it to them
	if n.callbackFuncFeature != nil {
		go n.callbackFuncFeature(context, feature)
		return nil
	}

	// Switch on the stored type:
	switch featureValueType {

	case models.TypeBoolean:
		if n.callbackFuncBoolean == nil {
			return errors.NewErrInvalidNotifierCallback("a bool callback function was not added to a bool notifier")
		}

		assertedValue, ok := feature.Value.(bool)
		if !ok {
			return errors.NewErrInvalidType("Unable to assert as bool")
		}
		go n.callbackFuncBoolean(context, assertedValue)

	case models.TypeJSON:
		if n.callbackFuncJSON == nil {
			return errors.NewErrInvalidNotifierCallback("a json callback function was not added to a json notifier")
		}

		assertedValue, ok := feature.Value.(string)
		if !ok {
			return errors.NewErrInvalidType("Unable to assert as string")
		}
		go n.callbackFuncJSON(context, assertedValue)

	case models.TypeNumber:
		if n.callbackFuncNumber == nil {
			return errors.NewErrInvalidNotifierCallback("a number callback function was not added to a number notifier")
		}

		assertedValue, ok := feature.Value.(float64)
		if !ok {
			return errors.NewErrInvalidType("Unable to assert as int64")
		}
		go n.callbackFuncNumber(context, assertedValue)

	case models.TypeString:
		if n.callbackFuncString == nil {
			return errors.NewErrInvalidNotifierCallback("a string callback function was not added to a string notifier")
		}

		assertedValue, ok := feature.Value.(string)
		if !ok {
			return errors.NewErrInvalidType("Unable to assert as string")
		}
		go n.callbackFuncString(context, assertedValue)

	default:
		return errors.NewErrInvalidType(string(featureValueType))
	}

	return nil
}

// notifiers is how they will be arranged in the streaming repository:
type notifiers map[string]map[string]notifier

func (n notifiers) add(newNotifier notifier) {

	// First make sure we have a map for this key:
	featureKey := newNotifier.featureKey
	if _, ok := n[featureKey]; !ok {
		n[featureKey] = make(map[string]notifier)
	}

	// Now that we have a map, add the new notifier:
	n[featureKey][newNotifier.uuid] = newNotifier

	return
}
