package updtree_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nnikolash/go-shdep/updtree"
	"github.com/stretchr/testify/require"
)

// These tests reproduce the multi-symbol backtest scenario that was flagged as
// non-deterministic (different byte-level results between identical runs) and
// prove that the update propagation tree delivers handler invocations in a
// byte-identical order across many freshly built trees.
//
// Why fresh trees expose non-determinism: Go randomizes map iteration order per
// range statement and re-seeds it for every freshly allocated map. So if any
// ordering decision in getUpdateOrder()/StableTopologicalSortWithSortedKeys
// leaked through map iteration (e.g. the historical maps.Keys(dependencies) bug,
// or the pointer-keyed dependenciesMap), rebuilding the tree N times within a
// single process would surface a different order on some iteration.

// buildMultiSymbolTree builds a "wide diamond" that mirrors a multi-symbol
// portfolio: one shared market tick fans out to per-symbol indicators, which
// converge on per-symbol signals, which all converge on one shared portfolio
// sink. The wide level of sibling indicators is exactly where tie-breaking in
// the topological sort decides the order.
func buildMultiSymbolTree(symbols int, trace *[]string) *UpdatePropagationNodeBase {
	tick := updtree.NewNode[Ctx]("tick", nil)

	// Sink: fires once per cascade, after every signal.
	portfolio := newRecordingNode("portfolio", trace, false)

	for s := 0; s < symbols; s++ {
		ema := newRecordingNode(fmt.Sprintf("sym%02d_ema", s), trace, true)
		rsi := newRecordingNode(fmt.Sprintf("sym%02d_rsi", s), trace, true)
		signal := newRecordingNode(fmt.Sprintf("sym%02d_signal", s), trace, true)

		tick.Subscribe(ema)
		tick.Subscribe(rsi)
		ema.Subscribe(signal)
		rsi.Subscribe(signal)
		signal.Subscribe(portfolio)
	}

	return tick
}

// newRecordingNode appends its name to trace whenever its handler runs. If
// propagate is true it re-notifies, pushing the cascade to its own subscribers.
func newRecordingNode(name string, trace *[]string, propagate bool) *UpdatePropagationNodeBase {
	n := updtree.NewNode[Ctx](name, nil)
	n.SetUpdateHandler(func(ctx Ctx, ts time.Time) {
		*trace = append(*trace, name)
		if propagate {
			n.NotifyUpdated(ctx, ts)
		}
	})
	return n
}

func cascadeOrder(symbols int) []string {
	trace := []string{}
	tick := buildMultiSymbolTree(symbols, &trace)
	tick.NotifyUpdated(context.Background(), time.Time{})
	return trace
}

// TestPropagationTree_DeterministicOrder_MultiSymbol rebuilds the multi-symbol
// tree many times and requires every cascade to produce the exact same handler
// order. A single deviation across iterations would prove non-determinism.
func TestPropagationTree_DeterministicOrder_MultiSymbol(t *testing.T) {
	t.Parallel()

	const symbols = 12
	const iterations = 3000

	want := cascadeOrder(symbols)

	// Sanity: the cascade actually did meaningful work.
	require.Len(t, want, symbols*3+1, "expected ema+rsi+signal per symbol plus one portfolio fire")
	require.Equal(t, "portfolio", want[len(want)-1], "portfolio (sink) must fire last")

	for i := 0; i < iterations; i++ {
		got := cascadeOrder(symbols)
		require.Equalf(t, want, got, "cascade order diverged on iteration %d (non-deterministic propagation)", i)
	}
}

// TestPropagationTree_PortfolioConvergesOnce guards the diamond-convergence
// contract under the wide multi-symbol shape: the shared sink fires exactly
// once per cascade, after all upstreams, regardless of fan-in width.
func TestPropagationTree_PortfolioConvergesOnce(t *testing.T) {
	t.Parallel()

	order := cascadeOrder(8)

	portfolioCount := 0
	lastSignalIdx := -1
	for i, name := range order {
		switch {
		case name == "portfolio":
			portfolioCount++
		case len(name) > 6 && name[len(name)-6:] == "signal":
			lastSignalIdx = i
		}
	}

	require.Equal(t, 1, portfolioCount, "shared sink must fire exactly once per cascade")
	require.Equal(t, "portfolio", order[len(order)-1])
	require.Less(t, lastSignalIdx, len(order)-1, "every signal fires before the portfolio sink")
}
