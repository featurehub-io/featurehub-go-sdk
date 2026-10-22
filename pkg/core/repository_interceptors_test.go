package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helpers

// seedRepo loads a repository with a minimal feature set from JSON.
func seedRepo(repo *ClientFeatureHubRepository) {
	features := []*models.FeatureState{
		ffs(`{"id":"id-flag","key":"flag","type":"BOOLEAN","value":false,"version":1}`),
		ffs(`{"id":"id-count","key":"count","type":"NUMBER","value":42,"version":1}`),
		ffs(`{"id":"id-label","key":"label","type":"STRING","value":"default","version":1}`),
	}
	repo.ProcessFeatures(features)
}

// interceptorThatMatches returns an interceptor that always matches with the given value.
func interceptorThatMatches(value interface{}) interfaces.FeatureValueInterceptor {
	return interfaces.NewInterceptor(func(_ context.Context, _ string, _ interfaces.FeatureRepository, _ *models.FeatureState) (interface{}, bool) {
		return value, true
	})
}

// interceptorThatMisses returns an interceptor that never matches.
func interceptorThatMisses() interfaces.FeatureValueInterceptor {
	return interfaces.NewInterceptor(func(_ context.Context, _ string, _ interfaces.FeatureRepository, _ *models.FeatureState) (interface{}, bool) {
		return nil, false
	})
}

// --- AddValueInterceptor ---

func TestAddValueInterceptorAppendsToSlice(t *testing.T) {
	repo := createRepository()
	assert.Nil(t, repo.interceptors())

	repo.AddValueInterceptor(interceptorThatMisses())
	assert.Len(t, repo.interceptors(), 1)

	repo.AddValueInterceptor(interceptorThatMisses())
	assert.Len(t, repo.interceptors(), 2)
}

// --- GetFeature: interceptor behaviour ---

func TestInterceptorOverridesFeatureValue(t *testing.T) {
	repo := createRepository()
	seedRepo(repo)
	repo.AddValueInterceptor(interceptorThatMatches(true))

	_, matched, value, err := repo.GetFeature(context.TODO(), "flag")

	require.NoError(t, err)
	assert.True(t, matched, "matched should be true when interceptor fires")
	assert.Equal(t, true, value, "value should be the interceptor's override")
}

func TestInterceptorNoMatchFallsThroughToFeatureValue(t *testing.T) {
	repo := createRepository()
	seedRepo(repo)
	repo.AddValueInterceptor(interceptorThatMisses())

	_, matched, value, err := repo.GetFeature(context.TODO(), "flag")

	require.NoError(t, err)
	assert.False(t, matched, "matched should be false when no interceptor fires")
	assert.Equal(t, false, value, "value should be the stored feature value")
}

func TestInterceptorCalledWhenFeatureNotInRepository(t *testing.T) {
	repo := createRepository()
	// No features loaded; interceptor provides value for unknown key.
	repo.AddValueInterceptor(interceptorThatMatches("injected"))

	_, matched, value, err := repo.GetFeature(context.TODO(), "unknown-key")

	require.NoError(t, err, "no error when interceptor matches an unknown feature")
	assert.True(t, matched)
	assert.Equal(t, "injected", value)
}

func TestNoInterceptorMatchOnUnknownFeatureReturnsNotFound(t *testing.T) {
	repo := createRepository()
	repo.AddValueInterceptor(interceptorThatMisses())

	_, _, _, err := repo.GetFeature(context.TODO(), "unknown-key")

	assert.Error(t, err, "should return ErrFeatureNotFound when interceptor misses and feature absent")
}

func TestInterceptorReceivesCorrectKeyAndFeatureState(t *testing.T) {
	repo := createRepository()
	seedRepo(repo)

	var capturedKey string
	var capturedFeature *models.FeatureState

	repo.AddValueInterceptor(interfaces.NewInterceptor(func(_ context.Context, key string, _ interfaces.FeatureRepository, feature *models.FeatureState) (interface{}, bool) {
		capturedKey = key
		capturedFeature = feature
		return nil, false
	}))

	repo.GetFeature(context.TODO(), "flag") //nolint:errcheck

	assert.Equal(t, "flag", capturedKey)
	require.NotNil(t, capturedFeature)
	assert.Equal(t, models.TypeBoolean, capturedFeature.Type)
}

func TestInterceptorReceivesNilFeatureWhenKeyAbsent(t *testing.T) {
	repo := createRepository()

	var capturedFeature *models.FeatureState
	var captureTriggered bool = false
	repo.AddValueInterceptor(interfaces.NewInterceptor(func(_ context.Context, _ string, _ interfaces.FeatureRepository, feature *models.FeatureState) (interface{}, bool) {
		capturedFeature = feature
		captureTriggered = true
		return nil, false
	}))

	repo.GetFeature(context.TODO(), "absent") //nolint:errcheck

	assert.True(t, captureTriggered)
	assert.Nil(t, capturedFeature, "interceptor should receive nil for unknown features")
}

// --- Multiple interceptors ---

