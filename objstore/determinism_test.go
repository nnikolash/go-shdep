package objstore_test

import (
	"fmt"
	"testing"

	"github.com/nnikolash/go-shdep/objstore"
	"github.com/nnikolash/go-shdep/utils"
	"github.com/stretchr/testify/require"
)

// These tests reproduce the multi-symbol backtest scenario that was flagged as
// non-deterministic and prove that the store initializes shared objects in a
// byte-identical order across many freshly built stores.
//
// Rebuilding a fresh store each iteration matters: every store allocates fresh
// `objects` and `dependenciesGraph` maps, and Go re-seeds map iteration order
// per allocation. If the initialization order leaked through map iteration
// (the historical maps.Keys(dependencies) bug fixed in 54578ef), some iteration
// here would produce a different order.

// recObj is a minimal shared object that records the order in which Init() fires
// into a shared trace, so determinism can be asserted directly on lifecycle order.
type recObj struct {
	id    string
	deps  []*recObj
	trace *[]string
}

func newRecObj(name string, trace *[]string, deps ...*recObj) *recObj {
	// Content-derived id, same scheme the real shdep store uses (Hash(params)).
	return &recObj{
		id:    fmt.Sprintf("%v-%v", name, utils.Must2(utils.Hash(name))),
		deps:  deps,
		trace: trace,
	}
}

func (o *recObj) ID() string { return o.id }

func (o *recObj) RegisterDependencies(s objstore.SharedStore[SharedObject, *InitParams]) {
	for i := range o.deps {
		s.Register(&o.deps[i])
	}
}

func (o *recObj) Init(p *InitParams) error {
	*o.trace = append(*o.trace, o.id)
	return nil
}

func (o *recObj) Start(p *InitParams) error { return nil }
func (o *recObj) Stop()                     {}
func (o *recObj) Close()                    {}

// buildSymbolGraph models a multi-symbol portfolio sharing two global objects
// (a clock every price depends on, and a portfolio every signal depends on).
// It returns the top-level objects to register and the shared trace recorder.
func buildSymbolGraph(symbols int) ([]*recObj, *[]string) {
	trace := &[]string{}

	clock := newRecObj("clock", trace)
	portfolio := newRecObj("portfolio", trace)

	tops := make([]*recObj, 0, symbols)
	for s := 0; s < symbols; s++ {
		price := newRecObj(fmt.Sprintf("sym%02d_price", s), trace, clock)
		ma := newRecObj(fmt.Sprintf("sym%02d_ma", s), trace, price)
		rsi := newRecObj(fmt.Sprintf("sym%02d_rsi", s), trace, price)
		signal := newRecObj(fmt.Sprintf("sym%02d_signal", s), trace, ma, rsi, portfolio)
		tops = append(tops, signal)
	}

	return tops, trace
}

func initOrder(t *testing.T, symbols int) []string {
	tops, trace := buildSymbolGraph(symbols)

	store := objstore.NewStore[SharedObject, *InitParams](func(obj SharedObject) string {
		return obj.ID()
	}, nil)

	for i := range tops {
		store.Register(&tops[i])
	}

	require.NoError(t, store.Init(&InitParams{InitParam: 1}))

	return *trace
}

// TestStore_DeterministicInitOrder_MultiSymbol rebuilds the multi-symbol store
// many times and requires Init() to fire in the exact same order every time.
func TestStore_DeterministicInitOrder_MultiSymbol(t *testing.T) {
	t.Parallel()

	const symbols = 12
	const iterations = 3000

	want := initOrder(t, symbols)

	// Each symbol: price, ma, rsi, signal (4) + 2 shared globals (clock, portfolio).
	require.Len(t, want, symbols*4+2, "every shared object must be initialized exactly once")

	for i := 0; i < iterations; i++ {
		got := initOrder(t, symbols)
		require.Equalf(t, want, got, "init order diverged on iteration %d (non-deterministic store init)", i)
	}
}

// TestStore_InitOrderRespectsDependencies verifies dependencies are initialized
// before their dependants (the lifecycle guarantee the determinism rests on).
func TestStore_InitOrderRespectsDependencies(t *testing.T) {
	t.Parallel()

	order := initOrder(t, 6)

	pos := make(map[string]int, len(order))
	for i, id := range order {
		pos[id] = i
	}

	idOf := func(name string) string {
		return fmt.Sprintf("%v-%v", name, utils.Must2(utils.Hash(name)))
	}

	// clock before every price; price before its ma/rsi; ma/rsi/portfolio before signal.
	for s := 0; s < 6; s++ {
		price := fmt.Sprintf("sym%02d_price", s)
		ma := fmt.Sprintf("sym%02d_ma", s)
		rsi := fmt.Sprintf("sym%02d_rsi", s)
		signal := fmt.Sprintf("sym%02d_signal", s)

		require.Less(t, pos[idOf("clock")], pos[idOf(price)], "clock must init before %s", price)
		require.Less(t, pos[idOf(price)], pos[idOf(ma)], "price must init before %s", ma)
		require.Less(t, pos[idOf(price)], pos[idOf(rsi)], "price must init before %s", rsi)
		require.Less(t, pos[idOf(ma)], pos[idOf(signal)], "ma must init before %s", signal)
		require.Less(t, pos[idOf(rsi)], pos[idOf(signal)], "rsi must init before %s", signal)
		require.Less(t, pos[idOf("portfolio")], pos[idOf(signal)], "portfolio must init before %s", signal)
	}
}
