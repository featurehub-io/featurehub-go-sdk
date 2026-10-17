package models

import "context"

// CallbackFuncFeature defines signature used for notifier callback functions:
type CallbackFuncFeature func(context.Context, *FeatureState)

// CallbackFuncBoolean defines signature used for notifier callback functions:
type CallbackFuncBoolean func(context.Context, bool)

// CallbackFuncJSON defines signature used for notifier callback functions:
type CallbackFuncJSON func(context.Context, string)

// CallbackFuncNumber defines signature used for notifier callback functions:
type CallbackFuncNumber func(context.Context, float64)

// CallbackFuncString defines signature used for notifier callback functions:
type CallbackFuncString func(context.Context, string)