func TestMultipleInterceptorsFirstMatchWins(t *testing.T) {
	repo := createRepository()
	seedRepo(repo)

	callCount := 0
	repo.AddValueInterceptor(interfaces.NewInterceptor(func(_ context.Context, _ string, _ interfaces.FeatureRepository, _ *models.FeatureState) (interface{}, bool) {
		callCount++
		return "first", true
	}))
	repo.AddValueInterceptor(interfaces.NewInterceptor(func(_ context.Context, _ string, _ interfaces.FeatureRepository, _ *models.FeatureState) (interface{}, bool) {
		callCount++
		return "second", true
	}))

	_, matched, value, err := repo.GetFeature(context.TODO(), "flag")

	require.NoError(t, err)
	assert.True(t, matched)
	assert.Equal(t, "first", value)
	assert.Equal(t, 1, callCount, "second interceptor should not be called when first matches")
}

func TestMultipleInterceptorsAllMissFallsThrough(t *testing.T) {
	repo := createRepository()
	seedRepo(repo)
	repo.AddValueInterceptor(interceptorThatMisses())
	repo.AddValueInterceptor(interceptorThatMisses())

	_, matched, value, err := repo.GetFeature(context.TODO(), "label")

	require.NoError(t, err)
	assert.False(t, matched)
	assert.Equal(t, "default", value)
}

// --- Typed accessors respect interceptors ---

func TestInterceptorGetBooleanWithBoolOverride(t *testing.T) {
	repo := createRepository()
	seedRepo(repo) // "flag" starts as false
	repo.AddValueInterceptor(interceptorThatMatches(true))

	value, err := repo.GetBoolean(context.TODO(), "flag")

	require.NoError(t, err)
	assert.True(t, value, "GetBoolean should return the interceptor's override value")
}

func TestInterceptorGetBooleanWithStringOverride(t *testing.T) {
	repo := createRepository()
	seedRepo(repo)
	repo.AddValueInterceptor(interceptorThatMatches("true")) // string → bool conversion

	value, err := repo.GetBoolean(context.TODO(), "flag")

	require.NoError(t, err)
	assert.True(t, value)
}

func TestInterceptorGetNumberWithOverride(t *testing.T) {
	repo := createRepository()
	seedRepo(repo) // "count" starts as 42
	repo.AddValueInterceptor(interceptorThatMatches(float64(99)))

	value, err := repo.GetNumber(context.TODO(), "count")

	require.NoError(t, err)
	require.NotNil(t, value)
	assert.Equal(t, float64(99), *value)
}

func TestInterceptorGetStringWithOverride(t *testing.T) {
	repo := createRepository()
	seedRepo(repo) // "label" starts as "default"
	repo.AddValueInterceptor(interceptorThatMatches("overridden"))

	value, err := repo.GetString(context.TODO(), "label")

	require.NoError(t, err)
	require.NotNil(t, value)
	assert.Equal(t, "overridden", *value)
}

func TestInterceptorMatchedPropagatedFromGetInternalBoolean(t *testing.T) {
	repo := createRepository()
	seedRepo(repo)
	repo.AddValueInterceptor(interceptorThatMatches(true))

	_, matched, value, err := repo.GetInternalBoolean(context.TODO(), "flag")

	require.NoError(t, err)
	assert.True(t, matched)
	assert.True(t, value)
}

func TestInterceptorMatchedPropagatedFromGetInternalNumber(t *testing.T) {
	repo := createRepository()
	seedRepo(repo)
	repo.AddValueInterceptor(interceptorThatMatches(float64(7)))

	_, matched, value, err := repo.GetInternalNumber(context.TODO(), "count")

	require.NoError(t, err)
	assert.True(t, matched)
	require.NotNil(t, value)
	assert.Equal(t, float64(7), *value)
}

func TestInterceptorMatchedPropagatedFromGetInternalString(t *testing.T) {
	repo := createRepository()
	seedRepo(repo)
	repo.AddValueInterceptor(interceptorThatMatches("injected"))

	_, matched, value, err := repo.GetInternalString(context.TODO(), "label", models.TypeString)

	require.NoError(t, err)
	assert.True(t, matched)
	require.NotNil(t, value)
	assert.Equal(t, "injected", *value)
}

// --- Key-specific interceptor ---

func TestInterceptorCanTargetSpecificKey(t *testing.T) {
	repo := createRepository()
	seedRepo(repo)

	// Only override "flag", leave "label" alone.
	repo.AddValueInterceptor(interfaces.NewInterceptor(func(_ context.Context, key string, _ interfaces.FeatureRepository, _ *models.FeatureState) (interface{}, bool) {
		if key == "flag" {
			return true, true
		}
		return nil, false
	}))

	flagValue, err := repo.GetBoolean(context.TODO(), "flag")
	require.NoError(t, err)
	assert.True(t, flagValue, "interceptor override should apply to 'flag'")

	labelValue, err := repo.GetString(context.TODO(), "label")
	require.NoError(t, err)
	require.NotNil(t, labelValue)
	assert.Equal(t, "default", *labelValue, "non-targeted key should use stored value")
}

