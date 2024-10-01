package shobj

import (
	"context"
	"sync"

	"github.com/nnikolash/go-shdep"
	"github.com/nnikolash/go-shdep/objstore"
)

type SharedObject = shdep.SharedObject[context.Context, *InitParams]
type SharedObjectBase = shdep.SharedObjectBase[context.Context, *InitParams]
type SharedStore = objstore.SharedStore[SharedObject, *InitParams]

var NewSharedStore = shdep.NewSharedStore[context.Context, *InitParams]
var NewSharedObjectBase = shdep.NewSharedObjectBase[context.Context, *InitParams]
var _ SharedObject = &SharedObjectBase{}

type InitParams struct {
	// Shared objects update tree is not thread-safe, so we need a lock to protect it.
	// This lock should be used only for updates, that come outside of update tree itself.
	// Another option to achieve same effect is to make external sources of updates single-threaded
	// if they provide updates via subscription and not as in this example by polling.
	ExternalUpdateLock *sync.Mutex

	GetPriceTicker func(asset string) chan float64
}
