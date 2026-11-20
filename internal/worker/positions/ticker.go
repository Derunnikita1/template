package positions

import "time"

type systemTickerFactory struct{}

type systemTicker struct {
	ticker *time.Ticker
}

func NewSystemTickerFactory() tickerFactory {
	return systemTickerFactory{}
}

func (systemTickerFactory) NewTicker(interval time.Duration) ticker {
	return systemTicker{ticker: time.NewTicker(interval)}
}

func (t systemTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t systemTicker) Stop() {
	t.ticker.Stop()
}
