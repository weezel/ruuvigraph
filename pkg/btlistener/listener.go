package btlistener

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	ruuvipb "weezel/ruuvigraph/pkg/generated/ruuvi/ruuvi/v1"
	"weezel/ruuvigraph/pkg/logging"
	"weezel/ruuvigraph/pkg/ruuvi"

	"github.com/go-ble/ble"
	blelinux "github.com/go-ble/ble/linux"
	"github.com/peterhellberg/ruuvitag"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var logger *slog.Logger = logging.NewColorLogHandler()

type BtListener struct {
	ruuvipb.UnimplementedRuuviServer

	streamerClient ruuvipb.RuuviClient
	device         *blelinux.Device
	deviceAliases  map[string]string
	measurements   map[string]*ruuvipb.RuuviStreamDataRequest
	lock           sync.RWMutex
	ticker         *time.Ticker
}

func NewListener(streamerClient ruuvipb.RuuviClient) *BtListener {
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
		deviceAliases:  devAliases,
		streamerClient: streamerClient,
		measurements:   make(map[string]*ruuvipb.RuuviStreamDataRequest),
		ticker:         time.NewTicker(30 * time.Second),
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

func (b *BtListener) SendMeasurements(ctx context.Context) error {
	started := time.Now()
	b.lock.Lock()
	// Normalise timestamps. All measurements will use the same timestamp.
	for _, m := range b.measurements {
		m.Timestamp = timestamppb.New(started)
	}
	b.lock.Unlock()

	b.lock.RLock()
	defer b.lock.RUnlock()

	stream, err := b.streamerClient.StreamData(ctx)
	if err != nil {
		return fmt.Errorf("stream data: %w", err)
	}
	defer stream.CloseSend()

	for _, m := range b.measurements {
		logger.Info(
			"Sending data",
			slog.String("device", m.Device),
			slog.String("mac", m.MacAddress),
			slog.Time("timestamp", m.Timestamp.AsTime()),
		)
		if err = stream.Send(m); err != nil {
			logger.Warn(
				"Error sending data",
				slog.String("device", m.Device),
				slog.String("mac", m.MacAddress),
				slog.Any("error", err),
			)
		}
		logger.Info(
			"Sent data",
			slog.String("device", m.Device),
			slog.String("mac", m.MacAddress),
			slog.Time("timestamp", m.Timestamp.AsTime()),
		)
	}
	// Receive ACK
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("receive ack: %w", err)
	}

	logger.Info(fmt.Sprintf(
		"Server responded: %q and operations took %s",
		resp.GetMessage(),
	))

	return nil
}

// Listen starts listening bluetooth beacons and specifially Ruuvi ones.
// Filtering happens in function handleAdvertisement.
// Stopping is built-in for ble package so no need to build extra
// functionality for such purposes.
func (b *BtListener) Listen(ctx context.Context) {
	logger.Info("Scanning for RuuviTags (press Ctrl+C to stop)...")

	go func() {
		for {
			select {
			case <-ctx.Done():
				b.ticker.Stop()
				return
			case <-b.ticker.C:
				started := time.Now()
				logger.Info("Streaming results")
				if err := b.SendMeasurements(ctx); err != nil {
					logger.Error(
						"Failed to send measurements",
						slog.Any("error", err),
						slog.Int("count", len(b.measurements)),
					)
				}

				b.lock.Lock()
				logger.Info(
					"Streamed results",
					slog.Int("count", len(b.measurements)),
					slog.Duration("duration", time.Since(started)),
				)
				clear(b.measurements)
				b.lock.Unlock()
			}
		}
	}()

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
	flogger := logger.With("device", devName) // FIXME this is broken and doesn't work

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

	b.lock.Lock()
	defer b.lock.Unlock()
	logger.Info(fmt.Sprintf("Received measures for %s", devName))
	b.measurements[bleAdv.Addr().String()] = &ruuvipb.RuuviStreamDataRequest{
		Device:      devName,
		MacAddress:  bleAdv.Addr().String(),
		Temperature: float32(payload.Temperature),
		Humidity:    float32(payload.Humidity),
		Pressure:    float32(payload.Pressure) / 10.0,
		BatterVolts: float32(payload.Battery) / 1000.0,
		Rssi:        int32(bleAdv.RSSI()),
		Timestamp:   timestamppb.New(time.Now().Local()),
	}
}
