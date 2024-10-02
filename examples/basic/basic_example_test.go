package basic_example

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/nnikolash/go-shdep"
	"github.com/nnikolash/go-shdep/objstore"
	"github.com/stretchr/testify/require"
)

// These type aliases are optinal - they are defined just for convenience.
// In this case we use context.Context as context, but anything else could be used.
// In my software I'm using coro.Context from github.com/nnikolash/go-coro.
type SharedObject = shdep.SharedObject[context.Context, *InitParams]
type SharedObjectBase = shdep.SharedObjectBase[context.Context, *InitParams]
type SharedStore = objstore.SharedStore[SharedObject, *InitParams]

var NewSharedStore = shdep.NewSharedStore[context.Context, *InitParams]
var NewSharedObjectBase = shdep.NewSharedObjectBase[context.Context, *InitParams]
var _ SharedObject = &SharedObjectBase{}

// This is structure, which provides parameters and external
// dependencies for shared objects upon initialization.
type InitParams struct {
	ExternalUpdateLock *sync.Mutex   // Lock to protect shared objects update tree from external updates.
	ExternalEvents     chan struct{} // Dummy source of external events
	Results            chan any      // Destination for results
	InitOrder          *[]string     // Recording initialization order for testing purposes
}

// This is a simple counter, which increments its value on each external event
// and sends notification to all subscribers.
func NewCounter(counterStart int) *Counter {
	return &Counter{
		SharedObjectBase: NewSharedObjectBase("Counter", counterStart),
		counter:          counterStart,
	}
}

type Counter struct {
	SharedObjectBase // Using SharedObjectBase is mandatory, but just convenient.
	counter          int
}

func (t *Counter) Init(p *InitParams) error {
	go func() {
		for range p.ExternalEvents {
			t.counter++

			func() {
				// We need this lock, because update propagation tree is not thread-safe,
				// so we need to protect it from external updates.
				p.ExternalUpdateLock.Lock()
				defer p.ExternalUpdateLock.Unlock()

				t.NotifyUpdated(context.Background(), time.Now())
			}()
		}
	}()

	// For testing purpose
	*p.InitOrder = append(*p.InitOrder, t.Name())

	return nil
}

func (t *Counter) Value() int {
	return t.counter
}

func NewConcatenator(counterStart int, prefix string) *Concatenator {
	c := &Concatenator{
		// We MUST pass ALL parameters to NewSharedObjectBase. They are used to create unique hash for this object
		// and distinguish it from other objects of this type. If we don't pass all parameters, we may get
		// same hash for different objects and this will lead to incorrect calculations.
		SharedObjectBase: NewSharedObjectBase("Concatenator", counterStart, prefix),

		// Note, that we create counter in a standard way. This local instance will later be replaced
		// with shared replica upon registration in RegisterDependencies.
		counter: NewCounter(counterStart),

		prefix: prefix,
	}

	// Note, that we must not forget to set update handler here to be able to process updates from dependencies.
	c.SetUpdateHandler(c.onDependenciesUpdated)

	return c
}

type Concatenator struct {
	SharedObjectBase
	prefix  string
	counter *Counter // Note, that we use pointer here. This is needed, because registration requires passing pointer to pointer.
	res     chan any
}

var _ SharedObject = &Concatenator{}

func (c *Concatenator) RegisterDependencies(store SharedStore) {
	// We register counter as dependency. This will provide us shared replica of this object.
	// Note, that we pass pointer to pointer here. This is mandatory.
	store.Register(&c.counter)

	// This could be done at any point after Register and before first event sent.
	// So Init() should also work.
	c.counter.SubscribeObj(c)
}

func (c *Concatenator) Init(p *InitParams) error {
	c.res = p.Results

	// For testing purpose
	*p.InitOrder = append(*p.InitOrder, c.Name())

	return nil
}

func (c *Concatenator) onDependenciesUpdated(ctx context.Context, evtTime time.Time) {
	c.res <- fmt.Sprintf("%v%v", c.prefix, c.counter.Value())
}

func NewMultiplier(counterStart int, mult float64) *Multiplier {
	m := &Multiplier{
		// We MUST pass ALL parameters to NewSharedObjectBase. They are used to create unique hash for this object
		// and distinguish it from other objects of this type. If we don't pass all parameters, we may get
		// same hash for different objects and this will lead to incorrect calculations.
		SharedObjectBase: NewSharedObjectBase("Multiplier", counterStart, mult),

		// Note, that we create counter in a standard way. This local instance will later be replaced
		// with shared replica upon registration in RegisterDependencies.
		counter: NewCounter(counterStart),

		mult: mult,
	}

	// Note, that we must not forget to set update handler here to be able to process updates from dependencies.
	m.SetUpdateHandler(m.onDependenciesUpdated)

	return m
}

type Multiplier struct {
	SharedObjectBase
	mult    float64
	counter *Counter
	res     chan any
}

var _ SharedObject = &Multiplier{}

func (m *Multiplier) RegisterDependencies(store SharedStore) {
	// We register counter as dependency. This will provide us shared replica of this object.
	// Note, that we pass pointer to pointer here. This is mandatory.
	store.Register(&m.counter)

	// This could be done at any point after Register and before first event sent.
	// So Init() should also work.
	m.counter.SubscribeObj(m)
}

func (m *Multiplier) Init(p *InitParams) error {
	m.res = p.Results

	// For testing purpose
	*p.InitOrder = append(*p.InitOrder, m.Name())

	return nil
}

func (m *Multiplier) onDependenciesUpdated(ctx context.Context, evtTime time.Time) {
	m.res <- float64(m.counter.Value()) * m.mult
}

func TestExampleBasic(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		succeded := t.Run(fmt.Sprintf("TestExampleBasic/%v", i), func(t *testing.T) {
			store := NewSharedStore(nil)

			concat := NewConcatenator(1, "a")
			mult := NewMultiplier(1, 2.0)

			// Registering our top-level objects.
			store.Register(&concat)
			store.Register(&mult)

			// Running store lifecycle: Init -> Start -> Stop -> Close

			externalEvents := make(chan struct{})
			initOrder := make([]string, 0, 10)
			res := make(chan any, 100)

			err := store.Init(&InitParams{
				ExternalUpdateLock: &sync.Mutex{},
				ExternalEvents:     externalEvents,
				InitOrder:          &initOrder,
				Results:            res,
			})
			require.NoError(t, err)

			err = store.Start()
			require.NoError(t, err)

			// Let's verify stability of the initialization order.
			require.Equal(t, []string{"Counter", "Multiplier", "Concatenator"}, initOrder)

			// Let's send some events to our objects.
			const eventsCount = 3
			// counter: 1 -> 2, concat: 2 -> "a2", mult: 2 -> 4
			// counter: 2 -> 3, concat: 3 -> "a3", mult: 3 -> 6
			// counter: 3 -> 4, concat: 4 -> "a4", mult: 4 -> 8
			for i := 0; i < eventsCount; i++ {
				externalEvents <- struct{}{}
			}

			// Finalizing the store.
			store.Stop()
			store.Close()

			// Let's check the results nad their order
			results := make([]any, 0, eventsCount*2)
			for len(results) < 2*eventsCount {
				results = append(results, <-res)
			}

			require.Equal(t, []any{"a2", 4.0, "a3", 6.0, "a4", 8.0}, results)

			// Lets check, that concatenator and multiplier used the same counter object.
			require.Equal(t, reflect.ValueOf(concat.counter).Pointer(), reflect.ValueOf(mult.counter).Pointer())
		})

		if !succeded {
			break
		}
	}
}
