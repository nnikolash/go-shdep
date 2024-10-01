package objstore

import (
	"reflect"
	"slices"
	"sort"

	"github.com/nnikolash/go-shdep/utils"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
)

type ObjRequirementsFunc[SharedObject any, ObjID comparable, InitParams any] func(obj SharedObject, s *GenericStore[SharedObject, ObjID, InitParams])
type ObjInitFunc[SharedObject any, InitParams any] func(obj SharedObject, params InitParams) error
type ObjStartFunc[SharedObject any, InitParams any] func(obj SharedObject, params InitParams) error
type ObjStopFunc[SharedObject any] func(obj SharedObject)
type ObjCloseFunc[SharedObject any] func(obj SharedObject)

// This is a generic store for shared objects.
// It can be used to support another interface instead of SharedObject.
// All lyfecycle functions are optional except Init.
func NewGenericStore[SharedObject any, ObjID comparable, InitParams any](
	getID func(obj SharedObject) ObjID,
	idLess func(a, b ObjID) bool, // optional
	gatherRequirements ObjRequirementsFunc[SharedObject, ObjID, InitParams],
	initObj ObjInitFunc[SharedObject, InitParams], // optional
	startObj ObjStartFunc[SharedObject, InitParams], // optional
	stopObj ObjStopFunc[SharedObject], // optional
	closeObj ObjCloseFunc[SharedObject], // optional
	l utils.Logger, // optional
) *GenericStore[SharedObject, ObjID, InitParams] {
	if l == nil {
		l = &utils.NoopLogger{}
	}

	return &GenericStore[SharedObject, ObjID, InitParams]{
		getID:              getID,
		idLess:             idLess,
		gatherRequirements: gatherRequirements,
		initObj:            initObj,
		startObj:           startObj,
		stopObj:            stopObj,
		closeObj:           closeObj,
		objects:            make(map[ObjID]SharedObject),
		l:                  l,
	}
}

type GenericStore[SharedObject any, ObjID comparable, InitParams any] struct {
	getID                           func(obj SharedObject) ObjID
	idLess                          func(a, b ObjID) bool
	gatherRequirements              ObjRequirementsFunc[SharedObject, ObjID, InitParams]
	initObj                         ObjInitFunc[SharedObject, InitParams]
	startObj                        ObjStartFunc[SharedObject, InitParams]
	stopObj                         ObjStopFunc[SharedObject]
	closeObj                        ObjCloseFunc[SharedObject]
	objects                         map[ObjID]SharedObject
	objectsRegistrationOrder        []ObjID
	topLevelDependencies            []ObjID
	recentlyRegisteredSharedObjects []ObjID
	dependencies                    []ObjID
	initializationOrder             []ObjID
	initParams                      InitParams
	l                               utils.Logger
}

// Register object to be shared with other users.
// Expects pointer to pointer.
func (s *GenericStore[SharedObject, ObjID, InitParams]) Register(obj interface{}) {
	objV := reflect.ValueOf(obj)
	objT := objV.Type()

	if objT.Kind() != reflect.Ptr || objT.Elem().Kind() != reflect.Ptr {
		s.l.Panicf("Register method accepts only pointers to pointers, got %T", obj)
		// TODO: maybe pointers to interfaces also makes sense?
	}

	if objV.IsNil() {
		s.l.Panicf("Pointer to object pointer must not be nil")
	}
	if objV.Elem().IsNil() {
		s.l.Panicf("Pointer to object must not be nil. Construct a desired object before registering it.")
	}

	var objAsSharedType SharedObject = objV.Elem().Interface().(SharedObject)
	objID := s.getID(objAsSharedType)

	if !slices.Contains(s.dependencies, objID) {
		s.dependencies = append(s.dependencies, objID)
	}

	s.recentlyRegisteredSharedObjects = append(s.recentlyRegisteredSharedObjects, objID)

	if existing, ok := s.objects[objID]; ok {
		existingT := reflect.TypeOf(existing)
		if !existingT.AssignableTo(objT.Elem()) {
			s.l.Panicf("Object with id %v of type %v is already registered and has different type: %v", objID, objT.Elem(), existingT)
		}
		objV.Elem().Set(reflect.ValueOf(existing))
		return
	}
	s.l.Debugf("Registering shared object %T/%v", obj, objID)
	s.objects[objID] = objAsSharedType
	s.objectsRegistrationOrder = append(s.objectsRegistrationOrder, objID)
}

