package updtree_test

import (
	"context"
	"testing"
	"time"

	"github.com/nnikolash/go-shdep/updtree"
	"github.com/stretchr/testify/require"
)

// These tests document the ordering contract of the update propagation tree.
// They are intentionally explicit so that downstream users can reason about
// what shdep does and does not guarantee.

// CONTRACT 1: within a single cascade triggered by NotifyUpdated(),
// handlers are invoked in topological order of dependencies. A node that
// has multiple upstream paths receives a single handler invocation, after
// ALL its upstreams have fired. Intermediate nodes must call NotifyUpdated
// from their handler to propagate the cascade further.
func TestOrderingContract_SingleCascadeTopologicalOrder(t *testing.T) {
	t.Parallel()

	order := []string{}

	//      root
	//      /  \
	//     a    b
	//      \  /
	//      sink
	root := updtree.NewNode[Ctx]("root", nil)

	var a, b, sink *updtree.NodeBase[Ctx]

	a = updtree.NewNode[Ctx]("a", func(ctx Ctx, ts time.Time) {
		order = append(order, "a")
		a.NotifyUpdated(ctx, ts)
	})
	b = updtree.NewNode[Ctx]("b", func(ctx Ctx, ts time.Time) {
		order = append(order, "b")
		b.NotifyUpdated(ctx, ts)
	})
	sink = updtree.NewNode[Ctx]("sink", func(ctx Ctx, _ time.Time) {
		order = append(order, "sink")
	})

	root.Subscribe(a)
	root.Subscribe(b)
	a.Subscribe(sink)
	b.Subscribe(sink)

	root.NotifyUpdated(context.Background(), time.Time{})

	// sink fires AFTER both a and b.
	require.Equal(t, "sink", order[len(order)-1])
	require.Contains(t, order, "a")
	require.Contains(t, order, "b")
	sinkCount := 0
	for _, name := range order {
		if name == "sink" {
			sinkCount++
		}
	}
	require.Equal(t, 1, sinkCount, "diamond convergence — sink fires once per cascade")
}

// CONTRACT 2: cascades from independent source nodes are NOT interleaved
// or reordered. Each NotifyUpdated() call drives its own cascade to
// completion before returning. The caller controls inter-cascade order.
func TestOrderingContract_IndependentCascadesAreSequential(t *testing.T) {
	t.Parallel()

	order := []string{}

	htf := updtree.NewNode[Ctx]("htf", nil)
	ltf := updtree.NewNode[Ctx]("ltf", nil)

	htfSub := updtree.NewNode[Ctx]("htfSub", func(ctx Ctx, _ time.Time) {
		order = append(order, "htfSub")
	})
	ltfSub := updtree.NewNode[Ctx]("ltfSub", func(ctx Ctx, _ time.Time) {
		order = append(order, "ltfSub")
	})

	htf.Subscribe(htfSub)
	ltf.Subscribe(ltfSub)

	// Caller decides ordering — shdep just runs each cascade to completion.
	ltf.NotifyUpdated(context.Background(), time.Time{})
	htf.NotifyUpdated(context.Background(), time.Time{})

	require.Equal(t, []string{"ltfSub", "htfSub"}, order,
		"cascades run in the order NotifyUpdated() is called — no cross-source reordering")
}

// CONTRACT 3: shdep does NOT track event time across cascades. If a
// caller fires cascades in non-chronological order (e.g. replay with
// out-of-order ticks), handlers will be invoked in the call order with
// whatever evtTime the caller passed.
func TestOrderingContract_NoTimeBasedReordering(t *testing.T) {
	t.Parallel()

	times := []time.Time{}
	src := updtree.NewNode[Ctx]("src", nil)
	sub := updtree.NewNode[Ctx]("sub", func(ctx Ctx, t time.Time) {
		times = append(times, t)
	})
	src.Subscribe(sub)

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC) // earlier than t1

	src.NotifyUpdated(context.Background(), t1)
	src.NotifyUpdated(context.Background(), t0)

	require.Equal(t, []time.Time{t1, t0}, times,
		"shdep delivers events in call order; chronological ordering is the caller's responsibility")
}

// CONTRACT 4: NotifyUpdated() called from inside a handler does NOT
// start a new cascade — the update is folded into the current one and
// reaches subscribers of the re-notifying node in the current pass.
func TestOrderingContract_ReentrantNotifyFoldsIntoCurrentCascade(t *testing.T) {
	t.Parallel()

	order := []string{}

	a := updtree.NewNode[Ctx]("a", nil)
	// b re-emits when it gets updated.
	b := updtree.NewNode[Ctx]("b", nil)
	c := updtree.NewNode[Ctx]("c", func(ctx Ctx, _ time.Time) {
		order = append(order, "c")
	})

	b.SetUpdateHandler(func(ctx Ctx, ts time.Time) {
		order = append(order, "b")
		b.NotifyUpdated(ctx, ts)
	})

	a.Subscribe(b)
	b.Subscribe(c)

	a.NotifyUpdated(context.Background(), time.Time{})

	// Order is b then c — c is reached via the in-cascade re-notify from b.
	require.Equal(t, []string{"b", "c"}, order)
}
