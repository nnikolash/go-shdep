package example_trading_test

import (
	"reflect"
	"sync"
	"testing"

	example_trading "github.com/nnikolash/go-shdep/examples/trading"
	"github.com/nnikolash/go-shdep/examples/trading/shobj"
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

var btcPrices = []float64{
	1, 2, 3, 4, 5, 6, 7, 8, 9, 10, // price was going up
	9.5, 8.5, 7.5, 6.5, 5.5, 4.5, 3.5, 2.5, 1.5, // and then went down
}

func TestExampleTrading(t *testing.T) {
	t.Parallel()

	store := shobj.NewSharedStore(nil)

	strat1 := example_trading.NewStrategy("BTC", 2, 5)
	strat2 := example_trading.NewStrategy("BTC", 5, 10)

	store.Register(&strat1)
	store.Register(&strat2)

	done := make(chan struct{})

	err := store.Init(&shobj.InitParams{
		ExternalUpdateLock: &sync.Mutex{},
		GetPriceTicker: func(asset string) chan float64 {
			ch := make(chan float64)

			go func() {
				for _, price := range btcPrices {
					ch <- price
				}

				close(ch)
				close(done) // This is really bad way to implement price ticker, but it's just an example.
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
	var maAddrInStrat1 = reflect.ValueOf(strat1.Cross().MASlow()).Pointer()
	var maAddrInStrat2 = reflect.ValueOf(strat2.Cross().MAFast()).Pointer()
	require.Equal(t, maAddrInStrat1, maAddrInStrat2)
}
