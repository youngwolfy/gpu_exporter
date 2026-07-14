package main

import (
	"encoding/binary"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NVIDIA/go-dcgm/pkg/dcgm"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type fakeSampleClient struct {
	batches []SampleBatch
	errs    []error
	index   int
}

func (f *fakeSampleClient) Samples() (SampleBatch, error) {
	index := f.index
	if index >= len(f.batches) {
		index = len(f.batches) - 1
	}
	f.index++
	if index < 0 {
		return SampleBatch{}, nil
	}
	var err error
	if index < len(f.errs) {
		err = f.errs[index]
	}
	return f.batches[index], err
}

func (*fakeSampleClient) StaticInfo() (string, string) { return "test-driver", "test-cuda" }

func testGPU(index, uuid string) GPUInfo {
	return GPUInfo{ID: 0, Index: index, UUID: uuid, PCIBusID: "00000000:01:00.0", Name: "NVIDIA Test"}
}

func newTestExporter(t *testing.T, client SampleClient) (*Exporter, *Metrics, *prometheus.Registry) {
	t.Helper()
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	cfg := Config{
		ScrapeInterval:      100 * time.Millisecond,
		ProfilingInterval:   500 * time.Millisecond,
		OperationalInterval: time.Second,
		ReliabilityInterval: 10 * time.Second,
		WindowInterval:      15 * time.Second,
		ActiveThreshold:     1,
		MinRequestTime:      50 * time.Millisecond,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewExporter(cfg, client, metrics, "host-a", logger), metrics, registry
}

func TestSampleIntervalSkipsFirstSampleAndLargeGaps(t *testing.T) {
	exporter := &Exporter{cfg: Config{ScrapeInterval: 100 * time.Millisecond}, lastSeen: make(map[string]time.Time)}
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

func TestExporterReadyRequiresCompleteRecentCollect(t *testing.T) {
	exporter := &Exporter{cfg: Config{ScrapeInterval: 100 * time.Millisecond}}
	now := time.Unix(100, 0)
	if exporter.Ready(now) {
		t.Fatal("Ready without successful collect = true, want false")
	}
	exporter.lastSuccess = now
	exporter.lastGPUCount = 1
	if !exporter.Ready(now.Add(time.Second)) {
		t.Fatal("Ready after recent complete collect = false, want true")
	}
	exporter.lastGPUCount = 0
	if exporter.Ready(now.Add(time.Second)) {
		t.Fatal("Ready after partial collect = true, want false")
	}
}

func TestFractionConvertersAreStrict(t *testing.T) {
	percentTests := map[float64]float64{-1: 0, 0: 0, 50: 0.5, 100: 1, 120: 1}
	for value, want := range percentTests {
		if got := percentFraction(value); got != want {
			t.Fatalf("percentFraction(%v) = %v, want %v", value, got, want)
		}
	}
	ratioTests := map[float64]float64{-1: 0, 0: 0, 0.75: 0.75, 1: 1, 75: 0}
	for value, want := range ratioTests {
		if got := ratioFraction(value); got != want {
			t.Fatalf("ratioFraction(%v) = %v, want %v", value, got, want)
		}
	}
	if got := ratio01Pointer(75); got != nil {
		t.Fatalf("ratio01Pointer(75) = %v, want nil", *got)
	}
}

func TestMiBAndRatioConversionRejectBlankOrOutOfRange(t *testing.T) {
	if got := mibToBytes(dcgm.DCGM_FT_INT64_BLANK); got != nil {
		t.Fatalf("mibToBytes(blank) = %v, want nil", *got)
	}
	if got := mibToBytes(2); got == nil || *got != 2*1024*1024 {
		t.Fatalf("mibToBytes(2) = %v, want %v", valueOrNil(got), float64(2*1024*1024))
	}
	tests := map[string]struct {
		value float64
		want  *float64
	}{
		"zero": {0, floatPtr(0)}, "half": {0.5, floatPtr(50)}, "one": {1, floatPtr(100)},
		"negative": {-0.01, nil}, "over": {1.01, nil}, "nan": {math.NaN(), nil},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := ratio01PercentPointer(tt.value)
			if (got == nil) != (tt.want == nil) || got != nil && *got != *tt.want {
				t.Fatalf("ratio01PercentPointer(%v) = %v, want %v", tt.value, valueOrNil(got), valueOrNil(tt.want))
			}
		})
	}
}

