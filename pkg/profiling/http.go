package profiling

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // It's purposely exposed
	"os"
	"runtime"
	"sync"
	"time"
	"weezel/ruuvigraph/pkg/logging"
)

var logger *slog.Logger = logging.NewColorLogHandler()

type PprofServer struct {
	server     *http.Server
	once       *sync.Once
	listenAddr string
}

// NewPprofServer provides new debug http server
func NewPprofServer() *PprofServer {
	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	hostname := cmp.Or(os.Getenv("TRACE_SERVER_HOST"), "127.0.0.1")
	port := cmp.Or(os.Getenv("TRACE_SERVER_PORT"), "1337")
	listenAddress := net.JoinHostPort(hostname, port)
	return &PprofServer{
		listenAddr: listenAddress,
		once:       &sync.Once{},
		server: &http.Server{
			Addr:              listenAddress,
			Handler:           http.DefaultServeMux,
			ReadHeaderTimeout: time.Second * 30,
		},
	}
}

func (p *PprofServer) Start() {
	go func() {
		logger.Info(fmt.Sprintf("Starting pprofiling server on %s", p.listenAddr))
		err := p.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(
				"Pprofiling server closed",
				slog.Any("error", err),
			)
		}
	}()
}

func (p *PprofServer) Shutdown(ctx context.Context) {
	p.once.Do(func() {
		cCtx, cancel := context.WithTimeoutCause(
			ctx,
			time.Second*3,
			errors.New("debug server shutdown timeout"),
		)
		defer cancel()
		logger.Info("Closing pprofiling server")
		if err := p.server.Shutdown(cCtx); err != nil {
			logger.Error(
				"Failed to shutdown profiling server",
				slog.Any("error", err),
			)
		}
	})
}
