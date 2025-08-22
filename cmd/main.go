package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"weezel/ruuvigraph/pkg/btlistener"
	ruuvipb "weezel/ruuvigraph/pkg/generated/ruuvi/ruuvi/v1"
	"weezel/ruuvigraph/pkg/logging"
	"weezel/ruuvigraph/pkg/plot"
	"weezel/ruuvigraph/pkg/profiling"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Linker will fill these
var (
	Version   string
	BuildTime string
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
	pprofServer := profiling.NewPprofServer()
	pprofServer.Start()
	defer pprofServer.Shutdown(ctx)

	server := plot.NewPlottingServer()
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Listen(*grpcHost, *grpcPort)
	}()
	defer server.Stop()

	select {
	case <-ctx.Done():
		return
	case err := <-errCh:
		if err != nil {
			logger.Error(
				"Failed to start server on listen mode",
				slog.Any("error", err),
			)
			return
		}
	}
}

func runAsClient(ctx context.Context) {
	os.Setenv("TRACE_SERVER_PORT", "1338")
	pprofServer := profiling.NewPprofServer()
	pprofServer.Start()
	defer pprofServer.Shutdown(ctx)

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

	btListener := btlistener.NewListener(
		client,
		btlistener.WithAliasesFile(*aliasesFile),
	)

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

	logger.Info(
		"Version info",
		slog.String("build_time", BuildTime),
		slog.String("version", Version),
	)

	flag.Parse()

	switch {
	case *runServer:
		runAsServer(ctx)
	default:
		runAsClient(ctx)
	}

	logger.Info("Shutdown completed")
}
