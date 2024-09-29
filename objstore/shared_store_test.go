package objstore_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/nnikolash/go-shdep/objstore"
	"github.com/nnikolash/go-shdep/utils"
	"github.com/stretchr/testify/require"
)

type SharedObject interface {
	objstore.SharedObject[SharedObject, *InitParams]
	ID() string
}

func TestSharedStore_Basic(t *testing.T) {
	t.Parallel()

	store := objstore.NewStore[SharedObject, *InitParams](func(obj SharedObject) string {
		return obj.ID()
	}, nil)

	so1 := NewSharedObj1([]string{"a", "b"}, "c", true, 1, 2.0)
	store.Register(&so1)
	err := store.Init(&InitParams{InitParam: 1})
	require.NoError(t, err)
	err = store.Start()
	require.NoError(t, err)

	randVal1 := so1.s2.s3.s5.randNum
	randVal2 := so1.s3.s5.randNum
	randVal3 := so1.s4.s5.randNum
	randVal4 := so1.s2.s5.randNum

	sameRandVal := randVal1 == randVal2 && randVal2 == randVal3 && randVal3 == randVal4
	require.True(t, sameRandVal, "object is not shared:", randVal1, randVal2, randVal3, randVal4)

	store.Stop()
	store.Close()

	so1.Verify(t)
}

// func TestSharedStore_WrongObjType(t *testing.T) {
// 	t.Parallel()

// 	defer func() {
// 		err := recover()
// 		errStr := fmt.Sprintf("%v", err)
// 		require.Contains(t, errStr, "does not implement", "panic message is incorrect")
// 	}()

// 	type WrongObj struct {
// 		ID string
// 	}

// 	store.NewStore[*WrongObj, *InitParams](func(obj *WrongObj) string {
// 		return obj.ID
// 	})

// 	require.True(t, false, "panic expected")
// }

func TestSharedStore_SameNameDifferentType(t *testing.T) {
	t.Parallel()

	defer func() {
		err := recover()
		errStr := fmt.Sprintf("%v", err)
		require.Contains(t, errStr, "is already registered and has different type", "panic message is incorrect")
	}()

	so1 := NewSharedObj1([]string{"a", "b"}, "c", true, 1, 2.0)

	type SharedObj5Copied struct {
		SharedObj5
	}
	var s5c *SharedObj5Copied = &SharedObj5Copied{}
	s5c.SharedObj5 = *NewSharedObj5(1, 2.0)
	s5c.SharedObjectBase = *NewSharedObjectBase("5", s5c.param1, s5c.param2)

	store := objstore.NewStore[SharedObject, *InitParams](func(obj SharedObject) string {
		return obj.ID()
	}, nil)

	store.Register(&so1)
	store.Register(&s5c)
	store.Init(&InitParams{InitParam: 1})

	require.True(t, false, "panic expected")
}

func NewSharedObjectBase(name string, params ...interface{}) *SharedObjectBase {
	hash := utils.Must2(utils.Hash(params...))

	return &SharedObjectBase{
		id: fmt.Sprintf("%v-%v", name, hash),
	}
}

type SharedObjectBase struct {
	id          string
	initialized bool
	started     bool
	stopped     bool
	closed      bool
}

func (so *SharedObjectBase) ID() string {
	return so.id
}

func (so *SharedObjectBase) Init(p *InitParams) error {
	if so.initialized {
		return fmt.Errorf("object %v is already initialized", so.id)
	}

	so.initialized = true
	return nil
}

func (so *SharedObjectBase) Start(p *InitParams) error {
	if !so.initialized {
		return fmt.Errorf("object %v is not initialized", so.id)
	}
	if so.started {
		return fmt.Errorf("object %v is already started", so.id)
	}

	so.started = true
	return nil
}

func (so *SharedObjectBase) Stop() {
	if !so.initialized {
		panic(fmt.Sprintf("object %v is not initialized", so.id))
	}
	if !so.started {
		panic(fmt.Sprintf("object %v is not started", so.id))
	}
	if so.stopped {
		panic(fmt.Sprintf("object %v is already stopped", so.id))
	}

	so.stopped = true
}

