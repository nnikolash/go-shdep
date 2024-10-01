package indicators

import (
	"context"
	"fmt"
	"time"

	"github.com/nnikolash/go-shdep/examples/trading/shobj"
)

// Monitors price of an asset and notified all dependencies it is updated.
func NewPriceProvider(asset string) *PriceProvider {
	name := fmt.Sprintf("PriceProvider-%v", asset)

	return &PriceProvider{
		SharedObjectBase: shobj.NewSharedObjectBase(name, asset),
		assetName:        asset,
		shouldStop:       make(chan struct{}),
	}
}

type PriceProvider struct {
	shobj.SharedObjectBase
	assetName string

	p *shobj.InitParams

	curentPrice float64
	shouldStop  chan struct{}
}

var _ shobj.SharedObject = &PriceProvider{}

func (p *PriceProvider) Init(params *shobj.InitParams) error {
	p.p = params
	return nil
}

func (p *PriceProvider) Start(params *shobj.InitParams) error {
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
