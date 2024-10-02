# go-shdep - Sharing objects and their updates in Go

## What this library can do?

* Implicitly share objects between multiple users based on their parameters.
* Initialize, update and shutdown objects in order of their dependecies.

## Intention

My crypto trading software has optimization module. The module runs tens of thousands of strategies at once.
Some of those strategies may rely on the same indicator with same configuration. And also there could exist other indicators, that use that indicator too. Without any mechanism for sharing of indicators that calculation will be repeated multiple times for all who requries it.

My intention was to share indicators among all users and reuse the calculations.
But multiple potential problems arise:

* The user of the indicator couldn't know about other existing users. So ideally it should create indicator as if he is the only user of it.
* Indicators may depend on each other, so we need to initialize, update and shutdown them in a proper order.
* While updating an indicator, it may trigger another update, and that would break the order.

To solve these problems, I introduced two components: Shared Store and Update Propagation Tree.

## Components

#### Shared Object Store

To share objects between multiple users Shared Object Store was introduced. It is responsible for storing all the shared objects, collecting their dependencies and ensuring proper order of their initialization, shutdown and other lifecycle steps.

To use store, you need to construct an object using its standard constructor, and then register the object as a required dependency.
This will:

1) replace your replica with the shared one of the same type and parameters,
2) inform the store that the registrator is dependant on that object.

Lifecycle methods of objects will then be called in an order, which takes into consideration their dependencies. This means, that `Init()` method of dependant object will always be called after `Init()` methods of all its dependencies and their dependencies. Note, that shutdown process happens in reverse order.

The store is not thread-safe, so different processing threads must use separate stores.
Although store controls lifecycle of the objects, the control over a lifecycle of the store itself is left to the user of the library.

#### Update propagation tree

Once all the objects are constructed, registered and initialized, they can start producing updates. The updates are propagated using Update Propagation Tree module.

The module is responsible for maintaining subscriptions and for ensuring proper order of updates. The objects are updated in an order of their dependecies. List of dependecies for updates is separate from the list of dependencies between objects created upon registration. It depends on connections made by using `Subsribe()` method.

Update proparation is trigged using `NotifyUpdated()` method. When it is called, all subscribers receive notification through function, which they have set using `SetUpdateHandler()`. If `NotifyUpdated()` is called while already processing update, the update will be propagated further. If not - the update proparation in that branch stops at that object.

Note, that update event in Update Propagation Tree does not indicate anything about the event itself (except of time). So in update handler you do not receive information about who triggered this update and what happened. But if you require this information, you can use `EventPullStorage` to actually pull events from your dependecies.

**WARNING**: It is crusial to do all subscriptions before sending a first event. This is because on a first even the library generates list of nodes to be updated from a specific source node. If subscription will happen after first event, it will not be included in update propagation.

## Usage

###### Define initialization parameters shared among all object

This structure will be passed to every shared object upon initialization. It can store connections to third-party services, configuration, anything external dependencies your objects may require.

```
// This is structure, which provides parameters and external
// dependencies for shared objects upon initialization.
type InitParams struct {
   ExternalUpdateLock *sync.Mutex   // Lock to protect shared objects update tree from external updates.
   ExternalEvents     chan struct{} // Dummy source of external events
   Results            chan any      // Destination for results
}
```

###### (optional) Define shared object interface for your business case:

```
// These type aliases are optinal - they are defined just for convenience.
// In this case we use context.Context as context, but anything else could be used.
// In my software I'm using coro.Context from github.com/nnikolash/go-coro.
type SharedObject = shdep.SharedObject[context.Context, *InitParams]
type SharedObjectBase = shdep.SharedObjectBase[context.Context, *InitParams]
type SharedStore = objstore.SharedStore[SharedObject, *InitParams]

var NewSharedStore = shdep.NewSharedStore[context.Context, *InitParams]
var NewSharedObjectBase = shdep.NewSharedObjectBase[context.Context, *InitParams]
var _ SharedObject = &SharedObjectBase{}
```

###### Define shared object

In this example it is just a simple counter of external events.
You can use `SharedObjectBase` as a helper, but its not required.

```
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

func (t *Counter) Init(params *InitParams) error {
   go func() {
      for range params.ExternalEvents {
         t.counter++

         func() {
            // We need this lock, because update propagation tree is not thread-safe,
            // so we need to protect it from external updates.
            params.ExternalUpdateLock.Lock()
            defer params.ExternalUpdateLock.Unlock()

            t.NotifyUpdated(context.Background(), time.Now())
         }()
      }
   }()

   return nil
}

func (t *Counter) Value() int {
   return t.counter
}

```

###### Define other objects which will depend on shared object

In this example two users are defined - `Concatenator` and `Multiplier`.

```

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

func (c *Concatenator) Init(params *InitParams) error {
   c.res = params.Results
   return nil
}

func (c *Concatenator) onDependenciesUpdated(ctx context.Context, evtTime time.Time) {
   c.res <- fmt.Sprintf("%v%v", c.prefix, c.counter.Value())
}

func NewMultiplier(counterStart int, mult float64) *Multiplier {
   m := &Multiplier{
      SharedObjectBase: NewSharedObjectBase("Multiplier", counterStart, mult),
      counter: NewCounter(counterStart),
      mult: mult,
   }

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
   store.Register(&m.counter)
   m.counter.SubscribeObj(m)
}

func (m *Multiplier) Init(params *InitParams) error {
   m.res = params.Results
   return nil
}

func (m *Multiplier) onDependenciesUpdated(ctx context.Context, evtTime time.Time) {
   m.res <- float64(m.counter.Value()) * m.mult
}
```

