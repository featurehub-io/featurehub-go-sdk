package core

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
	"github.com/stretchr/testify/assert"
)

func TestRepositoryConvenienceMethodsReturnFeatureValues(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeatures([]*models.FeatureState{
		{ID: "1", Key: "bool", Type: models.TypeBoolean, Value: true, Version: 1},
		{ID: "2", Key: "num", Type: models.TypeNumber, Value: float64(42), Version: 1},
		{ID: "3", Key: "str", Type: models.TypeString, Value: "hello", Version: 1},
		{ID: "4", Key: "json", Type: models.TypeJSON, Value: `{"a":1}`, Version: 1},
	})
	ctx := context.Background()

	assert.True(t, repo.Boolean(ctx, "bool", false))
	assert.Equal(t, float64(42), repo.Number(ctx, "num", 1))
	assert.Equal(t, "hello", repo.String(ctx, "str", "default"))
	assert.Equal(t, `{"a":1}`, repo.JSON(ctx, "json", "{}"))

	// missing keys fall back to the default
	assert.True(t, repo.Boolean(ctx, "missing", true))
	assert.Equal(t, float64(7), repo.Number(ctx, "missing", 7))
	assert.Equal(t, "default", repo.String(ctx, "missing", "default"))
	assert.Equal(t, "{}", repo.JSON(ctx, "missing", "{}"))
}

func TestReadinessListenerCanReadFeaturesOnSingleFeatureUpdate(t *testing.T) {
	repo := createRepository()

	done := make(chan bool, 1)
	repo.ReadinessListener(context.Background(), func(ctx context.Context) {
		done <- repo.Boolean(ctx, "bool", false)
	})

	go repo.ProcessFeature(&models.FeatureState{ID: "1", Key: "bool", Type: models.TypeBoolean, Value: true, Version: 1})

	select {
	case value := <-done:
		assert.True(t, value)
	case <-time.After(2 * time.Second):
		t.Fatal("deadlocked: readiness listener could not read features from within ProcessFeature")
	}
}

func TestUsageStreamsCanChangeWhileEmitting(t *testing.T) {
	repo := createRepository()
	repo.RegisterUsageStream(func(ctx context.Context, event usage.UsageEvent) {})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			repo.EmitUsageEvent(context.Background(), nil)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			repo.RemoveUsageStream(repo.RegisterUsageStream(func(ctx context.Context, event usage.UsageEvent) {}))
		}
	}()
	wg.Wait()
}

func TestAsConvertibleStringForInterceptedKeyUnknownToServer(t *testing.T) {
	repo := createRepository()
	repo.AddValueInterceptor(interfaces.FeatureValueInterceptor{
		Intercept: func(ctx context.Context, key string, r interfaces.FeatureRepository, feature *models.FeatureState) (interface{}, bool) {
			switch key {
			case "newFlag":
				return true, true
			case "newNumber":
				return 12, true
			case "newString":
				return "hi", true
			}
			return nil, false
		},
	})

	cc := repo.WithContext(&models.Context{})
	ctx := context.Background()

	s, err := cc.AsConvertibleString(ctx, "newFlag")
	assert.NoError(t, err)
	assert.Equal(t, "t", s)

	s, err = cc.AsConvertibleString(ctx, "newNumber")
	assert.NoError(t, err)
	assert.Equal(t, "12", s)

	s, err = cc.AsConvertibleString(ctx, "newString")
	assert.NoError(t, err)
	assert.Equal(t, "hi", s)
}

func TestNewContextBeforeConnectDoesNotPanic(t *testing.T) {
	cfg := NewConfig("http://localhost", "env/key", nil)

	assert.NotPanics(t, func() {
		assert.True(t, cfg.NewContext().Boolean(context.Background(), "missing", true))
		assert.True(t, cfg.WithContext(&models.Context{}).Boolean(context.Background(), "missing", true))
	})
}
