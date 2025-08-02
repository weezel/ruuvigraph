package plot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
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

	server        *grpc.Server
	measureData   *cache.Measurements
	once          *sync.Once
	lastGenerated time.Time
	doPlot        chan time.Duration
	stop          chan struct{}
}

func NewPlottingServer() *PlottingServer {
	ps := &PlottingServer{
		server:        grpc.NewServer(),
		measureData:   cache.New(),
		lastGenerated: time.Now(),
		once:          &sync.Once{},
		doPlot:        make(chan time.Duration, 1),
		stop:          make(chan struct{}),
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
		close(p.stop)
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
			} else {
				p.lastGenerated = time.Now()
			}
			logger.Info("Plotted measurements")
		}
	}
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
		lastGenerated := time.Since(p.lastGenerated)
		if lastGenerated.Minutes() >= 1 {
			select {
			case p.doPlot <- lastGenerated:
			default: // Plot already scheduled, no need to enqueue another
			}
		}
	}
}
