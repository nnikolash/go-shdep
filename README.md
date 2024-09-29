# go-shdep - Managing shared objects and their updated in Go

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

To share objects between multiple users Shared Object Store was introduced. It is responsible for storing all the shared objects, collecting their dependencies and ensuring proper order of their initialization and shutdown.
The store is not thread-safe, so different processing threads must use separate stores.
The control over a lifetime of the store itself is left to the user of the library.

#### Update propagation tree

Once all the objects are initialized, they can start producing updates. The updates are managed by Update Propagation Tree module.
The module is responsible for maintaining subscriptions and for ensuring proper order of updates.

## Usage

###### Define initialization parameters shared among all object

This structure will be passed to every shared object upon initialization. It can store connections to third-party services, configuration, anything external dependencies your objects may require.

```
type InitParams struct {
   ExternalUpdateLock *sync.Mutex
   GetPriceTicker func(asset string) chan float64
}
```

###### (optional) Define shared object interface for your business case:

```
// In this case we use context.Context as context, but anything else could be used.
// In my software I'm using coro.Context from github.com/nnikolash/go-coro

type SharedObject = shdep.SharedObject[context.Context, *InitParams]
type SharedObjectBase = shdep.SharedObjectBase[context.Context, *InitParams]
type SharedStore = objstore.SharedStore[SharedObject, *InitParams]

var NewSharedStore = shdep.NewSharedStore[context.Context, *InitParams]
var NewSharedObjectBase = shdep.NewSharedObjectBase[context.Context, *InitParams]
```

###### Define your object

You can use `SharedObjectBase` as a helper, but its not required.

```

```

## Examples

See examples in `examples` folder or in test files `*_test.go`.
