package plot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"

	ruuvipb "weezel/ruuvigraph/pkg/generated/ruuvi/ruuvi/v1"
	"weezel/ruuvigraph/pkg/logging"

	"google.golang.org/grpc"
)

var logger *slog.Logger = logging.NewColorLogHandler()

type PlottingServer struct {
	ruuvipb.UnimplementedRuuviServer

	server *grpc.Server
}

func NewPlottingServer() *PlottingServer {
	ps := &PlottingServer{
		server: grpc.NewServer(),
	}

	ruuvipb.RegisterRuuviServer(ps.server, ps)
	return ps
}

func (p *PlottingServer) Listen(ctx context.Context, host, port string) error {
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

func (r *PlottingServer) StreamData(stream ruuvipb.Ruuvi_StreamDataServer) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return stream.SendAndClose(&ruuvipb.RuuviStreamDataResponse{
					Message: "OK",
				})
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
			slog.Int("rssi", int(msg.Rssi)),
			slog.Time("timestamp", msg.Timestamp.AsTime()),
		)
	}
}
