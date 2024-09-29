package updtree_test

import (
	"context"
	"testing"
	"time"

	"github.com/nnikolash/go-shdep/updtree"
	"github.com/stretchr/testify/require"
)

type Ctx = context.Context
type UpdatePropagationNodeBase = updtree.NodeBase[Ctx]
type UpdatePropagationNode = updtree.Node[Ctx]

func Test_UpdatePropagationTree_Basic(t *testing.T) {
	t.Parallel()

	eventsOrder := []string{}
	partial := false

	notifyUpdated := func(name string, excludeFromPartial bool) func(self UpdatePropagationNode) {
		return func(self UpdatePropagationNode) {
			if !partial || !excludeFromPartial {
				eventsOrder = append(eventsOrder, name)
				require.False(t, self.HasUpdated())
				self.NotifyUpdated(context.Background(), time.Time{})
				require.True(t, self.HasUpdated())
			}
		}
	}

	f := newUpdatePropagationNode("f", nil)

	e1 := newUpdatePropagationNode("e1", notifyUpdated("e1", false))
	d1 := newUpdatePropagationNode("d1", notifyUpdated("d1", false))
	c1 := newUpdatePropagationNode("c1", notifyUpdated("c1", true))
	b1 := newUpdatePropagationNode("b1", notifyUpdated("b1", false))

	e2 := newUpdatePropagationNode("e2", notifyUpdated("e2", true))
	d2 := newUpdatePropagationNode("d2", notifyUpdated("d2", false))
	c2 := newUpdatePropagationNode("c2", notifyUpdated("c2", true))
	b2 := newUpdatePropagationNode("b2", notifyUpdated("b2", false))

	type RealA struct {
		UpdatePropagationNodeBase
	}

	a := &RealA{
		UpdatePropagationNodeBase: *newUpdatePropagationNode("a", func(self UpdatePropagationNode) {
			eventsOrder = append(eventsOrder, "a")
		}),
	}
	//a := newUpdatePropagationNode("a", notifyUpdated("a", false))

	f.Subscribe(e1)

	e1.Subscribe(d1)
	e1.Subscribe(c1)
	e1.Subscribe(b1)

	d1.Subscribe(b1)
	c1.Subscribe(b1)

	b1.Subscribe(a)

	f.Subscribe(e2)

	e2.Subscribe(b2)
	e2.Subscribe(c2)
	e2.Subscribe(d2)

	d2.Subscribe(b2)
	c2.Subscribe(b2)

	b2.Subscribe(a)

	testE1 := func() {
		eventsOrder = []string{"e1"}
		require.False(t, e1.HasUpdated())
		e1.NotifyUpdated(context.Background(), time.Time{})
		require.False(t, e1.HasUpdated())
		if !partial {
			require.Equal(t, []string{"e1", "d1", "c1", "b1", "a"}, eventsOrder)
		} else {
			require.Equal(t, []string{"e1", "d1", "b1", "a"}, eventsOrder)
		}
	}

	testE2 := func() {
		eventsOrder = []string{"e2"}
		require.False(t, e2.HasUpdated())
		e2.NotifyUpdated(context.Background(), time.Time{})
		require.False(t, e2.HasUpdated())
		if !partial {
			require.Equal(t, []string{"e2", "c2", "d2", "b2", "a"}, eventsOrder)
		} else {
			require.Equal(t, []string{"e2", "d2", "b2", "a"}, eventsOrder)
		}
	}

	testF := func() {
		eventsOrder = []string{"f"}
		require.False(t, f.HasUpdated())
		f.NotifyUpdated(context.Background(), time.Time{})
		require.False(t, f.HasUpdated())
		if !partial {
			require.Equal(t, []string{"f", "e1", "e2", "d1", "c1", "c2", "d2", "b1", "b2", "a"}, eventsOrder)
		} else {
			require.Equal(t, []string{"f", "e1", "d1", "b1", "a"}, eventsOrder)
		}
	}

	runAllTests := func() {
		testE1()
		testE1()
		testE2()
		testE2()
		testF()
		testF()
		testE1()
		testF()

		require.False(t, f.HasUpdated())
		require.False(t, e1.HasUpdated())
		require.False(t, e2.HasUpdated())
		require.False(t, d1.HasUpdated())
		require.False(t, d2.HasUpdated())
		require.False(t, c1.HasUpdated())
		require.False(t, c2.HasUpdated())
		require.False(t, b1.HasUpdated())
		require.False(t, b2.HasUpdated())
		require.False(t, a.HasUpdated())
	}

	runAllTests()

	partial = true

	runAllTests()
}

func newUpdatePropagationNode(name string, handler func(self UpdatePropagationNode)) *UpdatePropagationNodeBase {
	n := updtree.NewNode[Ctx](name, nil)
	n.SetUpdateHandler(func(ctx Ctx, evtTime time.Time) { handler(n) })
	return n
}
