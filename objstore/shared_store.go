package objstore

import (
	"fmt"
	"reflect"

	"github.com/nnikolash/go-shdep/utils"
)

type SharedObject[CustomSharedObject any, InitParams any] interface {
	// RegisterDependencies is the first lifecycle method called by store. It is used to
	// gather requirements of the object. You must call store.Register(obj) for each
	// object you depend on.
	RegisterDependencies(s SharedStore[CustomSharedObject, InitParams])

	// Init is called after all requirements are gathered. It is used to initialize object.
	// It is intended for setting up initial state of the object.
	// When Init returns, object is expected to be able to receive calls from other objects.
	Init(p InitParams) error

	// Start is called after Init. It is used as PostInit hook.
	// It is intended for starting background processes, timers, etc.
	Start(p InitParams) error

	// Stop is called before Close. It is used as PreClose hook.
	// It is intended for stopping background processes, timers, etc.
	Stop()

	// Close is called last. It is used to finalize objects.
	// Can be used to free resources and ensure they are not used anywhere else.
	Close()
}

type SharedRegistry[ObjType any] interface {
	// Register object to be shared with other users.
	// Expects pointer to pointer.
	Register(obj interface{})
}

type SharedStore[CustomSharedObject any, InitParams any] interface {
	SharedRegistry[CustomSharedObject]

	// Lifecycle methods. Must be called in order, and only once.

	// Init must be called first of all lifecycle methods.
	// It gathers objects requirements and then calls Init() on all objects.
	Init(params InitParams) error

	// Start must be called after Init. It is used as PostInit hook.
	// It is intended for starting background processes, timers, etc.
	// The onlt thing it does is calls Start() on all objects in the store.
	Start() error

	// Stop must be called after Start. It is used as PreClose hook.
	// It is intended for stopping background processes, timers, etc.
	// The only thing it does is calls Stop() on all objects in the store.
	Stop()

	// Close must be called after Stop. It is used to finalize objects.
	// Can be used to free resources and ensure they are not used anywhere else.
	// The only thing it does is calls Close() on all objects in the store.
	Close()

	// Returns object by its ID.
	Get(objID string) CustomSharedObject

	// Returns all objects, which were registered in the store before Init() was called.
	TopLevelDependencies() []string

	// Returns objects, which were registered from last call of this method.
	// It includes registration even if registered object was already in the store.
	// This might be useful to retrieve requrements of the object without knowing what it does.
	RecentlyRegisteredSharedObjects() []string
}

func NewStore[CustomSharedObject SharedObject[CustomSharedObject, InitParams], InitParams any](getID func(obj CustomSharedObject) string, l utils.Logger) *GenericStore[CustomSharedObject, string, InitParams] {
	var customObjType = reflect.TypeOf((*CustomSharedObject)(nil)).Elem()
	var genericObjType = reflect.TypeOf((*SharedObject[CustomSharedObject, InitParams])(nil)).Elem()

	if !customObjType.Implements(genericObjType) {
		panic(fmt.Sprintf("%v does not implement %v", customObjType, genericObjType))
	}

	return NewGenericStore(
		getID,
		func(id1, id2 string) bool {
			return id1 < id2
		},
		func(obj CustomSharedObject, s *GenericStore[CustomSharedObject, string, InitParams]) {
			interface{}(obj).(SharedObject[CustomSharedObject, InitParams]).RegisterDependencies(s)
		},
		func(obj CustomSharedObject, params InitParams) error {
			return interface{}(obj).(SharedObject[CustomSharedObject, InitParams]).Init(params)
		},
		func(obj CustomSharedObject, params InitParams) error {
			return interface{}(obj).(SharedObject[CustomSharedObject, InitParams]).Start(params)
		},
		func(obj CustomSharedObject) {
			interface{}(obj).(SharedObject[CustomSharedObject, InitParams]).Stop()
		},
		func(obj CustomSharedObject) {
			interface{}(obj).(SharedObject[CustomSharedObject, InitParams]).Close()
		},
		l,
	)
}

var _ SharedStore[interface{}, interface{}] = &GenericStore[interface{}, string, interface{}]{}
