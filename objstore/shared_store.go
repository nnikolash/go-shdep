package objstore

import (
	"fmt"
	"reflect"

	"github.com/nnikolash/go-shdep/utils"
)

type SharedObject[CustomSharedObject any, InitParams any] interface {
	RegisterDependencies(s SharedStore[CustomSharedObject, InitParams])
	Init(p InitParams) error
	Start(p InitParams) error
	Stop()
	Close()
}

type SharedRegistry[ObjType any] interface {
	Register(obj interface{})
}

type SharedStore[CustomSharedObject any, InitParams any] interface {
	SharedRegistry[CustomSharedObject]
	Init(params InitParams) error
	Start() error
	Stop()
	Close()
	TopLevelDependencies() []string
	RecentlyRegisteredSharedObjects() []string
	Get(objID string) CustomSharedObject
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
