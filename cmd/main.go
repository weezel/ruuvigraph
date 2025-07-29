package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"time"
	"weezel/ruuvigraph/pkg/btlistener"
	ruuvipb "weezel/ruuvigraph/pkg/generated/ruuvi/ruuvi/v1"
	"weezel/ruuvigraph/pkg/logging"

	"google.golang.org/grpc"
)

var logger *slog.Logger = logging.NewColorLogHandler()

var (
	grpcHost       = flag.String("h", "127.0.0.1", "Host where to serve or connect to")
	grpcPort       = flag.String("p", "50051", "Port where to serve or connect to")
	aliasesFile    = flag.String("a", "ruuvi_aliases.conf", "Aliases file for friendly names to devices")
	strictMatching = flag.Bool("s", true, "Only match devices which are listed in aliases configuration")    // TODO
	tickTime       = flag.Duration("t", 1*time.Minute, "Transmit measurements to server every N time units") // TODO
)

func main() {
	ctx := context.Background()

	flag.Parse()

	logger.Info("Collecting measurements")
	conn, err := grpc.NewClient(
		net.JoinHostPort(*grpcHost, *grpcPort),
		grpc.WithInsecure(),
	)
	if err != nil {
		logger.Error(
			"Failed to connect",
			slog.Any("error", err),
		)
		return
	}
	defer conn.Close()

	client := ruuvipb.NewRuuviClient(conn)

	cCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	btListener := btlistener.NewListener(client)

	if err := btListener.InitializeDevice(cCtx); err != nil {
		logger.Error(
			"Couldn't initialize device",
			slog.Any("error", err),
		)
		return
	}
	btListener.Listen(cCtx)

	logger.Info("Shutdown completed")
}
