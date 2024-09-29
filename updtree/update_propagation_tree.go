package updtree

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/nnikolash/go-shdep/utils"
)

var NoHandler = func(n interface{}) {}

type UpdateSubscription[Ctx any] interface {
	Subscribe(node Node[Ctx])
	HasUpdated() bool
}

type Node[Ctx any] interface {
	UpdateSubscription[Ctx]

	NotifyUpdated(ctx Ctx, evtTime time.Time)
	SetUpdateHandler(onSubscriptionUpdated func(ctx Ctx, evtTime time.Time)) // TODO: this method should be available only for the parent, but for the users of parent

	self() Node[Ctx]
	getDependencies(dest *[]dependencyEntry[Ctx])
	addSubscription(subscription Node[Ctx])
	setSubscriptionUpdated(v bool)
	hasUpdatedSubscription() bool
	setHasUpdated(v bool)
	handleSubscriptionsUpdated(ctx Ctx, evtTime time.Time)
}

func NewNode[Ctx any](name string, onSubscriptionUpdated func(ctx Ctx, evtTime time.Time)) *NodeBase[Ctx] {
	return &NodeBase[Ctx]{
		name:                  name,
		onSubscriptionUpdated: onSubscriptionUpdated,
	}
}

type NodeBase[Ctx any] struct {
	name string

	subscribers           []Node[Ctx]
	subscribtions         []Node[Ctx]
	onSubscriptionUpdated func(ctx Ctx, evtTime time.Time)

	treeUpdateOrder []Node[Ctx]

	updated             bool
	subscriptionUpdated bool
}

var _ Node[interface{}] = &NodeBase[interface{}]{}

func (n *NodeBase[Ctx]) Subscribe(node Node[Ctx]) {
	n.subscribers = append(n.subscribers, node.self())
	node.addSubscription(n)
}

func (n *NodeBase[Ctx]) SetUpdateHandler(onSubscriptionUpdated func(ctx Ctx, evtTime time.Time)) {
	n.onSubscriptionUpdated = onSubscriptionUpdated
}

func (n *NodeBase[Ctx]) addSubscription(subscription Node[Ctx]) {
	n.subscribtions = append(n.subscribtions, subscription)
}

type dependencyEntry[Ctx any] struct {
	Node       Node[Ctx]
	Dependants []Node[Ctx]
}

func (n *NodeBase[Ctx]) getDependencies(dest *[]dependencyEntry[Ctx]) {
	*dest = append(*dest, dependencyEntry[Ctx]{Node: n, Dependants: n.subscribers})

	for _, subscriber := range n.subscribers {
		subscriber.getDependencies(dest)
	}
}

func (n *NodeBase[Ctx]) self() Node[Ctx] {
	return n
}

func (n *NodeBase[Ctx]) setSubscriptionUpdated(v bool) {
	n.subscriptionUpdated = v
}

func (n *NodeBase[Ctx]) hasUpdatedSubscription() bool {
	return n.subscriptionUpdated
}

func (n *NodeBase[Ctx]) setHasUpdated(v bool) {
	n.updated = v
}

func (n *NodeBase[Ctx]) HasUpdated() bool {
	return n.updated
}

func (n *NodeBase[Ctx]) getUpdateOrder() ([]Node[Ctx], error) {
	dependencies := make([]dependencyEntry[Ctx], 0, len(n.subscribers))
	n.getDependencies(&dependencies)

	stability := make([]Node[Ctx], 0, len(dependencies))

	for _, entry := range dependencies {
		stability = append(stability, entry.Node)
	}

	stability = utils.Uniq(stability)

	dependenciesMap := make(map[Node[Ctx]][]Node[Ctx], len(dependencies))

	for _, entry := range dependencies {
		dependenciesMap[entry.Node] = entry.Dependants
	}

	return utils.StableTopologicalSortWithSortedKeys(dependenciesMap, stability)
}

func (n *NodeBase[Ctx]) handleSubscriptionsUpdated(ctx Ctx, evtTime time.Time) {
	if n.onSubscriptionUpdated != nil {
		n.onSubscriptionUpdated(ctx, evtTime)
	}
}

func (n *NodeBase[Ctx]) NotifyUpdated(ctx Ctx, evtTime time.Time) {
	n.updated = true

	n.notifySubscribers()

	if n.subscriptionUpdated {
		return
	}

	n.processUpdate(ctx, evtTime)
}

func (n NodeBase[Ctx]) notifySubscribers() {
	for _, node := range n.subscribers {
		node.setSubscriptionUpdated(true)
	}
}

func (n *NodeBase[Ctx]) processUpdate(ctx Ctx, evtTime time.Time) {
	if n.treeUpdateOrder == nil {
		var err error
		if n.treeUpdateOrder, err = n.getUpdateOrder(); err != nil {
			panic(fmt.Sprintf("%+v", err))
		}
	}

	for _, node := range n.treeUpdateOrder {
		if node.hasUpdatedSubscription() {
			node.handleSubscriptionsUpdated(ctx, evtTime)
		}
		node.setSubscriptionUpdated(false)
	}

	for _, node := range n.treeUpdateOrder {
		node.setHasUpdated(false)
	}
}

func (n *NodeBase[Ctx]) String() string {
	return n.name + fmt.Sprintf("-%x", uintptr(unsafe.Pointer(n)))
}
