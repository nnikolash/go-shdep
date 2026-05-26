package shdep_test

import (
	"context"
	"testing"
	"time"

	"github.com/nnikolash/go-shdep"
	"github.com/stretchr/testify/require"
)

type testCtx = context.Context

type testInitParams struct{}

type testEvent struct {
	Val int
}

type counterObj struct {
	shdep.SharedObjectBase[testCtx, *testInitParams]
	updates int
}

func newCounterObj(name string, p int) *counterObj {
	return &counterObj{
		SharedObjectBase: shdep.NewSharedObjectBase[testCtx, *testInitParams](name, p),
	}
}

type emitterObj struct {
	shdep.SharedObjectBaseWithEvent[testCtx, *testInitParams, testEvent]
}

func newEmitterObj(name string, p int) *emitterObj {
	return &emitterObj{
		SharedObjectBaseWithEvent: shdep.NewSharedObjectBaseWithEvent[testCtx, *testInitParams, testEvent](name, p),
	}
}

func TestSharedObjectBase_NoParamsPanic(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t, "no params provided for hash", func() {
		_ = shdep.NewSharedObjectBase[testCtx, *testInitParams]("X")
	})
}

func TestSharedObjectBase_HashDifferByParams(t *testing.T) {
	t.Parallel()

	a := newCounterObj("Counter", 1)
	b := newCounterObj("Counter", 2)
	c := newCounterObj("Counter", 1)

	require.NotEqual(t, a.Hash(), b.Hash(), "objects with different params must have different hashes")
	require.Equal(t, a.Hash(), c.Hash(), "objects with same name and params must have same hash")
	require.Equal(t, "Counter", a.Name())
}

func TestSharedObjectBase_HashDifferByName(t *testing.T) {
	t.Parallel()

	a := newCounterObj("A", 1)
	b := newCounterObj("B", 1)

	require.NotEqual(t, a.Hash(), b.Hash(), "name participates in hash")
}

func TestSharedObjectBase_SubscribeObjPropagates(t *testing.T) {
	t.Parallel()

	upstream := newCounterObj("Upstream", 1)
	downstream := newCounterObj("Downstream", 1)

	downstream.SetUpdateHandler(func(ctx testCtx, _ time.Time) {
		downstream.updates++
	})

	upstream.SubscribeObj(downstream)

	upstream.NotifyUpdated(context.Background(), time.Time{})
	upstream.NotifyUpdated(context.Background(), time.Time{})

	require.Equal(t, 2, downstream.updates)
}

func TestSharedObjectBase_HasUpdatedFlag(t *testing.T) {
	t.Parallel()

	upstream := newCounterObj("Upstream", 1)
	downstream := newCounterObj("Downstream", 1)

	downstream.SetUpdateHandler(func(ctx testCtx, _ time.Time) {
		require.True(t, upstream.HasUpdated(),
			"upstream.HasUpdated() must report true while processing its cascade")
	})

	upstream.SubscribeObj(downstream)
	upstream.NotifyUpdated(context.Background(), time.Time{})

	require.False(t, upstream.HasUpdated(), "HasUpdated() resets after cascade")
}

func TestSharedObjectBaseWithEvent_PublishEvent(t *testing.T) {
	t.Parallel()

	emitter := newEmitterObj("Emitter", 1)
	subscriber := newCounterObj("Subscriber", 1)

	puller := emitter.NewEventPuller()

	notifies := 0
	subscriber.SetUpdateHandler(func(ctx testCtx, _ time.Time) {
		notifies++
	})

	emitter.SubscribeObj(subscriber)

	emitter.PublishEvent(context.Background(), time.Time{}, testEvent{Val: 1})
	emitter.PublishEvent(context.Background(), time.Time{}, testEvent{Val: 2})

	require.Equal(t, 2, notifies, "PublishEvent must trigger notify cascade for each call")

	events := puller.Pull()
	require.Len(t, events, 2)
	require.Equal(t, 1, events[0].Event.Val)
	require.Equal(t, 2, events[1].Event.Val)
}

func TestSharedObjectBaseWithEvent_PublishEventWithoutNotify(t *testing.T) {
	t.Parallel()

	emitter := newEmitterObj("Emitter", 1)
	subscriber := newCounterObj("Subscriber", 1)

	puller := emitter.NewEventPuller()

	notifies := 0
	subscriber.SetUpdateHandler(func(ctx testCtx, _ time.Time) {
		notifies++
	})
	emitter.SubscribeObj(subscriber)

	// Publish without Ctx — useful for tests where the cascade is irrelevant
	// or downstream cannot construct a real Ctx.
	emitter.PublishEventWithoutNotify(testEvent{Val: 7})
	emitter.PublishEventWithoutNotify(testEvent{Val: 8})

	require.Equal(t, 0, notifies, "PublishEventWithoutNotify must NOT trigger cascade")

	events := puller.Pull()
	require.Len(t, events, 2)
	require.Equal(t, 7, events[0].Event.Val)
	require.Equal(t, 8, events[1].Event.Val)
}

func TestSharedObjectBaseWithEvent_LastFromPuller(t *testing.T) {
	t.Parallel()

	emitter := newEmitterObj("Emitter", 1)
	puller := emitter.NewEventPuller()

	require.Nil(t, puller.Last(), "Last() on empty puller must return nil")

	emitter.PublishEvent(context.Background(), time.Time{}, testEvent{Val: 1})
	emitter.PublishEvent(context.Background(), time.Time{}, testEvent{Val: 5})

	last := puller.Last()
	require.NotNil(t, last)
	require.Equal(t, 5, last.Val, "Last() must return last published event and discard the rest")

	require.Nil(t, puller.Last(), "after Last() puller must be drained")
}

func TestNewSharedStore_Register(t *testing.T) {
	t.Parallel()

	store := shdep.NewSharedStore[testCtx, *testInitParams](nil)

	a := newCounterObj("A", 1)
	store.Register(&a)

	err := store.Init(&testInitParams{})
	require.NoError(t, err)
	err = store.Start()
	require.NoError(t, err)

	store.Stop()
	store.Close()

	require.Len(t, store.TopLevelDependencies(), 1)
}
