package examples_test

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/nnikolash/go-shdep"
	"github.com/nnikolash/go-shdep/objstore"
	"github.com/stretchr/testify/require"
)

// This is an example of how to use shared objects and update tree to build a simple hierarchy of trading indicators.
// Here same price provider is used by two moving average indicators, which in turn are used by MACross indicator.
//
//
//                  /----> MA1
//                 /           \
//                /             \
//               /               ---> MACross1 --> Strategy1
//              /               /
//             /               /
//   Price --> ----------> MA2
//             \               \
//              \               \
//               \               ---> MACross2 --> Strategy2
//                \             /
//                 \           /
//                  \----> MA3
//

type SharedObject = shdep.SharedObject[context.Context, *InitParams]
type SharedObjectBase = shdep.SharedObjectBase[context.Context, *InitParams]
type SharedStore = objstore.SharedStore[SharedObject, *InitParams]

var NewSharedStore = shdep.NewSharedStore[context.Context, *InitParams]
var NewSharedObjectBase = shdep.NewSharedObjectBase[context.Context, *InitParams]
var _ SharedObject = &SharedObjectBase{}

type InitParams struct {
	// Shared objects update tree is not thread-safe, so we need a lock to protect it.
	// This lock should be used only for updates, that come outside of update tree itself.
	// Another option to achieve same effect is to make external sources of updates single-threaded
	// if they provide updates via subscription and not as in this example by polling.
	ExternalUpdateLock *sync.Mutex

	GetPriceTicker func(asset string) chan float64
}

// Monitors price of an asset and notified all dependencies it is updated.
func NewPriceProvider(asset string) *PriceProvider {
	name := fmt.Sprintf("PriceProvider-%v", asset)

	return &PriceProvider{
		SharedObjectBase: NewSharedObjectBase(name, asset),
		assetName:        asset,
		shouldStop:       make(chan struct{}),
	}
}

type PriceProvider struct {
	SharedObjectBase
	assetName string

	p *InitParams

	curentPrice float64
	shouldStop  chan struct{}
}

var _ SharedObject = &PriceProvider{}

func (p *PriceProvider) Init(params *InitParams) error {
	p.p = params
	return nil
}

func (p *PriceProvider) Start(params *InitParams) error {
	getPrice := p.p.GetPriceTicker(p.assetName)

	go func() {
		for {
			select {
			case currentPrice, running := <-getPrice:
				if !running {
					return
				}

				p.curentPrice = currentPrice

				p.p.ExternalUpdateLock.Lock()
				p.NotifyUpdated(context.Background(), time.Now())
				p.p.ExternalUpdateLock.Unlock()
			case <-p.shouldStop:
				return
			}
		}
	}()

	return nil
}

func (p *PriceProvider) Price() float64 {
	return p.curentPrice
}

type PriceProviderConfig struct {
	Asset  string
	Period int
}

