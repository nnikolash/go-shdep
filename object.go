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

func NewSharedObjectBase[Ctx, InitParams any](name string, params ...interface{}) (_ SharedObjectBase[Ctx, InitParams]) {
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

type SharedObjectBase[Ctx, InitParams any] struct {
	updateNode updtree.NodeBase[Ctx]
	name       string
	hash       string
}

func (o *SharedObjectBase[Ctx, InitParams]) Name() string {
	return o.name
}

func (o *SharedObjectBase[Ctx, InitParams]) Hash() string {
	return o.hash
}

func (s *SharedObjectBase[Ctx, InitParams]) SetUpdateHandler(handler func(ctx Ctx, evtTime time.Time)) {
	s.updateNode.SetUpdateHandler(handler)
}

func (o *SharedObjectBase[Ctx, InitParams]) GetUpdateNode() updtree.Node[Ctx] {
	return &o.updateNode
}

func (o *SharedObjectBase[Ctx, InitParams]) RegisterDependencies(store objstore.SharedStore[SharedObject[Ctx, InitParams], InitParams]) {
}

func (p *SharedObjectBase[Ctx, InitParams]) Init(params InitParams) error {
	return nil
}

func (o *SharedObjectBase[Ctx, InitParams]) Start(params InitParams) error {
	return nil
}

func (o *SharedObjectBase[Ctx, InitParams]) Stop() {
}

func (o *SharedObjectBase[Ctx, InitParams]) Close() {
}

func (s *SharedObjectBase[Ctx, InitParams]) NotifyUpdated(ctx Ctx, evtTime time.Time) {
	s.updateNode.NotifyUpdated(ctx, evtTime)
}

func (o *SharedObjectBase[Ctx, InitParams]) SubscribeObj(subscriber SharedObject[Ctx, InitParams]) {
	o.updateNode.Subscribe(subscriber.GetUpdateNode())
}

func (o *SharedObjectBase[Ctx, InitParams]) Subscribe(node updtree.Node[Ctx]) {
	o.updateNode.Subscribe(node)
}

func (o *SharedObjectBase[Ctx, InitParams]) HasUpdated() bool {
	return o.updateNode.HasUpdated()
}

var _ SharedObject[context.Context, string] = &SharedObjectBase[context.Context, string]{}
