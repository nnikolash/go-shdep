package indicators

import (
	"context"
	"time"

	"github.com/nnikolash/go-shdep"
	"github.com/nnikolash/go-shdep/examples/trading/shobj"
	"github.com/nnikolash/go-shdep/objstore"
)

func NewMACrossIndicator(asset string, maPeriodFast, maPeriodSlow int) *MACrossIndicator {
	ind := &MACrossIndicator{
		// NOTE that ALL parameters are passed into NewSharedObjectBase.
		// This is crusial for the correct hash calculation. Otherwise, same hash may be
		// shared by different objects and your algorythm will work incorrectly.
		SharedObjectBase: shdep.NewSharedObjectBase[context.Context, *shobj.InitParams]("MACross", asset, maPeriodFast, maPeriodSlow),
		ma1: NewMAIndicator(PriceProviderConfig{
			Asset:  asset,
			Period: maPeriodFast,
		}),
		ma2: NewMAIndicator(PriceProviderConfig{
			Asset:  asset,
			Period: maPeriodSlow,
		}),
	}

	ind.SetUpdateHandler(ind.onDependenciesUpdated)

	return ind
}

type MACrossIndicator struct {
	shobj.SharedObjectBase

	ma1       *MAIndicator
	ma1Events *shdep.EventPuller[MAEvent]
	ma2       *MAIndicator
	ma2Events *shdep.EventPuller[MAEvent]

	state int
}

var _ shobj.SharedObject = &MACrossIndicator{}

func (o *MACrossIndicator) RegisterDependencies(store objstore.SharedStore[shobj.SharedObject, *shobj.InitParams]) {
	store.Register(&o.ma1)
	store.Register(&o.ma2)

	o.ma1.SubscribeObj(o)
	o.ma2.SubscribeObj(o)

	o.ma1Events = o.ma1.NewEventPuller()
	o.ma2Events = o.ma2.NewEventPuller()
}

func (o *MACrossIndicator) onDependenciesUpdated(ctx context.Context, evtTime time.Time) {
	// In this case it is not needed, but still we use even puller just as an example.

	var ma1 float64
	for _, e := range o.ma1Events.Pull() {
		ma1 = e.Event.MA
	}

	if ma1 == 0 {
		// ma1 was not yet updated
		return
	}

	var ma2 float64
	for _, e := range o.ma2Events.Pull() {
		ma2 = e.Event.MA
	}

	if ma2 == 0 {
		// ma2 was not yet updated
		return
	}

	var newState int
	if ma1 > ma2 {
		newState = 1
	} else if ma1 < ma2 {
		newState = -1
	} else {
		return
	}

	if o.state == 0 {
		o.state = newState
		return
	}

	if o.state == newState {
		return
	}

	o.state = newState
	o.NotifyUpdated(ctx, evtTime)
}

func (o *MACrossIndicator) GetCrossState() int {
	return o.state
}

func (o *MACrossIndicator) MAFast() *MAIndicator {
	return o.ma1
}

func (o *MACrossIndicator) MASlow() *MAIndicator {
	return o.ma2
}
