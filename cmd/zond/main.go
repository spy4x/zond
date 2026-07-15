// Command zond is an internal health probe bridge.
//
// It accepts external health-check requests and forwards them to internal
// Docker containers via Docker DNS names. Responses are limited to
// "ok" / "unreachable" / "unknown target" — no internal URLs leak out.
//
// Two operating modes:
//
//	zond              — run the HTTP server
//	zond -healthcheck — probe the running server (used by Docker HEALTHCHECK)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spy4x/zond/internal/config"
	"github.com/spy4x/zond/internal/probe"
	"github.com/spy4x/zond/internal/server"
)

const healthcheckTimeout = 2 * time.Second

func main() {
	healthcheck := flag.Bool("healthcheck", false, "Probe the running server (for Docker HEALTHCHECK). Exit 0 if reachable, 1 if not.")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *healthcheck {
		os.Exit(runHealthcheck(logger))
	}

	if err := run(logger); err != nil {
		logger.Error("zond failed", slog.Any("err", err))
		os.Exit(1)
	}
}

// runHealthcheck opens a TCP connection to the configured port.
// Distroless static images don't ship curl/wget — using a binary
// self-probe avoids baking a heavier base image just for health.
func runHealthcheck(logger *slog.Logger) int {
	port := os.Getenv("ZOND_PORT")
	if port == "" {
		port = fmt.Sprintf("%d", config.DefaultPort)
	}
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, healthcheckTimeout)
	if err != nil {
		logger.Warn("healthcheck failed", slog.String("addr", "127.0.0.1:"+port), slog.Any("err", err))
		return 1
	}
	_ = conn.Close()
	return 0
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	srv := server.New(server.Config{
		Targets: cfg.Targets,
		Checker: probe.New(),
		Logger:  logger,
	})

	httpSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		// Worst case: single longest probe (~5s default) + parallel fan-out overhead.
		// 15s leaves headroom for slow upstreams without holding stale connections.
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("zond listening", slog.Int("port", cfg.Port), slog.Int("targets", len(cfg.Targets)))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Info("zond stopped")
	return nil
}
