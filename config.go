package main

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

const (
	defaultListenAddress       = "127.0.0.1:9990"
	defaultScrapeInterval      = 100 * time.Millisecond
	defaultProfilingInterval   = 500 * time.Millisecond
	defaultOperationalInterval = time.Second
	defaultReliabilityInterval = 10 * time.Second
	defaultWindowInterval      = 15 * time.Second
	defaultActiveThreshold     = 1.0
	defaultMinRequestTime      = 50 * time.Millisecond
)

type Config struct {
	ListenAddress       string
	ScrapeInterval      time.Duration
	ProfilingInterval   time.Duration
	OperationalInterval time.Duration
	ReliabilityInterval time.Duration
	WindowInterval      time.Duration
	ActiveThreshold     float64
	MinRequestTime      time.Duration
	DCGMMode            string
	LogLevel            slog.Level
}

func LoadConfig() Config {
	return Config{
		ListenAddress:       stringEnv("GPU_EXPORTER_ADDR", defaultListenAddress),
		ScrapeInterval:      durationEnv("GPU_EXPORTER_SCRAPE_INTERVAL", defaultScrapeInterval),
		ProfilingInterval:   durationEnv("GPU_EXPORTER_PROFILING_INTERVAL", defaultProfilingInterval),
		OperationalInterval: durationEnv("GPU_EXPORTER_OPERATIONAL_INTERVAL", defaultOperationalInterval),
		ReliabilityInterval: durationEnv("GPU_EXPORTER_RELIABILITY_INTERVAL", defaultReliabilityInterval),
		WindowInterval:      durationEnv("GPU_EXPORTER_WINDOW_INTERVAL", defaultWindowInterval),
		ActiveThreshold:     floatEnv("GPU_EXPORTER_ACTIVE_THRESHOLD", defaultActiveThreshold),
		MinRequestTime:      durationEnv("GPU_EXPORTER_MIN_REQUEST_TIME", defaultMinRequestTime),
		DCGMMode:            stringEnv("GPU_EXPORTER_DCGM_MODE", "embedded"),
		LogLevel:            logLevelEnv("GPU_EXPORTER_LOG_LEVEL", slog.LevelInfo),
	}
}

func stringEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func floatEnv(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func logLevelEnv(key string, fallback slog.Level) slog.Level {
	switch os.Getenv(key) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info", "":
		return fallback
	default:
		return fallback
	}
}
