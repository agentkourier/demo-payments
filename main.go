// Command payments is the payments service: it authorizes payments for orders.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	logger := slog.New(traceLogHandler{slog.NewJSONHandler(os.Stdout, nil)}).
		With("service", serviceName, "version", version)
	if err := run(logger); err != nil {
		logger.Error("payments exited", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := setupTracing(ctx)
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(ctx); err != nil {
			logger.Error("flush traces", "error", err)
		}
	}()

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	var ready atomic.Bool
	reg := prometheus.NewRegistry()
	srv := &http.Server{
		Addr:              addr,
		Handler:           newRouter(logger, reg, &ready),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", addr)
		errc <- srv.ListenAndServe()
	}()
	ready.Store(true)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	ready.Store(false)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// newRouter returns the service's HTTP handler. Requests other than probes and
// metrics scrapes get a server span; every request is counted and timed.
func newRouter(logger *slog.Logger, reg *prometheus.Registry, ready *atomic.Bool) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST /v1/authorize", &authorizeHandler{logger: logger})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	return otelhttp.NewHandler(instrument(logger, newMetrics(reg), mux), serviceName,
		otelhttp.WithFilter(func(r *http.Request) bool { return !isProbe(r.URL.Path) }))
}
