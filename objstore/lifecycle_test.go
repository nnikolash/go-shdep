package objstore_test

import (
	"testing"

	"github.com/nnikolash/go-shdep/objstore"
	"github.com/stretchr/testify/require"
)

func TestSharedStore_InitTwiceFails(t *testing.T) {
	t.Parallel()

	store := objstore.NewStore[SharedObject, *InitParams](func(obj SharedObject) string {
		return obj.ID()
	}, nil)

	so1 := NewSharedObj1([]string{"a", "b"}, "c", true, 1, 2.0)
	store.Register(&so1)

	err := store.Init(&InitParams{InitParam: 1})
	require.NoError(t, err)

	err = store.Init(&InitParams{InitParam: 2})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already initialized")
}

func TestSharedStore_StartWithoutInitFails(t *testing.T) {
	t.Parallel()

	store := objstore.NewStore[SharedObject, *InitParams](func(obj SharedObject) string {
		return obj.ID()
	}, nil)

	so1 := NewSharedObj1([]string{"a", "b"}, "c", true, 1, 2.0)
	store.Register(&so1)

	err := store.Start()
	require.Error(t, err)
	require.Contains(t, err.Error(), "was not initialized")
}

func TestSharedStore_StartEmptyStoreWithoutInitIsOK(t *testing.T) {
	t.Parallel()

	store := objstore.NewStore[SharedObject, *InitParams](func(obj SharedObject) string {
		return obj.ID()
	}, nil)

	require.NoError(t, store.Start(),
		"empty store must allow Start without Init (no-op)")
}

func TestSharedStore_TopLevelDependencies(t *testing.T) {
	t.Parallel()

	store := objstore.NewStore[SharedObject, *InitParams](func(obj SharedObject) string {
		return obj.ID()
	}, nil)

	so1 := NewSharedObj1([]string{"a", "b"}, "c", true, 1, 2.0)
	store.Register(&so1)

	require.NoError(t, store.Init(&InitParams{InitParam: 1}))

	top := store.TopLevelDependencies()
	require.Len(t, top, 1)
	require.Equal(t, so1.ID(), top[0])
}

func TestSharedStore_RecentlyRegisteredAndClears(t *testing.T) {
	t.Parallel()

	store := objstore.NewStore[SharedObject, *InitParams](func(obj SharedObject) string {
		return obj.ID()
	}, nil)

	so1 := NewSharedObj1([]string{"a", "b"}, "c", true, 1, 2.0)
	store.Register(&so1)

	recent := store.RecentlyRegisteredSharedObjects()
	require.Len(t, recent, 1, "must include just-registered object")
	require.Equal(t, so1.ID(), recent[0])

	recent = store.RecentlyRegisteredSharedObjects()
	require.Len(t, recent, 0, "must clear after read")
}

func TestSharedStore_RegisterPanicsOnNonPointerToPointer(t *testing.T) {
	t.Parallel()

	store := objstore.NewStore[SharedObject, *InitParams](func(obj SharedObject) string {
		return obj.ID()
	}, nil)

	so1 := NewSharedObj1([]string{"a", "b"}, "c", true, 1, 2.0)

	require.Panics(t, func() {
		store.Register(so1) // single pointer, not pointer-to-pointer
	})
}

func TestSharedStore_RegisterPanicsOnNilPointer(t *testing.T) {
	t.Parallel()

	store := objstore.NewStore[SharedObject, *InitParams](func(obj SharedObject) string {
		return obj.ID()
	}, nil)

	var so *SharedObj1
	require.Panics(t, func() {
		store.Register(&so) // points to a nil pointer
	})
}

func TestSharedStore_GetReturnsRegisteredObject(t *testing.T) {
	t.Parallel()

	store := objstore.NewStore[SharedObject, *InitParams](func(obj SharedObject) string {
		return obj.ID()
	}, nil)

	so1 := NewSharedObj1([]string{"a", "b"}, "c", true, 1, 2.0)
	store.Register(&so1)

	got := store.Get(so1.ID())
	require.NotNil(t, got)
	require.Equal(t, so1.ID(), got.ID())

	require.Nil(t, store.Get("nonexistent"))
}

func TestSharedStore_LifecycleOrder(t *testing.T) {
	t.Parallel()

	store := objstore.NewStore[SharedObject, *InitParams](func(obj SharedObject) string {
		return obj.ID()
	}, nil)

	so1 := NewSharedObj1([]string{"a", "b"}, "c", true, 1, 2.0)
	store.Register(&so1)

	require.NoError(t, store.Init(&InitParams{InitParam: 1}))
	require.NoError(t, store.Start())
	store.Stop()
	store.Close()

	// SharedObj5 sits at the bottom of the dep graph — verify it walked the whole lifecycle.
	require.True(t, so1.s2.s3.s5.initialized)
	require.True(t, so1.s2.s3.s5.started)
	require.True(t, so1.s2.s3.s5.stopped)
	require.True(t, so1.s2.s3.s5.closed)
}