###### Create shared objects store

```
store := NewSharedStore(nil)
```

###### Create and register top-level objects

```
concat := NewConcatenator(1, "a")
mult := NewMultiplier(1, 2.0)

// Registering our top-level objects.
store.Register(&concat)
store.Register(&mult)
```

###### Initialize shared objects

```
// Running store lifecycle: Init -> Start -> Stop -> Close

externalEvents := make(chan struct{})
res := make(chan any, 100)

err := store.Init(&InitParams{
	ExternalUpdateLock: &sync.Mutex{},
	ExternalEvents:     externalEvents,
	Results:            res,
})
require.NoError(t, err)

err = store.Start()
require.NoError(t, err)
```

###### Generate some external events

```
// Let's send some events to our objects.
externalEvents <- struct{}{} // counter: 1 -> 2, concat: 2 -> "a2", mult: 2 -> 4
externalEvents <- struct{}{} // counter: 2 -> 3, concat: 3 -> "a3", mult: 3 -> 6
externalEvents <- struct{}{} // counter: 3 -> 4, concat: 4 -> "a4", mult: 4 -> 8
```

###### Shutdown shared objects

```
// Finalizing the store.
store.Stop()
store.Close()
```

###### Check results

```
// Let's check the results.
results := make([]any, 0, 6)
for len(results) < 6 {
   results = append(results, <-res)
}

require.Equal(t, []any{"a2", 4.0, "a3", 6.0, "a4", 8.0}, results)

// Lets check, that concatenator and multiplier used the same counter object.
require.Equal(t, reflect.ValueOf(concat.counter).Pointer(), reflect.ValueOf(mult.counter).Pointer())
```

## Events with data

Function `NotifyUpdated()` only notified subscribes, that something has changes. Subscribers then expected to pull new information from the objects they depend on. This mechanism does not provide a way to determite what has changed. You can call `HasUpdated()` for each of dependencies to find out if it has changed, but if you want more detailed approach you can use `EventPullStorage`.

This storage accumulates events, so that subscribers can pull them when processing notification about update.
Each subscriber pulls event separately. Events are stored until all of "pullers" retrieve them.

There is also `SharedObjectWithEventBase` helper exists as a version of `SharedObjectBase` helper but with built-it `EventPullStorage`.

**WARNING:** Don't forget to periodically pull all events for each puller created by  `NewPuller()`. Calling `NewPuller()` and then not using it will lead to memory leak.

###### Define event structure

```
type TestEvent struct {
   Field1 int
   Field2 string
}
```

###### Add event pull storage into object

```
func NewTestObjWithUpdates() *TestObjWithUpdates {
   ...
   return &TestObjWithUpdates{
      eventsPublisher: shdep.NewEventsPullStorage[TestEvent](),
      ...
   }
}

type TestObjWithUpdates struct {
   SharedObjectBase
   eventsPublisher *shdep.EventsPullStorage[TestEvent]
   ...
}

func (t *TestObjWithUpdates) NewEventsPuller() *shdep.EventPuller[TestEvent] {
   return t.eventsPublisher.NewPuller()
}

```

###### Publish event before notifying about update

```
func (t *TestObjWithUpdates) recalculate() {
   ...
   
   t.eventsPublisher.Publish(TestEvent{
      Field1: 1,
      Field2: "test",
   })
   
   t.NotifyUpdated(context.Background(), time.Now())
}
```

###### Add event puller into subscriber

```
...

type TestSubscriberObj struct {
   SharedObjectBase
   sub    *TestObjWithUpdates
   events *shdep.EventPuller[TestEvent]
   ...
}

func (t *TestSubscriberObj) Init(p *InitParams) error {
   t.events = t.sub.NewEventsPuller()

   // Note, that subscribing is not required, but most likely you would want to do it.
   // Otherwise, you won't be notified when new events are ready to be pulled.
   t.sub.SubscribeObj(t)

   ...
}
```

###### Pull events

```
func (t *TestSubscriberObj) onDependenciesUpdated(ctx context.Context, evtTime time.Time) {
   for _, evt := range t.events.Pull() {
      // Process pending event
      ...
   }
}
```

## SharedObjectBase helper

This helper does three main things:

* implements `SharedObject` interface,
* manages `updtree.Node` to provide method `NotifyUpdated()`,
* calculates hash of the object based on its parameters, which then used as object ID.

It is not mandatory to use it, but usually it is convenient. You can create your own base, e.g. with built-in `EventPullStorage`, with logger, with storing of init params etc.

**WARNING:** It is critical to pass ALL parameters into `NewSharedObjectBase()`. If not all parameters are passed, then objects with different parameters might have same ID and will be considered as "equal" or "same" upon registration. This will lead to unexpected and confusing behaviour and your calculations will be incorrect.

## Custom interface instead of SharedObject

Interface `SharedObject` and helper `SharedObjectBase` are created to provide a quick start. But if you don't like names of method, or don't like using parameters hash for objects identification, you can implement you own storage using `NewGenericStore()`. See implementation of `NewStore()` for hints.

## Examples

See examples in `examples` folder or in test files `*_test.go`.

---
