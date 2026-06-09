package main

import (
	"sync"
	"time"
)

type PeakTracker struct {
	mu      sync.Mutex
	maximum map[string]float64
}

func NewPeakTracker() *PeakTracker {
	return &PeakTracker{maximum: make(map[string]float64)}
}

func (t *PeakTracker) Update(key string, value float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Keep the highest value seen inside the current scrape window.
	if value > t.maximum[key] {
		t.maximum[key] = value
	}
}

func (t *PeakTracker) GetAndReset(key string) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Prometheus receives the window peak, then the next window starts fresh.
	value := t.maximum[key]
	t.maximum[key] = 0
	return value
}

type ActivityTracker struct {
	mu              sync.Mutex
	states          map[string]gpuActivity
	activeThreshold float64
	minRequestTime  time.Duration
}

type gpuActivity struct {
	active     bool
	activeFrom time.Time
}

func NewActivityTracker(activeThreshold float64, minRequestTime time.Duration) *ActivityTracker {
	return &ActivityTracker{
		states:          make(map[string]gpuActivity),
		activeThreshold: activeThreshold,
		minRequestTime:  minRequestTime,
	}
}

func (t *ActivityTracker) Observe(gpuIndex string, util float64, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	state := t.states[gpuIndex]
	isActive := util > t.activeThreshold

	switch {
	case isActive && !state.active:
		// Rising edge: remember when GPU activity started.
		state.active = true
		state.activeFrom = now
	case !isActive && state.active:
		// Falling edge: count one request only if activity lasted long enough.
		wasRequest := now.Sub(state.activeFrom) >= t.minRequestTime
		state.active = false
		t.states[gpuIndex] = state
		return wasRequest
	}

	t.states[gpuIndex] = state
	return false
}
