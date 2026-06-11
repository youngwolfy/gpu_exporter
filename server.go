package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	httpServer *http.Server
	exporter   *Exporter
	handler    http.Handler
	logger     *slog.Logger
}

func NewServer(cfg Config, exporter *Exporter, gatherer prometheus.Gatherer, logger *slog.Logger) *Server {
	mux := http.NewServeMux()
	server := &Server{
		exporter: exporter,
		handler:  promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}),
		logger:   logger,
	}

	mux.HandleFunc("/metrics", server.metricsHandler())
	mux.HandleFunc("/health", healthHandler)

	server.httpServer = &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return server
}

func (s *Server) Run() error {
	s.logger.Info("starting GPU metrics exporter", "address", s.httpServer.Addr)
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) metricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Перед сбором registry публикуем накопленную с прошлого скрейпа
		// оконную статистику (*_max, *_avg) и начинаем новое окно.
		// Окно сбрасывается при каждом обращении.
		s.exporter.FlushWindow()
		s.handler.ServeHTTP(w, r)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK\n"))
}
