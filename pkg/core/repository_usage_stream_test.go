package core

import (
	"context"
	"testing"

	"github.com/featurehub-io/featurehub-go-sdk/pkg/usage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// simpleEvent builds a minimal BaseUsageEvent with a user key.
func simpleEvent(userKey string) *usage.BaseUsageEvent {
	e := &usage.BaseUsageEvent{}
	e.SetUserKey(userKey)
	return e
}

// --- EmitUsageEvent ---

func TestEmitUsageEventNoHandlersDoesNotPanic(t *testing.T) {
	repo := createRepository()

	assert.NotPanics(t, func() {
		repo.EmitUsageEvent(context.TODO(), simpleEvent("user-1"))
	})
}

func TestEmitUsageEventDeliveredToSingleStream(t *testing.T) {
	repo := createRepository()

	var received usage.UsageEvent
	repo.RegisterUsageStream(func(_ context.Context, event usage.UsageEvent) {
		received = event
	})

	event := simpleEvent("user-1")
	repo.EmitUsageEvent(context.TODO(), event)

	require.NotNil(t, received)
	assert.Equal(t, "user-1", received.UserKey())
}

func TestEmitUsageEventDeliveredToAllStreams(t *testing.T) {
	repo := createRepository()

	var calls [2]int
	repo.RegisterUsageStream(func(_ context.Context, _ usage.UsageEvent) { calls[0]++ })
	repo.RegisterUsageStream(func(_ context.Context, _ usage.UsageEvent) { calls[1]++ })

	repo.EmitUsageEvent(context.TODO(), simpleEvent("user-1"))

	assert.Equal(t, 1, calls[0])
	assert.Equal(t, 1, calls[1])
}

func TestEmitUsageEventMultipleEventsDeliveredInOrder(t *testing.T) {
	repo := createRepository()

	var received []string
	repo.RegisterUsageStream(func(_ context.Context, event usage.UsageEvent) {
		received = append(received, event.UserKey())
	})

	repo.EmitUsageEvent(context.TODO(), simpleEvent("first"))
	repo.EmitUsageEvent(context.TODO(), simpleEvent("second"))
	repo.EmitUsageEvent(context.TODO(), simpleEvent("third"))

	assert.Equal(t, []string{"first", "second", "third"}, received)
}

// --- RegisterUsageStream / RemoveUsageStream ---

func TestRegisterUsageStreamReturnsDistinctIDs(t *testing.T) {
	repo := createRepository()

	id1 := repo.RegisterUsageStream(func(_ context.Context, _ usage.UsageEvent) {})
	id2 := repo.RegisterUsageStream(func(_ context.Context, _ usage.UsageEvent) {})

	assert.NotEqual(t, id1, id2)
}

func TestRemoveUsageStreamStopsDelivery(t *testing.T) {
	repo := createRepository()

	var count int
	id := repo.RegisterUsageStream(func(_ context.Context, _ usage.UsageEvent) { count++ })

	repo.EmitUsageEvent(context.TODO(), simpleEvent("before"))
	assert.Equal(t, 1, count)

	repo.RemoveUsageStream(id)
	repo.EmitUsageEvent(context.TODO(), simpleEvent("after"))

	assert.Equal(t, 1, count, "handler should not be called after removal")
}

func TestRemoveUsageStreamOnlyRemovesTargeted(t *testing.T) {
	repo := createRepository()

	var countA, countB int
	idA := repo.RegisterUsageStream(func(_ context.Context, _ usage.UsageEvent) { countA++ })
	repo.RegisterUsageStream(func(_ context.Context, _ usage.UsageEvent) { countB++ })

	repo.RemoveUsageStream(idA)
	repo.EmitUsageEvent(context.TODO(), simpleEvent("user-1"))

	assert.Equal(t, 0, countA, "removed handler should not fire")
	assert.Equal(t, 1, countB, "remaining handler should still fire")
}

func TestRemoveUsageStreamUnknownIDDoesNotPanic(t *testing.T) {
	repo := createRepository()

	assert.NotPanics(t, func() {
		repo.RemoveUsageStream(9999)
	})
}