// Calculates moving average of asset price.
func NewMAIndicator(cfg PriceProviderConfig) *MAIndicator {
	// NOTE that name may be constructed as you want, it exists just for convenience of distingushing objects.
	name := fmt.Sprintf("MA-%v/%v", cfg.Asset, cfg.Period)

	ind := &MAIndicator{
		// NOTE that the entire configuration is passed into NewSharedObjectBase.
		// This is crusial for the correct hash calculation. Otherwise, same hash may be
		// shared by different objects and your algorythm will work incorrectly.
		SharedObjectBase: NewSharedObjectBase(name, cfg),
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
	SharedObjectBase

	asset           string
	period          int
	priceProvider   *PriceProvider
	eventsPublisher *shdep.EventsPullStorage[MAEvent]

	lastPrices   []float64
	currentValue float64
}

var _ SharedObject = &MAIndicator{}

type MAEvent struct {
	Asset  string
	Period int
	MA     float64
}

func (s *MAIndicator) NewEventPuller() *shdep.EventPuller[MAEvent] {
	return s.eventsPublisher.NewPuller()
}

func (o *MAIndicator) RegisterDependencies(store objstore.SharedStore[SharedObject, *InitParams]) {
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

func NewMACrossIndicator(asset string, maPeriodFast, maPeriodSlow int) *MACrossIndicator {
	ind := &MACrossIndicator{
		// NOTE that ALL parameters are passed into NewSharedObjectBase.
		// This is crusial for the correct hash calculation. Otherwise, same hash may be
		// shared by different objects and your algorythm will work incorrectly.
		SharedObjectBase: shdep.NewSharedObjectBase[context.Context, *InitParams]("MACross", asset, maPeriodFast, maPeriodSlow),
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
	SharedObjectBase

	ma1       *MAIndicator
	ma1Events *shdep.EventPuller[MAEvent]
	ma2       *MAIndicator
	ma2Events *shdep.EventPuller[MAEvent]

	state int
}

var _ SharedObject = &MACrossIndicator{}

func (o *MACrossIndicator) RegisterDependencies(store objstore.SharedStore[SharedObject, *InitParams]) {
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

func NewStrategy(asset string, maPeriodFast, maPeriodSlow int) *Strategy {
	ind := &Strategy{
		SharedObjectBase: NewSharedObjectBase("Strategy", asset, maPeriodFast, maPeriodSlow),
		priceProvider:    NewPriceProvider(asset),
		cross:            NewMACrossIndicator(asset, maPeriodFast, maPeriodSlow),
	}

	ind.SetUpdateHandler(ind.onDependenciesUpdated)

	return ind
}

type Strategy struct {
	// Nobody is subscribing on strategy, but it still must implement SharedObject
	// interface to be able to subscribe on other objects.
	SharedObjectBase

	priceProvider *PriceProvider
	cross         *MACrossIndicator

	tradeOperationsLog []float64
}

var _ SharedObject = &Strategy{}

func (s *Strategy) RegisterDependencies(store objstore.SharedStore[SharedObject, *InitParams]) {
	store.Register(&s.priceProvider)
	store.Register(&s.cross)

	// NOTE that we are specifically not subscribing on price provider here.
	// We are only interested in updated from cross indicator. Price provider only used
	// to get current price when cross indicator is updated.
	// BUT we still registered it, because we need shared replica which everyone else also uses.
	s.cross.SubscribeObj(s)
}

func (s *Strategy) onDependenciesUpdated(ctx context.Context, evtTime time.Time) {
	crossState := s.cross.GetCrossState()

	if crossState == 1 {
		s.buy(1)
	} else if crossState == -1 {
		s.sell(1)
	}
}

func (s *Strategy) buy(amount float64) {
	currentPrice := s.priceProvider.Price()
	s.tradeOperationsLog = append(s.tradeOperationsLog, amount*currentPrice)
}

func (s *Strategy) sell(amount float64) {
	currentPrice := s.priceProvider.Price()
	s.tradeOperationsLog = append(s.tradeOperationsLog, -amount*currentPrice)
}

func (s *Strategy) TradeOperationsLog() []float64 {
	return s.tradeOperationsLog
}

var btcPrices = []float64{
	1, 2, 3, 4, 5, 6, 7, 8, 9, 10, // price was going up
	9.5, 8.5, 7.5, 6.5, 5.5, 4.5, 3.5, 2.5, 1.5, // and then went down
}

func TestExample1(t *testing.T) {
	t.Parallel()

	store := NewSharedStore(nil)

	strat1 := NewStrategy("BTC", 2, 5)
	strat2 := NewStrategy("BTC", 5, 10)

	store.Register(&strat1)
	store.Register(&strat2)

	done := make(chan struct{})

	err := store.Init(&InitParams{
		ExternalUpdateLock: &sync.Mutex{},
		GetPriceTicker: func(asset string) chan float64 {
			ch := make(chan float64)

			go func() {
				for _, price := range btcPrices {
					ch <- price
				}

				close(ch)
				close(done)
			}()

			return ch
		},
	})
	require.NoError(t, err)

	err = store.Start()
	require.NoError(t, err)

	<-done

	store.Stop()
	store.Close()

	log1 := strat1.TradeOperationsLog()
	require.Equal(t, []float64{-7.5}, log1)
	// Strategy1 entered short position at price 7.5.

	log2 := strat2.TradeOperationsLog()
	require.Equal(t, []float64{-5.5}, log2)
	// Strategy2 entered short position at price 5.5. Later than Strategy1,
	// because it uses longer moving averages.

	// Lets check that same MA(5) object was by both strat1 and strat2.
	var maAddrInStrat1 = reflect.ValueOf(strat1.cross.ma2).Pointer()
	var maAddrInStrat2 = reflect.ValueOf(strat2.cross.ma1).Pointer()
	require.Equal(t, maAddrInStrat1, maAddrInStrat2)
}
