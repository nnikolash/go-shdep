package indicators

import (
	"context"
	"fmt"
	"time"

	"github.com/nnikolash/go-shdep"
	"github.com/nnikolash/go-shdep/examples/trading/shobj"
	"github.com/nnikolash/go-shdep/objstore"
)

// Calculates moving average of asset price.
func NewMAIndicator(cfg PriceProviderConfig) *MAIndicator {
	// NOTE that name may be constructed as you want, it exists just for convenience of distingushing objects.
	name := fmt.Sprintf("MA-%v/%v", cfg.Asset, cfg.Period)

	ind := &MAIndicator{
		// NOTE that the entire configuration is passed into NewSharedObjectBase.
		// This is crusial for the correct hash calculation. Otherwise, same hash may be
		// shared by different objects and your algorythm will work incorrectly.
		SharedObjectBase: shobj.NewSharedObjectBase(name, cfg),
		priceProvider:    NewPriceProvider(cfg.Asset),
		asset:            cfg.Asset,
		period:           cfg.Period,
		eventsPublisher:  shdep.NewEventsPullStorage[MAEvent](),
	}

	// NOTE that we must set this to be able to receive updates from dependencies.
	ind.SetUpdateHandler(ind.onDependenciesUpdated)

	return ind
}

type MAIndicator struct {
	shobj.SharedObjectBase

	asset           string
	period          int
	priceProvider   *PriceProvider
	eventsPublisher *shdep.EventsPullStorage[MAEvent]

	lastPrices   []float64
	currentValue float64
}

var _ shobj.SharedObject = &MAIndicator{}

type MAEvent struct {
	Asset  string
	Period int
	MA     float64
}

func (s *MAIndicator) NewEventPuller() *shdep.EventPuller[MAEvent] {
	return s.eventsPublisher.NewPuller()
}

func (o *MAIndicator) RegisterDependencies(store objstore.SharedStore[shobj.SharedObject, *shobj.InitParams]) {
	// NOTE that we pass pointer to pointer here.
	store.Register(&o.priceProvider)

	// Registration only provides us shared replica of that object.
	// But if we need to listen for updates, we must also subscribe on it.
	o.priceProvider.SubscribeObj(o)
}

// This function will be called when any of dependencies will call NotifyUpdated.
func (o *MAIndicator) onDependenciesUpdated(ctx context.Context, evtTime time.Time) {
	o.lastPrices = append(o.lastPrices, o.priceProvider.Price())

	if len(o.lastPrices) > o.period {
		o.lastPrices = o.lastPrices[1:]
	}

	o.currentValue = o.calculateValue()

	// In this example event published using pull model.
	// After event is published, dependencies will be notified about update and can pull this event.
	// NOTE that no need to lock here, because this update comes from inside of the update tree.

	o.eventsPublisher.Publish(MAEvent{
		Asset:  o.asset,
		Period: o.period,
		MA:     o.currentValue,
	})

	o.NotifyUpdated(ctx, evtTime)
}

func (o *MAIndicator) calculateValue() float64 {
	sum := 0.0
	count := 0

	for i := 0; i < len(o.lastPrices) && i < o.period; i++ {
		sum += o.lastPrices[i]
		count++
	}

	return sum / float64(count)
}

func (o *MAIndicator) Value() float64 {
	return o.currentValue
}
