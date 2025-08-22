package btlistener

import (
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

	streamerClient  ruuvipb.RuuviClient
	device          *blelinux.Device
	ticker          *time.Ticker
	deviceAliases   map[string]string
	aliasesFilename string
	measurements    sync.Map // key=string, value=*ruuvipb.RuuviStreamDataRequest
}

type ListenerOption func(*BtListener)

func WithAliasesFile(name string) ListenerOption {
	return func(bl *BtListener) {
		bl.aliasesFilename = name
	}
}

func NewListener(streamerClient ruuvipb.RuuviClient, opts ...ListenerOption) *BtListener {
	listener := &BtListener{
		streamerClient:  streamerClient,
		ticker:          time.NewTicker(10 * time.Minute),
		aliasesFilename: "ruuvi_aliases.conf",
	}

	for _, opt := range opts {
		opt(listener)
	}

	devAliases, err := ruuvi.ReadAliases(listener.aliasesFilename)
	if err != nil {
		logger.Error(
			"Failed to read aliases file",
			slog.Any("error", err),
		)
		os.Exit(1)
	}

	listener.deviceAliases = devAliases

	return listener
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

	// Normalise timestamps
	b.measurements.Range(func(_, value any) bool {
		if m, ok := value.(*ruuvipb.RuuviStreamDataRequest); ok {
			m.Timestamp = timestamppb.New(started)
		}
		return true
	})

	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()
	stream, err := b.streamerClient.StreamData(ctx)
	if err != nil {
		return fmt.Errorf("stream data: %w", err)
	}
	defer func() {
		if err = stream.CloseSend(); err != nil {
			logger.Error(
				"Failed to close stream",
				slog.Any("error", err),
			)
		}
	}()

	b.measurements.Range(func(_, value any) bool {
		m, ok := value.(*ruuvipb.RuuviStreamDataRequest)
		if !ok {
			return true
		}
		logger.Info(
			"Sending data",
			slog.String("device", m.Device),
			slog.String("mac", m.MacAddress),
			slog.Time("timestamp", m.Timestamp.AsTime().Local()),
		)
		if err = stream.Send(m); err != nil {
			logger.Warn(
				"Error sending data",
				slog.String("device", m.Device),
				slog.String("mac", m.MacAddress),
				slog.Any("error", err),
			)
			return true
		}

		logger.Info(
			"Sent data",
			slog.String("device", m.Device),
			slog.String("mac", m.MacAddress),
			slog.Time("timestamp", m.Timestamp.AsTime().Local()),
		)
		return true
	})

	// Receive ACK
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("receive ack: %w", err)
	}
	logger.Info(fmt.Sprintf("Server responded: %q", resp.GetMessage()))

	return nil
}

// Listen starts listening bluetooth beacons and specifially Ruuvi ones.
// Filtering happens in function handleAdvertisement.
// Stopping is built-in for ble package so no need to build extra
// functionality for such purposes.
func (b *BtListener) Listen(ctx context.Context) {
	defer func() {
		b.ticker.Stop()
	}()

	logger.Info("Scanning for RuuviTags (press Ctrl+C to stop)...")

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-b.ticker.C:
				b.handleMeasurementSending(ctx)
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

func (b *BtListener) handleMeasurementSending(ctx context.Context) {
	started := time.Now()
	countMeasurements := 0
	b.measurements.Range(func(_, _ any) bool {
		countMeasurements++
		return true
	})

	logger.Info("Streaming results")
	if err := b.SendMeasurements(ctx); err != nil {
		logger.Error(
			"Failed to send measurements",
			slog.Any("error", err),
			slog.Int("count", countMeasurements),
		)
		return
	}

	logger.Info(
		"Streamed results",
		slog.Int("count", countMeasurements),
		slog.Duration("duration", time.Since(started)),
	)

	// Clear measurements
	b.measurements.Range(func(key, _ any) bool {
		b.measurements.Delete(key)
		return true
	})
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

	logger.Info(fmt.Sprintf("Received measures for %s", devName))

	b.measurements.Store(
		bleAdv.Addr().String(),
		&ruuvipb.RuuviStreamDataRequest{
			Device:      devName,
			MacAddress:  bleAdv.Addr().String(),
			Temperature: float32(payload.Temperature),
			Humidity:    float32(payload.Humidity),
			Pressure:    float32(payload.Pressure) / 10.0,
			BatterVolts: float32(payload.Battery) / 1000.0,
			Rssi:        int32(bleAdv.RSSI()),
			Timestamp:   timestamppb.New(time.Now().Local()),
		},
	)
}