// Init must be called first of all lifecycle methods.
// It is intended for gathering objects requirements and then setting their initial state.
// After Init has finished, object must be able to receive calls from other objects.
func (s *GenericStore[SharedObject, ObjID, InitParams]) Init(initParams InitParams) error {
	if len(s.topLevelDependencies) != 0 {
		return errors.New("shared objects store is already initialized")
	}

	s.topLevelDependencies = s.dependencies
	dependenciesGraph := make(map[ObjID][]ObjID, len(s.topLevelDependencies))
	s.collectDependencies(dependenciesGraph)

	utils.Assert(len(dependenciesGraph) == len(s.objects), "failed to collect all shared objects dependencies")

	s.l.Debugf("Dependecies graph: %v", dependenciesGraph)
	stability := s.objectsRegistrationOrder
	if s.idLess != nil {
		stability = maps.Keys(s.objects)

		sort.Slice(stability, func(i, j int) bool {
			return s.idLess(stability[i], stability[j])
		})
	}

	initializationOrder, err := utils.StableTopologicalSortWithSortedKeys(dependenciesGraph, stability)
	if err != nil {
		return errors.Wrapf(err, "failed to determine objects initialization order")
	}

	slices.Reverse(initializationOrder)
	s.l.Debugf("Shared objects initialization order: %v", initializationOrder)

	if s.initObj != nil {
		for _, objID := range initializationOrder {
			object := s.objects[objID]
			s.l.Debugf("Initializing object %T/%v", object, objID)
			if err := s.initObj(object, initParams); err != nil {
				return err
			}
		}
	}

	s.initializationOrder = initializationOrder
	s.initParams = initParams

	return nil
}

func (s *GenericStore[SharedObject, ObjID, InitParams]) collectDependencies(dependenciesGraph map[ObjID][]ObjID) {
	dependencies := s.dependencies
	s.dependencies = make([]ObjID, 0, len(s.dependencies))

	for _, objID := range dependencies {
		if _, processed := dependenciesGraph[objID]; processed {
			continue
		}
		obj := s.objects[objID]
		s.l.Debugf("Gathering requirements for object %T/%v", obj, objID)
		s.gatherRequirements(obj, s)

		dependenciesGraph[objID] = s.dependencies
		s.collectDependencies(dependenciesGraph)
	}
}

// Start must be called after Init. It is used as PostInit hook.
// It is intended for starting background processes, timers, etc.
// The onlt thing it does is calls Start() on all objects in the store.
func (s *GenericStore[SharedObject, ObjID, InitParams]) Start() error {
	if s.startObj == nil {
		return nil
	}

	if len(s.initializationOrder) == 0 && len(s.objects) != 0 {
		return errors.New("shared objects store was not initialized")
	}

	for _, objID := range s.initializationOrder {
		object := s.objects[objID]
		s.l.Debugf("Starting object %T/%v", object, objID)
		if err := s.startObj(object, s.initParams); err != nil {
			return err
		}
	}

	return nil
}

// Stop must be called after Start. It is used as PreClose hook.
// It is intended for stopping background processes, timers, etc.
// The only thing it does is calls Stop() on all objects in the store.
func (s *GenericStore[SharedObject, ObjID, InitParams]) Stop() {
	if s.stopObj == nil {
		return
	}

	for i := len(s.initializationOrder) - 1; i >= 0; i-- {
		objID := s.initializationOrder[i]
		object := s.objects[objID]

		s.l.Debugf("Stopping object %T/%v", object, objID)
		s.stopObj(object)
	}
}

// Close must be called after Stop. It is used to finalize objects.
// Can be used to free resources and ensure they are not used anywhere else.
// The only thing it does is calls Close() on all objects in the store.
func (s *GenericStore[SharedObject, ObjID, InitParams]) Close() {
	if s.closeObj == nil {
		return
	}

	for i := len(s.initializationOrder) - 1; i >= 0; i-- {
		objID := s.initializationOrder[i]
		object := s.objects[objID]

		s.l.Debugf("Closing object %T/%v", object, objID)
		s.closeObj(object)
	}
}

// Returns object by its ID.
func (s *GenericStore[SharedObject, ObjID, InitParams]) Get(objID ObjID) SharedObject {
	return s.objects[objID]
}

// Returns all objects, which were registered in the store before Init() was called.
func (s *GenericStore[SharedObject, ObjID, InitParams]) TopLevelDependencies() []ObjID {
	// TODO: rename
	return s.topLevelDependencies
}

// Returns objects, which were registered from last call of this method.
// It includes registration even if registered object was already in the store.
// This might be useful to retrieve requrements of the object without knowing what it does.
func (s *GenericStore[SharedObject, ObjID, InitParams]) RecentlyRegisteredSharedObjects() []ObjID {
	// TODO: rename/redesign
	ind := s.recentlyRegisteredSharedObjects
	s.recentlyRegisteredSharedObjects = nil
	return ind
}
