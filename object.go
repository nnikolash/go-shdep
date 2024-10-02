package shdep

import (
	"context"
	"time"

	"github.com/nnikolash/go-shdep/objstore"
	"github.com/nnikolash/go-shdep/updtree"
	"github.com/nnikolash/go-shdep/utils"
)

type SharedObject[Ctx, InitParams any] interface {
	objstore.SharedObject[SharedObject[Ctx, InitParams], InitParams]
	updtree.UpdateSubscription[Ctx]
	Hash() string
	Name() string
	GetUpdateNode() updtree.Node[Ctx]
}

func NewSharedObjectBase[Ctx, InitParams any](name string, params ...interface{}) SharedObjectBase[Ctx, InitParams] {
	if len(params) == 0 {
		panic("no params provided for hash")
	}
	hash := utils.Must2(utils.Hash(append(params, name)...))
	if hash == "" {
		panic("hash is empty")
	}

	o := SharedObjectBase[Ctx, InitParams]{
		updateNode: *updtree.NewNode[Ctx](name, nil),
		hash:       hash,
		name:       name,
	}

	return o
}

// SharedObjectBase is a helper base struct for shared objects.
// It does three things:
// * implements SharedObject interface,
// * manages updtree.Node to provide method NotifyUpdated(),
// * calculates hash of the object based on its parameters, which then used as object ID.
// If in addition to that you also need event publishing capabilities, use SharedObjectBaseWithEvent.
type SharedObjectBase[Ctx, InitParams any] struct {
	updateNode updtree.NodeBase[Ctx]
	name       string
	hash       string
}

// Hash is used as unique ID of the object.
// It is calculated based on object parameters and name. That's why it is important
// to pass ALL parameters into contructor.
func (o *SharedObjectBase[Ctx, InitParams]) Hash() string {
	return o.hash
}

// Name returns object name.
// It is mostly for debugging purposes. However, it is still used as part of object hash.
// So avoid making it different for the objects of same type and parameters, because that
// would negate purpose of sharing.
func (o *SharedObjectBase[Ctx, InitParams]) Name() string {
	return o.name
}

// SetUpdateHandler sets function, which will be called when any of subscriptions has updated.
func (s *SharedObjectBase[Ctx, InitParams]) SetUpdateHandler(handler func(ctx Ctx, evtTime time.Time)) {
	s.updateNode.SetUpdateHandler(handler)
}

func (o *SharedObjectBase[Ctx, InitParams]) GetUpdateNode() updtree.Node[Ctx] {
	return &o.updateNode
}

// One of lifecycle methods. See SharedObject interface for details.
func (o *SharedObjectBase[Ctx, InitParams]) RegisterDependencies(store objstore.SharedStore[SharedObject[Ctx, InitParams], InitParams]) {
}

// One of lifecycle methods. See SharedObject interface for details.
func (p *SharedObjectBase[Ctx, InitParams]) Init(params InitParams) error {
	return nil
}

// One of lifecycle methods. See SharedObject interface for details.
func (o *SharedObjectBase[Ctx, InitParams]) Start(params InitParams) error {
	return nil
}

// One of lifecycle methods. See SharedObject interface for details.
func (o *SharedObjectBase[Ctx, InitParams]) Stop() {
}

// One of lifecycle methods. See SharedObject interface for details.
func (o *SharedObjectBase[Ctx, InitParams]) Close() {
}

// NotifyUpdated notifies all subscribers that something changed.
func (s *SharedObjectBase[Ctx, InitParams]) NotifyUpdated(ctx Ctx, evtTime time.Time) {
	s.updateNode.NotifyUpdated(ctx, evtTime)
}

// SubscribeObj subscribes given object to updates of receiver.
func (o *SharedObjectBase[Ctx, InitParams]) SubscribeObj(subscriber SharedObject[Ctx, InitParams]) {
	o.updateNode.Subscribe(subscriber.GetUpdateNode())
}

func (o *SharedObjectBase[Ctx, InitParams]) Subscribe(subscriber updtree.Node[Ctx]) {
	o.updateNode.Subscribe(subscriber)
}

// Check that this node has been updated. Can be used, when processing
// updates and need to know which of subscriptions has been updated.
func (o *SharedObjectBase[Ctx, InitParams]) HasUpdated() bool {
	return o.updateNode.HasUpdated()
}

var _ SharedObject[context.Context, string] = &SharedObjectBase[context.Context, string]{}

// NewSharedObjectBaseWithEvent creates new SharedObjectBaseWithEvent.
// SharedObjectBaseWithEvent is same as SharedObjectBase, but with event publishing capabilities.
func NewSharedObjectBaseWithEvent[Ctx, InitParams any, Event any](name string, params ...interface{}) SharedObjectBaseWithEvent[Ctx, InitParams, Event] {
	return SharedObjectBaseWithEvent[Ctx, InitParams, Event]{
		SharedObjectBase: NewSharedObjectBase[Ctx, InitParams](name, params...),
		evtPublisher:     updtree.NewEventsPullStorage[Event](),
	}
}

// SharedObjectBaseWithEvent is same as SharedObjectBase, but with event publishing capabilities.
type SharedObjectBaseWithEvent[Ctx, InitParams any, Event any] struct {
	SharedObjectBase[Ctx, InitParams]
	evtPublisher *updtree.EventsPullStorage[Event]
}

// NewEventPuller returns new event puller for this object.
// Events must be pulled from it at least periodically, or it will cause memory leak.
func (o *SharedObjectBaseWithEvent[Ctx, InitParams, Event]) NewEventPuller() *updtree.EventPuller[Event] {
	return o.evtPublisher.NewPuller()
}

// PublishEvent publishes event and notifies all subscribers about update.
func (o *SharedObjectBaseWithEvent[Ctx, InitParams, Event]) PublishEvent(ctx Ctx, evtTime time.Time, evt Event) {
	o.evtPublisher.Publish(evt)
	o.NotifyUpdated(ctx, evtTime)
}
