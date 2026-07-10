package main

import (
	"math"
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

func TestExporterReadyRequiresRecentSuccessfulCollectWithGPU(t *testing.T) {
	exporter := &Exporter{
		cfg: Config{ScrapeInterval: 100 * time.Millisecond},
	}

	now := time.Unix(100, 0)
	if exporter.Ready(now) {
		t.Fatal("Ready without successful collect = true, want false")
	}

	exporter.lastSuccess = now
	exporter.lastGPUCount = 1
	if !exporter.Ready(now.Add(1 * time.Second)) {
		t.Fatal("Ready after recent successful collect = false, want true")
	}

	if exporter.Ready(now.Add(10 * time.Second)) {
		t.Fatal("Ready after stale successful collect = true, want false")
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

func TestMiBToBytesReturnsNilForBlankValues(t *testing.T) {
	blank := int64(9223372036854775792)
	if got := mibToBytes(blank); got != nil {
		t.Fatalf("mibToBytes(blank) = %v, want nil", *got)
	}

	got := mibToBytes(2)
	if got == nil {
		t.Fatal("mibToBytes(2) = nil, want bytes")
	}
	if *got != 2*1024*1024 {
		t.Fatalf("mibToBytes(2) = %v, want %v", *got, float64(2*1024*1024))
	}
}

func TestRatio01PercentPointer(t *testing.T) {
	tests := map[string]struct {
		value float64
		want  *float64
	}{
		"zero":     {0, floatPtr(0)},
		"half":     {0.5, floatPtr(50)},
		"one":      {1, floatPtr(100)},
		"negative": {-0.01, nil},
		"over":     {1.01, nil},
		"nan":      {math.NaN(), nil},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := ratio01PercentPointer(tt.value)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("ratio01PercentPointer(%v) = %v, want nil", tt.value, *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("ratio01PercentPointer(%v) = %v, want %v", tt.value, valueOrNil(got), *tt.want)
			}
		})
	}
}

func TestLabelsIncludeStableGPUIdentifiers(t *testing.T) {
	labels := labelsFor(GPUInfo{
		Index:    "0",
		UUID:     "GPU-abc",
		PCIBusID: "00000000:01:00.0",
		Name:     "NVIDIA Test",
	}, "host-a")

	if labels["gpu_uuid"] != "GPU-abc" {
		t.Fatalf("gpu_uuid label = %q, want GPU-abc", labels["gpu_uuid"])
	}
	if labels["pci_bus_id"] != "00000000:01:00.0" {
		t.Fatalf("pci_bus_id label = %q, want 00000000:01:00.0", labels["pci_bus_id"])
	}
}

func TestLabelsWithCopiesBaseLabels(t *testing.T) {
	base := labelsFor(GPUInfo{
		Index:    "0",
		UUID:     "GPU-abc",
		PCIBusID: "00000000:01:00.0",
		Name:     "NVIDIA Test",
	}, "host-a")

	extended := labelsWith(base, "cause", "sbe")
	if extended["cause"] != "sbe" {
		t.Fatalf("extended cause label = %q, want sbe", extended["cause"])
	}
	if _, ok := base["cause"]; ok {
		t.Fatal("labelsWith mutated base labels")
	}
	if extended["gpu_uuid"] != base["gpu_uuid"] {
		t.Fatalf("extended gpu_uuid = %q, want %q", extended["gpu_uuid"], base["gpu_uuid"])
	}
}

func TestHardwareCounterKeyUsesStableGPUIdentifier(t *testing.T) {
	info := GPUInfo{
		Index:    "0",
		UUID:     "GPU-abc",
		PCIBusID: "00000000:01:00.0",
	}
	if got, want := hardwareCounterKey(info, "ecc", "volatile", "sbe"), "GPU-abc:ecc:volatile:sbe"; got != want {
		t.Fatalf("hardwareCounterKey = %q, want %q", got, want)
	}

	info.UUID = "unknown"
	if got, want := hardwareCounterKey(info, "energy"), "00000000:01:00.0:energy"; got != want {
		t.Fatalf("hardwareCounterKey without uuid = %q, want %q", got, want)
	}
}

func floatPtr(value float64) *float64 {
	return &value
}

func valueOrNil(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}
