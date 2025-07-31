package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"time"

	"weezel/ruuvigraph/pkg/btlistener"
	ruuvipb "weezel/ruuvigraph/pkg/generated/ruuvi/ruuvi/v1"
	"weezel/ruuvigraph/pkg/logging"
	"weezel/ruuvigraph/pkg/plot"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var logger *slog.Logger = logging.NewColorLogHandler()

var (
	grpcHost       = flag.String("h", "127.0.0.1", "Host where to serve or connect to")
	grpcPort       = flag.String("p", "50051", "Port where to serve or connect to")
	aliasesFile    = flag.String("a", "ruuvi_aliases.conf", "Aliases file for friendly names to devices")
	runServer      = flag.Bool("s", false, "Run as a server & plotter")
	strictMatching = flag.Bool("S", false, "Only match devices which are listed in aliases configuration")   // TODO
	tickTime       = flag.Duration("t", 1*time.Minute, "Transmit measurements to server every N time units") // TODO
)

func runAsServer(ctx context.Context) {
	server := plot.NewPlottingServer()
	if err := server.Listen(ctx, *grpcHost, *grpcPort); err != nil {
		logger.Error(
			"Failed to start server on listen mode",
			slog.Any("error", err),
		)
		return
	}

	select {
	case <-ctx.Done():
		server.Stop()
	}
}

func runAsClient(ctx context.Context) {
	logger.Info("Collecting measurements")
	conn, err := grpc.NewClient(
		net.JoinHostPort(*grpcHost, *grpcPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		logger.Error(
			"Failed to connect",
			slog.Any("error", err),
		)
		return
	}
	defer conn.Close()
	logger.Info(fmt.Sprintf("Measurements receiving endpoint configured to %s:%s", *grpcHost, *grpcPort))

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
}

func main() {
	ctx := context.Background()

	flag.Parse()

	switch {
	case *runServer:
		runAsServer(ctx)
	default:
		runAsClient(ctx)
	}

	logger.Info("Shutdown completed")
}
