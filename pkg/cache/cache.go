package cache

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	ruuvipb "weezel/ruuvigraph/pkg/generated/ruuvi/ruuvi/v1"
	"weezel/ruuvigraph/pkg/logging"
)

var logger *slog.Logger = logging.NewColorLogHandler()

type Measurements struct {
	storeFilename *string
	ticker        *time.Ticker
	once          *sync.Once
	quit          chan struct{}
	data          atomic.Pointer[[]*ruuvipb.RuuviStreamDataRequest]
	maxAge        time.Duration
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

func WithArchiveFilename(fname string) OptionMeasurement {
	return func(mopt *Measurements) {
		fullPath, err := filepath.Abs(fname)
		if err != nil {
			logger.Error(
				"Couldn't get absolute file path for the archive file",
				slog.String("fname", fname),
				slog.Any("error", err),
			)
			return
		}
		mopt.storeFilename = &fullPath
	}
}

func New(opts ...OptionMeasurement) *Measurements {
	m := &Measurements{
		quit:   make(chan struct{}),
		once:   &sync.Once{},
		maxAge: time.Hour * 24 * 7,
		ticker: time.NewTicker(time.Minute * 5),
	}
	m.data.Store(&[]*ruuvipb.RuuviStreamDataRequest{})

	for _, opt := range opts {
		opt(m)
	}

	go m.run()

	return m
}

func (m *Measurements) Stop() {
	logger.Info("Shutting down measurements ticker")
	m.once.Do(func() {
		m.quit <- struct{}{}
	})
	logger.Info("Shat down measurements ticker")
}

func (m *Measurements) Add(req *ruuvipb.RuuviStreamDataRequest) {
	for {
		old := m.data.Load()
		newSlice := make([]*ruuvipb.RuuviStreamDataRequest, len(*old)+1)
		copy(newSlice, *old)
		newSlice[len(*old)] = req

		if m.data.CompareAndSwap(old, &newSlice) {
			return
		}
	}
}

func (m *Measurements) All() []*ruuvipb.RuuviStreamDataRequest {
	data := m.data.Load()
	// Return a copy to prevent external mutation
	copied := make([]*ruuvipb.RuuviStreamDataRequest, len(*data))
	copy(copied, *data)
	return copied
}

func (m *Measurements) run() {
	defer func() {
		m.ticker.Stop()
		close(m.quit)
	}()

	for {
		select {
		case <-m.quit:
			return
		case <-m.ticker.C:
			logger.Info(
				"Cleaning old measurements",
				slog.Int("len", len(*m.data.Load())),
			)

			wg := sync.WaitGroup{}
			if m.storeFilename != nil && *m.storeFilename != "" {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := m.archive(); err != nil {
						logger.Error(
							"Failed to write archive file",
							slog.Any("error", err),
						)
					}
				}()
			}

			countBefore := len(*m.data.Load())
			m.pruneOldData()
			removedItems := len(*m.data.Load()) - countBefore
			logger.Info(
				"Cleaned old measurements",
				slog.Int("removed_items", removedItems),
			)

			wg.Wait() // This is zero if no task has been launched, hence not blocking
		}
	}
}

func (m *Measurements) archive() error {
	logger.Info("Writing archive file")

	dataCopy := m.All()

	j, err := json.MarshalIndent(dataCopy, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal measurements: %w", err)
	}

	path := filepath.Dir(*m.storeFilename)
	basename := filepath.Base(*m.storeFilename)
	ext := filepath.Ext(basename)
	fname := strings.TrimSuffix(basename, ext)
	datenow := strings.ReplaceAll(time.Now().Local().Format(time.RFC3339), ":", "")
	generatedFname := fmt.Sprintf("%s_%s%s", fname, datenow, ext)
	fpath := filepath.Join(path, generatedFname)
	if err = os.WriteFile(fpath, j, 0o600); err != nil {
		return fmt.Errorf("write json: %w", err)
	}

	logger.Info(
		"Wrote archive file",
		slog.String("fname", fpath),
	)

	return nil
}

// pruneOldData method will be run by the ticker and is executed in scheduled manner.
// There shouldn't be a need to run this manually.
func (m *Measurements) pruneOldData() {
	cutoff := time.Now().Add(-m.maxAge)

	for {
		old := m.data.Load()
		original := *old

		// Copy original slice to avoid mutating shared memory
		copied := make([]*ruuvipb.RuuviStreamDataRequest, len(original))
		copy(copied, original)

		cleaned := slices.DeleteFunc(copied, func(d *ruuvipb.RuuviStreamDataRequest) bool {
			if d.Timestamp == nil {
				return true
			}
			return d.Timestamp.AsTime().Before(cutoff)
		})

		if m.data.CompareAndSwap(old, &cleaned) {
			break
		}
	}
}
