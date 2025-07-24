package btlistener

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"weezel/ruuvigraph/pkg/logging"
	"weezel/ruuvigraph/pkg/ruuvi"

	"github.com/go-ble/ble"
	blelinux "github.com/go-ble/ble/linux"
	"github.com/peterhellberg/ruuvitag"
)

var logger *slog.Logger = logging.NewColorLogHandler()

type BtListener struct {
	device        *blelinux.Device
	once          *sync.Once
	deviceAliases map[string]string
}

func NewListener() *BtListener {
	aliasesFname := cmp.Or(os.Getenv("ALIASES_FILE"), "ruuvi_aliases.conf")
	devAliases, err := ruuvi.ReadAliases(aliasesFname)
	if err != nil {
		logger.Error(
			"Failed to read aliases file",
			slog.Any("error", err),
		)
		os.Exit(1)
	}

	return &BtListener{
		once:          &sync.Once{},
		deviceAliases: devAliases,
	}
}

func (b *BtListener) InitializeDevice(ctx context.Context) error {
	dev, err := blelinux.NewDevice()
	if err != nil {
		return fmt.Errorf("initialize bluetooth device: %w", err)
	}
	b.device = dev
	ble.SetDefaultDevice(b.device)

	return nil
}

// Listen starts listening bluetooth beacons and specifially Ruuvi ones.
// Filtering happens in function handleAdvertisement.
// Stopping is built-in for ble package so no need to build extra
// functionality for such purposes.
func (b *BtListener) Listen(ctx context.Context) {
	logger.Info("Scanning for RuuviTags (press Ctrl+C to stop)...")

	go func() {
		err := ble.Scan(ctx, true, b.handleAdvertisement, nil)
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("Scan failed", slog.Any("error", err))
		}
		logger.Info("Scanning stopped")
	}()

	<-ctx.Done()
}

func (b *BtListener) handleAdvertisement(bleAdv ble.Advertisement) {
	var found bool
	var devName string
	if devName, found = b.deviceAliases[bleAdv.Addr().String()]; !found {
		return
	}
	flogger := logger.With("device", devName) // FIXME

	mfData := bleAdv.ManufacturerData()
	if len(mfData) == 0 {
		flogger.Warn("Manufacturing data was empty")
		return
	}
	payload, err := ruuvitag.ParseRAWv2(mfData)
	if err != nil {
		flogger.Error(
			"Failed to parse tag",
			slog.Any("error", err),
		)
		return
	}

	data := ruuvi.Data{}.MergeRuuviRaw2AndBleAdv(payload, bleAdv, devName)
	flogger.Info(data.String())
}
