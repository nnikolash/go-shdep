package shdep

import (
	"reflect"

	"github.com/nnikolash/go-shdep/objstore"
	"github.com/nnikolash/go-shdep/utils"
)

func NewSharedStore[Ctx, InitParams any](l utils.Logger) SharedStore[Ctx, InitParams] {
	s := objstore.NewStore(func(obj SharedObject[Ctx, InitParams]) string {
		return reflect.TypeOf(obj).String() + "-" + obj.Hash()
	}, l)

	return s
}

type SharedStore[Ctx, InitParams any] objstore.SharedStore[SharedObject[Ctx, InitParams], InitParams]