func TestApplyFieldValuesNanosecondsToSeconds(t *testing.T) {
	fields := []dcgm.Short{
		dcgm.DCGM_FI_DEV_POWER_VIOLATION,
		dcgm.DCGM_FI_DEV_THERMAL_VIOLATION,
		dcgm.DCGM_FI_DEV_SYNC_BOOST_VIOLATION,
		dcgm.DCGM_FI_DEV_BOARD_LIMIT_VIOLATION,
		dcgm.DCGM_FI_DEV_LOW_UTIL_VIOLATION,
		dcgm.DCGM_FI_DEV_RELIABILITY_VIOLATION,
		dcgm.DCGM_FI_DEV_TOTAL_APP_CLOCKS_VIOLATION,
		dcgm.DCGM_FI_DEV_TOTAL_BASE_CLOCKS_VIOLATION,
	}
	for _, fieldID := range fields {
		t.Run(fieldLabel(fieldID), func(t *testing.T) {
			sample := GPUSample{}
			observations := applyFieldValues([]dcgm.FieldValue_v1{intFieldValue(fieldID, 1_000_000_000, dcgm.DCGM_ST_OK, 1)}, &sample)
			target := sampleFieldTarget(&sample, fieldID)
			if len(observations) != 1 || target == nil || *target == nil || **target != 1 {
				t.Fatalf("1e9 ns decoded as observations=%v value=%v, want 1 second", len(observations), pointerPointerValue(target))
			}
		})
	}
}

func TestApplyFieldValuesRejectsStatusTypeAndDoubleBlank(t *testing.T) {
	tests := []struct {
		name  string
		value dcgm.FieldValue_v1
	}{
		{"non-ok status", intFieldValue(dcgm.DCGM_FI_DEV_GPU_UTIL, 50, dcgm.DCGM_ST_NO_DATA, 1)},
		{"wrong field type", floatFieldValue(dcgm.DCGM_FI_DEV_GPU_UTIL, 0.5, dcgm.DCGM_ST_OK, 1)},
		{"double blank", floatFieldValue(dcgm.DCGM_FI_DEV_POWER_USAGE, dcgm.DCGM_FT_FP64_BLANK, dcgm.DCGM_ST_OK, 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sample := GPUSample{}
			if got := applyFieldValues([]dcgm.FieldValue_v1{tt.value}, &sample); len(got) != 0 {
				t.Fatalf("decoded %d observations, want 0", len(got))
			}
		})
	}
}

func TestApplyFieldValuesProcessesHistoryInTimestampOrder(t *testing.T) {
	sample := GPUSample{}
	values := []dcgm.FieldValue_v1{
		intFieldValue(dcgm.DCGM_FI_DEV_GPU_UTIL, 80, dcgm.DCGM_ST_OK, 200_000),
		intFieldValue(dcgm.DCGM_FI_DEV_GPU_UTIL, 20, dcgm.DCGM_ST_OK, 100_000),
	}
	observations := applyFieldValues(values, &sample)
	if len(observations) != 2 || observations[0].Value != 20 || observations[1].Value != 80 {
		t.Fatalf("observations = %#v, want values [20,80]", observations)
	}
	if sample.Utilization == nil || *sample.Utilization != 80 {
		t.Fatalf("latest utilization = %v, want 80", valueOrNil(sample.Utilization))
	}
}

