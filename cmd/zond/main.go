// Command zond is an internal health probe bridge.
//
// It accepts external health-check requests and forwards them to internal
// Docker containers via Docker DNS names. Responses are limited to
// "ok" / "unreachable" / "unknown target" — no internal URLs leak out.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spy4x/zond/internal/config"
	"github.com/spy4x/zond/internal/probe"
	"github.com/spy4x/zond/internal/server"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("zond failed", slog.Any("err", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	checker := probe.New(config.DefaultProbeTimeout)
	srv := server.New(server.Config{
		Targets: toServerTargets(cfg.Targets),
		Checker: checker,
		Logger:  logger,
	})

	httpSrv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second, // accommodates slow per-target fan-out
		IdleTimeout:       60 * time.Second,
	}

	// Graceful shutdown on SIGINT / SIGTERM.
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

func toServerTargets(in []config.Target) []server.Target {
	out := make([]server.Target, len(in))
	for i, t := range in {
		out[i] = server.Target{Name: t.Name, URL: t.URL, Timeout: t.Timeout}
	}
	return out
}