func TestAddValueInterceptorWhileReading(t *testing.T) {
	repo := createRepository()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			repo.AddValueInterceptor(interceptorThatMisses())
		}
	}()
	for i := 0; i < 50; i++ {
		repo.GetFeature(context.Background(), "missing") //nolint:errcheck
	}
	wg.Wait()
	assert.Len(t, repo.interceptors(), 50)
}

func TestInterceptorAddedAfterCloseIsClosed(t *testing.T) {
	repo := createRepository()
	closes := 0
	repo.AddValueInterceptor(interfaces.FeatureValueInterceptor{
		Intercept: interceptorThatMisses().Intercept,
		Close:     func() { closes++ },
	})
	repo.Close()
	repo.Close() // idempotent: each interceptor is closed once
	assert.Equal(t, 1, closes)

	late := false
	repo.AddValueInterceptor(interfaces.FeatureValueInterceptor{
		Intercept: interceptorThatMisses().Intercept,
		Close:     func() { late = true },
	})
	assert.True(t, late, "an interceptor added after Close is closed, not leaked")
	assert.Empty(t, repo.interceptors(), "closed interceptors are no longer called by feature reads")
}

func TestClosedInterceptorsAreNotCalled(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeature(ffs(`{"id":"id-a","key":"a","type":"BOOLEAN","value":true,"version":1}`))
	calls := 0
	repo.AddValueInterceptor(interfaces.NewInterceptor(func(context.Context, string, interfaces.FeatureRepository, *models.FeatureState) (interface{}, bool) {
		calls++
		return false, true
	}))
	repo.Close()

	_, _, value, err := repo.GetFeature(context.Background(), "a")
	assert.NoError(t, err)
	assert.Equal(t, true, value, "the feature's own value, not the closed interceptor's")
	assert.Zero(t, calls)
}

func TestSecondRepositoryCloseWaitsForTheFirst(t *testing.T) {
	repo := createRepository()
	closing, release := make(chan struct{}), make(chan struct{})
	repo.AddValueInterceptor(interfaces.FeatureValueInterceptor{
		Intercept: interceptorThatMisses().Intercept,
		Close: func() {
			close(closing)
			<-release
		},
	})
	go repo.Close()
	<-closing

	second := make(chan struct{})
	go func() {
		repo.closeWithin(time.Second)
		close(second)
	}()
	select {
	case <-second:
		t.Fatal("a second Close returned while the first was still closing interceptors")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-second:
	case <-time.After(time.Second):
		t.Fatal("the second Close did not return once the first finished")
	}
}

func TestInterceptorCanReadFeatures(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeature(ffs(`{"id":"id-a","key":"a","type":"BOOLEAN","value":true,"version":1}`))
	repo.ProcessFeature(ffs(`{"id":"id-b","key":"b","type":"BOOLEAN","value":false,"version":1}`))
	// an interceptor that makes "b" follow "a" reads a feature while "b" is being read
	repo.AddValueInterceptor(interfaces.NewInterceptor(func(ctx context.Context, key string, r interfaces.FeatureRepository, _ *models.FeatureState) (interface{}, bool) {
		if key != "b" {
			return nil, false
		}
		_, _, value, err := r.GetFeature(ctx, "a")
		return value, err == nil
	}))

	done := make(chan interface{})
	go func() {
		_, _, value, _ := repo.GetFeature(context.Background(), "b")
		done <- value
	}()
	select {
	case value := <-done:
		assert.Equal(t, true, value)
	case <-time.After(time.Second):
		t.Fatal("an interceptor reading a feature deadlocked")
	}
}

func TestFeatureReadsWhileFeaturesChange(t *testing.T) {
	repo := createRepository()
	repo.ProcessFeature(ffs(`{"id":"id-a","key":"a","type":"BOOLEAN","value":true,"version":1}`))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 2; i < 50; i++ {
			repo.ProcessFeature(ffs(fmt.Sprintf(`{"id":"id-a","key":"a","type":"BOOLEAN","value":true,"version":%d}`, i)))
			repo.ProcessFeature(ffs(fmt.Sprintf(`{"id":"id-%d","key":"k%d","type":"BOOLEAN","value":true,"version":1}`, i, i)))
			repo.ProcessDeleteFeature(ffs(fmt.Sprintf(`{"id":"id-%d","key":"k%d"}`, i, i)))
		}
		repo.ProcessFeatures([]*models.FeatureState{ffs(`{"id":"id-a","key":"a","type":"BOOLEAN","value":true,"version":99}`)})
	}()
	for i := 0; i < 200; i++ {
		_, _, value, err := repo.GetFeature(context.Background(), "a")
		assert.NoError(t, err)
		assert.Equal(t, true, value)
		repo.Properties(context.Background(), "a")
		repo.AllKeys()
		repo.GetFeatures()
	}
	wg.Wait()
	assert.Equal(t, []string{"a"}, repo.AllKeys())
}