func (so *SharedObjectBase) Close() {
	if !so.initialized {
		panic(fmt.Sprintf("object %v is not initialized", so.id))
	}
	if !so.started {
		panic(fmt.Sprintf("object %v is not started", so.id))
	}
	if !so.stopped {
		panic(fmt.Sprintf("object %v is not stopped", so.id))
	}
	if so.closed {
		panic(fmt.Sprintf("object %v is already closed", so.id))
	}
	so.closed = true
}

func (so *SharedObjectBase) Verify(t *testing.T) {
	require.True(t, so.initialized, "object %v is not initialized", so.id)
	require.True(t, so.started, "object %v is not started", so.id)
	require.True(t, so.stopped, "object %v is not stopped", so.id)
	require.True(t, so.closed, "object %v is closed", so.id)
}

type InitParams struct {
	InitParam int
}

func NewSharedObj1(param1 []string, param2 string, param3 bool, param4 int, param5 float32) *SharedObj1 {
	return &SharedObj1{
		SharedObjectBase: *NewSharedObjectBase("1", param1, param2, param3, param4, param5),
		s2:               NewSharedObj2(param2, param3, param4, param5),
		s3:               NewSharedObj3(param3, param4, param5),
		s4:               NewSharedObj4(param4, param5),
	}
}

type SharedObj1 struct {
	SharedObjectBase

	s2 *SharedObj2
	s3 *SharedObj3
	s4 *SharedObj4
}

func (so *SharedObj1) RegisterDependencies(s objstore.SharedStore[SharedObject, *InitParams]) {
	s.Register(&so.s2)
	s.Register(&so.s3)
	s.Register(&so.s4)
}

func NewSharedObj2(param1 string, param2 bool, param3 int, param4 float32) *SharedObj2 {
	return &SharedObj2{
		SharedObjectBase: *NewSharedObjectBase("2", param1, param3, param4),
		param1:           param1,
		s3:               NewSharedObj3(param2, param3, param4),
		s5:               NewSharedObj5(param3, param4),
	}
}

type SharedObj2 struct {
	SharedObjectBase
	param1 string
	s3     *SharedObj3
	s5     *SharedObj5
}

func (so *SharedObj2) RegisterDependencies(s objstore.SharedStore[SharedObject, *InitParams]) {
	s.Register(&so.s3)
	s.Register(&so.s5)
}

func NewSharedObj3(param1 bool, param3 int, param4 float32) *SharedObj3 {
	return &SharedObj3{
		SharedObjectBase: *NewSharedObjectBase("3", param1, param3, param4),
		param1:           param1,
		s5:               NewSharedObj5(param3, param4),
	}
}

type SharedObj3 struct {
	SharedObjectBase
	param1 bool
	s5     *SharedObj5
}

func (so *SharedObj3) RegisterDependencies(s objstore.SharedStore[SharedObject, *InitParams]) {
	s.Register(&so.s5)
}

func NewSharedObj4(param1 int, param2 float32) *SharedObj4 {
	return &SharedObj4{
		SharedObjectBase: *NewSharedObjectBase("4", param1, param2),
		s5:               NewSharedObj5(param1, param2),
	}
}

type SharedObj4 struct {
	SharedObjectBase
	s5 *SharedObj5
}

func (so *SharedObj4) RegisterDependencies(s objstore.SharedStore[SharedObject, *InitParams]) {
	s.Register(&so.s5)
}

func NewSharedObj5(param1 int, param2 float32) *SharedObj5 {
	return &SharedObj5{
		SharedObjectBase: *NewSharedObjectBase("5", param1, param2),
		param1:           param1,
		param2:           param2,
		randNum:          rand.Int(),
	}
}

type SharedObj5 struct {
	SharedObjectBase
	param1  int
	param2  float32
	randNum int
}

func (so *SharedObj5) RegisterDependencies(s objstore.SharedStore[SharedObject, *InitParams]) {
}
