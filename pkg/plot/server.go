package plot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
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
	lastGenerated time.Time
}

func NewPlottingServer() *PlottingServer {
	ps := &PlottingServer{
		server:        grpc.NewServer(),
		measureData:   cache.New(),
		lastGenerated: time.Now(),
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

	logger.Info(fmt.Sprintf("gRPC server listening on %s", addr))
	if err = p.server.Serve(listen); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}

func (p *PlottingServer) Stop() {
	p.measureData.Stop()
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
					return fmt.Errorf("send and close: %w", err)
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
			slog.Time("timestamp", msg.Timestamp.AsTime()),
		)

		p.measureData.Add(msg)
		lastGenerated := time.Since(p.lastGenerated)
		if lastGenerated.Minutes() >= 1 {
			logger.Info("Generating results HTML file")
			if err = Plot(p.measureData.All()); err != nil {
				logger.Error(
					"Failed to generate plot",
					slog.Any("error", err),
					slog.Duration("last_generated", lastGenerated),
				)
			} else {
				logger.Info("Generated results HTML file")
				p.lastGenerated = time.Now()
			}

		}
	}
}
