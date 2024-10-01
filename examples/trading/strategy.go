package example_trading

import (
	"context"
	"time"

	"github.com/nnikolash/go-shdep/examples/trading/indicators"
	"github.com/nnikolash/go-shdep/examples/trading/shobj"
	"github.com/nnikolash/go-shdep/objstore"
)

func NewStrategy(asset string, maPeriodFast, maPeriodSlow int) *Strategy {
	ind := &Strategy{
		SharedObjectBase: shobj.NewSharedObjectBase("Strategy", asset, maPeriodFast, maPeriodSlow),
		priceProvider:    indicators.NewPriceProvider(asset),
		cross:            indicators.NewMACrossIndicator(asset, maPeriodFast, maPeriodSlow),
	}

	ind.SetUpdateHandler(ind.onDependenciesUpdated)

	return ind
}

type Strategy struct {
	// Nobody is subscribing on strategy, but it still must implement SharedObject
	// interface to be able to subscribe on other objects.
	shobj.SharedObjectBase

	priceProvider *indicators.PriceProvider
	cross         *indicators.MACrossIndicator

	tradeOperationsLog []float64
}

var _ shobj.SharedObject = &Strategy{}

func (s *Strategy) RegisterDependencies(store objstore.SharedStore[shobj.SharedObject, *shobj.InitParams]) {
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

func (s *Strategy) Cross() *indicators.MACrossIndicator {
	return s.cross
}