func TestGPUWatcherExcludesLinkAndNVSwitchFields(t *testing.T) {
	invalid := []dcgm.Short{
		dcgm.DCGM_FI_DEV_NVLINK_TX_BANDWIDTH_TOTAL,
		dcgm.DCGM_FI_DEV_NVLINK_RX_BANDWIDTH_TOTAL,
		dcgm.DCGM_FI_DEV_NVLINK_PPCNT_PLR_RCV_CODE_ERR,
		dcgm.DCGM_FI_DEV_NVLINK_PPCNT_PLR_RCV_UNCORRECTABLE_CODE,
		dcgm.DCGM_FI_DEV_NVLINK_PPCNT_PLR_XMIT_RETRY_CODES,
		dcgm.DCGM_FI_DEV_NVLINK_PPCNT_PLR_XMIT_RETRY_EVENTS,
		dcgm.DCGM_FI_DEV_NVLINK_PPCNT_PHYSICAL_LINK_DOWN_COUNTER,
	}
	for _, fieldID := range invalid {
		if containsField(monitoredFields(), fieldID) {
			t.Fatalf("GPU watcher includes non-GPU field %s", fieldLabel(fieldID))
		}
	}
	if !containsField(monitoredFields(), dcgm.DCGM_FI_PROF_NVLINK_TX_BYTES) {
		t.Fatal("GPU watcher does not include GPU-level DCP NVLink TX field")
	}
}

func TestProfilingGroupSelectionIsLargestAndTieStable(t *testing.T) {
	requested := []dcgm.Short{dcgm.DCGM_FI_PROF_SM_ACTIVE, dcgm.DCGM_FI_PROF_DRAM_ACTIVE, dcgm.DCGM_FI_PROF_PIPE_FP32_ACTIVE}
	groups := []dcgm.MetricGroup{
		{FieldIds: []uint{uint(dcgm.DCGM_FI_PROF_SM_ACTIVE), uint(dcgm.DCGM_FI_PROF_DRAM_ACTIVE)}},
		{FieldIds: []uint{uint(dcgm.DCGM_FI_PROF_SM_ACTIVE), uint(dcgm.DCGM_FI_PROF_PIPE_FP32_ACTIVE)}},
	}
	got := selectBestProfilingFields(requested, groups)
	if len(got) != 2 || got[0] != requested[0] || got[1] != requested[1] {
		t.Fatalf("selected fields = %v, want stable first tied group %v", got, requested[:2])
	}
}

func TestHardwareCounterCreatesZeroSeriesAndHandlesIncrementAndReset(t *testing.T) {
	exporter, metrics, _ := newTestExporter(t, &fakeSampleClient{})
	info := testGPU("0", "GPU-a")
	labels := labelsFor(info, "host-a")
	key := hardwareCounterKey(info, "pcie_replay")

	exporter.observeHardwareCounter(key, 0, metrics.GPUPCIeReplayTotal, labels)
	if got := testutil.ToFloat64(metrics.GPUPCIeReplayTotal.With(labels)); got != 0 {
		t.Fatalf("zero series = %v, want 0", got)
	}
	exporter.observeHardwareCounter(key, 5, metrics.GPUPCIeReplayTotal, labels)
	exporter.observeHardwareCounter(key, 8, metrics.GPUPCIeReplayTotal, labels)
	exporter.observeHardwareCounter(key, 2, metrics.GPUPCIeReplayTotal, labels)
	if got := testutil.ToFloat64(metrics.GPUPCIeReplayTotal.With(labels)); got != 10 {
		t.Fatalf("counter after increment and reset = %v, want 10", got)
	}
}

