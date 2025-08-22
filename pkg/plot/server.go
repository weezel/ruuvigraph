package plot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"weezel/ruuvigraph/pkg/cache"
	ruuvipb "weezel/ruuvigraph/pkg/generated/ruuvi/ruuvi/v1"
	"weezel/ruuvigraph/pkg/logging"

	"google.golang.org/grpc"
)

var logger *slog.Logger = logging.NewColorLogHandler()

type PlottingServer struct {
	ruuvipb.UnimplementedRuuviServer

	storeFilename *string
	server        *grpc.Server
	measureData   *cache.Measurements
	once          *sync.Once
	lastGenerated time.Time
	doPlot        chan time.Duration
	stop          chan struct{}
}

type OptionServer func(pOpt *PlottingServer)

func WithArchiveFilename(fname string) OptionServer {
	return func(psopt *PlottingServer) {
		fullPath, err := filepath.Abs(fname)
		if err != nil {
			logger.Error(
				"Couldn't get absolute file path for the archive file",
				slog.String("fname", fname),
				slog.Any("error", err),
			)
			return
		}
		psopt.storeFilename = &fullPath
	}
}

func NewPlottingServer() *PlottingServer {
	ps := &PlottingServer{
		server:        grpc.NewServer(),
		measureData:   cache.New(),
		lastGenerated: time.Now(),
		once:          &sync.Once{},
		doPlot:        make(chan time.Duration, 1),
		stop:          make(chan struct{}, 1),
	}

	ruuvipb.RegisterRuuviServer(ps.server, ps)
	return ps
}

func (p *PlottingServer) Listen(host, port string) error {
	addr := net.JoinHostPort(host, port)

	listen, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("net listen: %w", err)
	}

	logger.Info("Starting plotter service")
	go p.plotter()
	logger.Info("Started plotter service")

	logger.Info(fmt.Sprintf("gRPC server listening on %s", addr))
	errCh := make(chan error, 1)
	go func() {
		if err = p.server.Serve(listen); err != nil {
			errCh <- fmt.Errorf("serve grpc: %w", err)
		}
	}()

	return <-errCh
}

func (p *PlottingServer) Stop() {
	p.once.Do(func() {
		logger.Info("Shutting down plotting service")
		p.measureData.Stop()
		p.stop <- struct{}{}
		logger.Info("Shutting down plotting service")
	})
}

func (p *PlottingServer) plotter() {
	defer func() {
		logger.Info("Stopping plotter")

		if p.server != nil {
			logger.Info("Stopping gRPC server")
			p.server.GracefulStop()
			logger.Info("Stopped gRPC server")
		}
		close(p.doPlot)
		close(p.stop)
	}()

	for {
		select {
		case <-p.stop:
			return
		case lastGenerated := <-p.doPlot:
			logger.Info("Plotting measurements")
			if err := Plot(p.measureData.All()); err != nil {
				logger.Error(
					"Failed to generate plot",
					slog.Any("error", err),
					slog.Duration("last_generated", lastGenerated),
				)
				continue
			}

			p.lastGenerated = time.Now()

			wg := sync.WaitGroup{}
			if p.storeFilename != nil && *p.storeFilename != "" {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := p.archive(); err != nil {
						logger.Error(
							"Failed to write archive file",
							slog.Any("error", err),
						)
					}
				}()
			}
			wg.Wait() // This is zero if no task has been launched, hence not blocking
			logger.Info("Plotted measurements")
		}
	}
}

func (p *PlottingServer) archive() error {
	logger.Info("Writing archive file")

	dataCopy := p.measureData.All()

	j, err := json.MarshalIndent(dataCopy, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal measurements: %w", err)
	}

	if err = os.WriteFile(*p.storeFilename, j, 0o600); err != nil {
		return fmt.Errorf("write json: %w", err)
	}

	logger.Info(
		"Wrote archive file",
		slog.String("fpath", *p.storeFilename),
	)

	return nil
}

func (p *PlottingServer) StreamData(stream ruuvipb.Ruuvi_StreamDataServer) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				err1 := stream.SendAndClose(&ruuvipb.RuuviStreamDataResponse{
					Message: "OK",
				})
				if err1 != nil {
					return fmt.Errorf("send and close: %w (%w)", err1, err)
				}
				return nil
			}
			return fmt.Errorf("stream receive error: %w", err)
		}

		logger.Info(
			"Received measurement",
			slog.String("device", msg.Device),
			slog.String("mac", msg.MacAddress),
			slog.Float64("temperature", float64(msg.Temperature)),
			slog.Float64("humidity", float64(msg.Humidity)),
			slog.Float64("pressure", float64(msg.Pressure)),
			slog.Float64("battery_volts", float64(msg.BatterVolts)),
			slog.Int("rssi", int(msg.Rssi)),
			slog.Time("timestamp", msg.Timestamp.AsTime().Local()),
		)

		p.measureData.Add(msg)

		if time.Since(p.lastGenerated) >= time.Minute {
			p.lastGenerated = time.Now()
			select {
			case p.doPlot <- time.Since(p.lastGenerated):
			default: // Plot already scheduled, no need to enqueue another
			}
		}
	}
}
