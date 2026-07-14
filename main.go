package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg := LoadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	hostname, err := os.Hostname()
	if err != nil {
		logger.Warn("failed to resolve hostname", "error", err)
		hostname = "unknown"
	}

	client, err := NewDCGMClient(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize DCGM", "error", err)
		return 1
	}
	defer client.Close()
	logger.Info("DCGM initialized", "mode", cfg.DCGMMode, "version", version)

	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	exporter := NewExporter(cfg, client, metrics, hostname, logger)
	exporter.SetStaticMetrics()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := exporter.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("metrics collector stopped", "error", err)
			stop()
		}
	}()

	server := NewServer(cfg, exporter, registry, logger)
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run()
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			logger.Error("HTTP server failed", "error", err)
			return 1
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown failed", "error", err)
		return 1
	}

	logger.Info("GPU metrics exporter stopped")
	return 0
}