func TestPartialGPUFailureIsNotCompleteSuccess(t *testing.T) {
	good := testGPU("0", "GPU-good")
	bad := testGPU("1", "GPU-bad")
	client := &fakeSampleClient{batches: []SampleBatch{{
		Supported: []GPUInfo{good, bad},
		Samples:   []GPUSample{{Info: good}},
		Failures:  []GPUCollectionFailure{{Info: bad, Reason: "watch", Err: io.ErrUnexpectedEOF}},
	}}}
	exporter, metrics, _ := newTestExporter(t, client)
	if err := exporter.collect(); err != nil {
		t.Fatalf("collect() error = %v", err)
	}
	labels := exporterLabelsFor("host-a")
	if got := testutil.ToFloat64(metrics.ExporterCollectSuccess.With(labels)); got != 0 {
		t.Fatalf("collect_success = %v, want 0", got)
	}
	if got := testutil.ToFloat64(metrics.ExporterDiscoveredGPUs.With(labels)); got != 2 {
		t.Fatalf("discovered_gpus = %v, want 2", got)
	}
	if got := testutil.ToFloat64(metrics.ExporterCollectedGPUs.With(labels)); got != 1 {
		t.Fatalf("collected_gpus = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.ExporterFailedGPUs.With(labels)); got != 1 {
		t.Fatalf("failed_gpus = %v, want 1", got)
	}
	if exporter.Ready(time.Now()) {
		t.Fatal("Ready after partial collect = true, want false")
	}
}

func TestFieldAvailabilityMarksStaleGaugeWithoutInventingZero(t *testing.T) {
	exporter, metrics, _ := newTestExporter(t, &fakeSampleClient{})
	info := testGPU("0", "GPU-a")
	fieldID := dcgm.DCGM_FI_DEV_MEMORY_TEMP
	last := time.Unix(100, 0)
	exporter.collectSample(GPUSample{
		Info: info, MemoryTemperature: floatPtr(80),
		Fields: map[dcgm.Short]DCGMFieldStatus{fieldID: {Supported: true, Available: true, LastSuccess: last}},
	})
	exporter.collectSample(GPUSample{
		Info:   info,
		Fields: map[dcgm.Short]DCGMFieldStatus{fieldID: {Supported: true, Available: false, LastSuccess: last}},
	})
	labels := labelsFor(info, "host-a")
	if got := testutil.ToFloat64(metrics.GPUMemoryTemperature.With(labels)); got != 80 {
		t.Fatalf("stale gauge = %v, want last value 80", got)
	}
	statusLabels := labelsWith(labels, "field", fieldLabel(fieldID))
	if got := testutil.ToFloat64(metrics.GPUFieldAvailable.With(statusLabels)); got != 0 {
		t.Fatalf("field_available = %v, want 0", got)
	}
}

func TestDisappearingGPUDeletesAllMetricSeries(t *testing.T) {
	info := testGPU("0", "GPU-a")
	client := &fakeSampleClient{batches: []SampleBatch{
		{Supported: []GPUInfo{info}, Samples: []GPUSample{{Info: info, Utilization: floatPtr(25)}}},
		{},
	}}
	exporter, _, registry := newTestExporter(t, client)
	if err := exporter.collect(); err != nil {
		t.Fatal(err)
	}
	if err := exporter.collect(); err != nil {
		t.Fatal(err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		for _, metric := range family.Metric {
			for _, label := range metric.Label {
				if label.GetName() == "gpu_uuid" && label.GetValue() == "GPU-a" {
					t.Fatalf("stale GPU series remains in %s", family.GetName())
				}
			}
		}
	}
}

func TestHistoryFeedsWindowAndObservedTimeCounters(t *testing.T) {
	exporter, metrics, _ := newTestExporter(t, &fakeSampleClient{})
	info := testGPU("0", "GPU-a")
	start := time.Unix(100, 0)
	sample := GPUSample{Info: info, Observations: []FieldObservation{
		{FieldID: dcgm.DCGM_FI_DEV_GPU_UTIL, Timestamp: start, Value: 0},
		{FieldID: dcgm.DCGM_FI_DEV_GPU_UTIL, Timestamp: start.Add(100 * time.Millisecond), Value: 100},
	}}
	exporter.processObservations(sample)
	stats := exporter.window.Snapshot()[windowKey{gpuIndex: "GPU-a", metric: aggUtilization}]
	if stats.Count != 2 || stats.Max != 100 || stats.Avg() != 50 {
		t.Fatalf("window stats = %+v, want count=2 max=100 avg=50", stats)
	}
	labels := labelsFor(info, "host-a")
	if got := testutil.ToFloat64(metrics.Integrals[aggUtilization].Observed.With(labels)); math.Abs(got-0.1) > 1e-9 {
		t.Fatalf("observed seconds = %v, want 0.1", got)
	}
	if got := testutil.ToFloat64(metrics.Integrals[aggUtilization].Weighted.With(labels)); math.Abs(got-0.1) > 1e-9 {
		t.Fatalf("weighted seconds = %v, want 0.1", got)
	}
}

func TestHTTPReadyAndMetricsHandlerIsReadOnly(t *testing.T) {
	exporter, _, registry := newTestExporter(t, &fakeSampleClient{})
	info := testGPU("0", "GPU-a")
	exporter.activeGPUs[stableGPUIdentifier(info)] = info
	exporter.window.Observe(stableGPUIdentifier(info), aggUtilization, 42)
	server := NewServer(Config{}, exporter, registry, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ready := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if ready.Code != http.StatusServiceUnavailable {
		t.Fatalf("initial /ready status = %d, want 503", ready.Code)
	}
	exporter.lastSuccess = time.Now()
	exporter.lastGPUCount = 1
	ready = httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("successful /ready status = %d, want 200", ready.Code)
	}

	for range 2 {
		response := httptest.NewRecorder()
		server.httpServer.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("/metrics status = %d, want 200", response.Code)
		}
	}
	stats := exporter.window.Snapshot()[windowKey{gpuIndex: "GPU-a", metric: aggUtilization}]
	if stats.Count != 1 || stats.Max != 42 {
		t.Fatalf("HTTP requests mutated aggregation window: %+v", stats)
	}
}

func TestMetricRegistrationAndGatherHaveNoDuplicateDescriptors(t *testing.T) {
	registry := prometheus.NewRegistry()
	NewMetrics(registry)
	if _, err := registry.Gather(); err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
}

func TestLabelsAndHardwareKeyUseStableIdentifiers(t *testing.T) {
	info := testGPU("0", "GPU-abc")
	labels := labelsFor(info, "host-a")
	if labels["gpu_uuid"] != "GPU-abc" || labels["pci_bus_id"] != "00000000:01:00.0" {
		t.Fatalf("stable labels = %v", labels)
	}
	if got, want := hardwareCounterKey(info, "ecc", "volatile", "sbe"), "GPU-abc:ecc:volatile:sbe"; got != want {
		t.Fatalf("hardwareCounterKey = %q, want %q", got, want)
	}
	base := labelsFor(info, "host-a")
	extended := labelsWith(base, "cause", "sbe")
	if extended["cause"] != "sbe" {
		t.Fatal("labelsWith did not add label")
	}
	if _, ok := base["cause"]; ok {
		t.Fatal("labelsWith mutated base labels")
	}
}

func intFieldValue(fieldID dcgm.Short, value int64, status int, timestampMicroseconds int64) dcgm.FieldValue_v1 {
	result := dcgm.FieldValue_v1{FieldID: fieldID, FieldType: dcgm.DCGM_FT_INT64, Status: status, TS: timestampMicroseconds}
	binary.LittleEndian.PutUint64(result.Value[:8], uint64(value))
	return result
}

func floatFieldValue(fieldID dcgm.Short, value float64, status int, timestampMicroseconds int64) dcgm.FieldValue_v1 {
	result := dcgm.FieldValue_v1{FieldID: fieldID, FieldType: dcgm.DCGM_FT_DOUBLE, Status: status, TS: timestampMicroseconds}
	binary.LittleEndian.PutUint64(result.Value[:8], math.Float64bits(value))
	return result
}

func floatPtr(value float64) *float64 { return &value }

func valueOrNil(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func pointerPointerValue(value **float64) any {
	if value == nil || *value == nil {
		return nil
	}
	return **value
}
