package cache

import (
	"log/slog"
	"slices"
	"sync"
	"time"

	ruuvipb "weezel/ruuvigraph/pkg/generated/ruuvi/ruuvi/v1"
	"weezel/ruuvigraph/pkg/logging"
)

var logger *slog.Logger = logging.NewColorLogHandler()

type Measurements struct {
	data   []*ruuvipb.RuuviStreamDataRequest
	ticker *time.Ticker
	maxAge time.Duration
	lock   sync.RWMutex
	quit   chan struct{}
}

type OptionMeasurement func(mopt *Measurements)

func WithTickerRate(rate time.Duration) OptionMeasurement {
	return func(mopt *Measurements) {
		mopt.ticker = time.NewTicker(rate)
	}
}

func WithMaxMeasureAge(maxAge time.Duration) OptionMeasurement {
	return func(mopt *Measurements) {
		mopt.maxAge = maxAge
	}
}

func New(opts ...OptionMeasurement) *Measurements {
	m := &Measurements{
		data:   []*ruuvipb.RuuviStreamDataRequest{},
		quit:   make(chan struct{}),
		maxAge: time.Hour * 24 * 30,
		ticker: time.NewTicker(time.Minute * 10),
	}

	for _, opt := range opts {
		opt(m)
	}

	go m.run()

	return m
}

func (m *Measurements) Stop() {
	logger.Info("Shutting down measurements ticker")
	m.quit <- struct{}{}
	close(m.quit)
	logger.Info("Shat down measurements ticker")
}

func (m *Measurements) Add(req *ruuvipb.RuuviStreamDataRequest) {
	m.data = append(m.data, req)
}

func (m *Measurements) All() []*ruuvipb.RuuviStreamDataRequest {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.data
}

func (m *Measurements) run() {
	for {
		select {
		case <-m.quit:
			m.ticker.Stop()
		case <-m.ticker.C:
			m.lock.Lock()
			logger.Info(
				"Cleaning old measurements",
				slog.Int("len", len(m.data)),
			)
			m.data = m.pruneOldData()
			logger.Info(
				"Cleaned old measurements",
				slog.Int("len", len(m.data)),
			)
			m.lock.Unlock()
		}
	}
}

// pruneOldData method will be run by the ticker and is executed in scheduled manner.
// There shouldn't be a need to run this manually.
func (m *Measurements) pruneOldData() []*ruuvipb.RuuviStreamDataRequest {
	cutoff := time.Now().Add(-m.maxAge)

	return slices.DeleteFunc(m.data, func(d *ruuvipb.RuuviStreamDataRequest) bool {
		if d.Timestamp == nil {
			return true
		}
		return d.Timestamp.AsTime().Before(cutoff)
	})
}
