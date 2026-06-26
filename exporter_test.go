package main

import (
	"testing"
	"time"
)

func TestSampleIntervalSkipsFirstSampleAndLargeGaps(t *testing.T) {
	exporter := &Exporter{
		cfg:      Config{ScrapeInterval: 100 * time.Millisecond},
		lastSeen: make(map[string]time.Time),
	}

	start := time.Unix(100, 0)
	if got := exporter.sampleInterval("0", start); got != 0 {
		t.Fatalf("first interval = %v, want 0", got)
	}
	if got := exporter.sampleInterval("0", start.Add(200*time.Millisecond)); got < 0.199 || got > 0.201 {
		t.Fatalf("normal interval = %v, want 0.2", got)
	}
	if got := exporter.sampleInterval("0", start.Add(10*time.Second)); got != 0 {
		t.Fatalf("large gap interval = %v, want 0", got)
	}
}

func TestPercentFraction(t *testing.T) {
	tests := map[string]struct {
		value float64
		want  float64
	}{
		"negative": {-1, 0},
		"zero":     {0, 0},
		"half":     {50, 0.5},
		"full":     {100, 1},
		"over":     {120, 1},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := percentFraction(tt.value); got != tt.want {
				t.Fatalf("percentFraction(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestRatioFraction(t *testing.T) {
	tests := map[string]struct {
		value float64
		want  float64
	}{
		"negative": {-1, 0},
		"zero":     {0, 0},
		"ratio":    {0.75, 0.75},
		"percent":  {75, 0.75},
		"full":     {100, 1},
		"over":     {120, 1},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := ratioFraction(tt.value); got != tt.want {
				t.Fatalf("ratioFraction(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
