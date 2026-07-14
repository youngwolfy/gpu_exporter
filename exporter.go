package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/NVIDIA/go-dcgm/pkg/dcgm"
	"github.com/prometheus/client_golang/prometheus"
)

type SampleClient interface {
	Samples() (SampleBatch, error)
	StaticInfo() (driverVersion, cudaVersion string)
}

type Exporter struct {
	cfg      Config
	client   SampleClient
	metrics  *Metrics
	window   *WindowAggregator
	activity *ActivityTracker
	hostname string
	logger   *slog.Logger

	hardwareCounterMu sync.Mutex
	hardwareCounters  map[string]float64
	observationMu     sync.Mutex
	observationTimes  map[string]time.Time

	// Сохранено для обратной совместимости unit-тестов старой sampling-логики.
	lastSeen map[string]time.Time

	infoMu     sync.RWMutex
	activeGPUs map[string]GPUInfo

	healthMu     sync.RWMutex
	lastSuccess  time.Time
	lastGPUCount int
}

type clockEventReason struct {
	label string
	mask  uint64
}

var clockEventReasons = []clockEventReason{
	{label: "gpu_idle", mask: 0x0000000000000001},
	{label: "applications_clocks_setting", mask: 0x0000000000000002},
	{label: "software_power_cap", mask: 0x0000000000000004},
	{label: "hardware_slowdown", mask: 0x0000000000000008},
	{label: "sync_boost", mask: 0x0000000000000010},
	{label: "software_thermal", mask: 0x0000000000000020},
	{label: "hardware_thermal", mask: 0x0000000000000040},
	{label: "hardware_power_brake", mask: 0x0000000000000080},
	{label: "display_clocks", mask: 0x0000000000000100},
}

func NewExporter(cfg Config, client SampleClient, metrics *Metrics, hostname string, logger *slog.Logger) *Exporter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Exporter{
		cfg:              cfg,
		client:           client,
		metrics:          metrics,
		window:           NewWindowAggregator(),
		activity:         NewActivityTracker(cfg.ActiveThreshold, cfg.MinRequestTime),
		hostname:         hostname,
		logger:           logger,
		hardwareCounters: make(map[string]float64),
		observationTimes: make(map[string]time.Time),
		lastSeen:         make(map[string]time.Time),
		activeGPUs:       make(map[string]GPUInfo),
	}
}

func (e *Exporter) SetStaticMetrics() {
	driverVersion, cudaVersion := e.client.StaticInfo()
	e.metrics.GPUDriverVersion.WithLabelValues(driverVersion, e.hostname).Set(1)
	e.metrics.GPUCudaVersion.WithLabelValues(cudaVersion, e.hostname).Set(1)
	e.metrics.ExporterBuildInfo.WithLabelValues(version).Set(1)
}

// Run опрашивает fast-поля и независимо завершает fixed aggregation windows.
func (e *Exporter) Run(ctx context.Context) error {
	collectInterval := durationOrDefault(e.cfg.ScrapeInterval, defaultScrapeInterval)
	windowInterval := durationOrDefault(e.cfg.WindowInterval, defaultWindowInterval)
	collectTicker := time.NewTicker(collectInterval)
	windowTicker := time.NewTicker(windowInterval)
	defer collectTicker.Stop()
	defer windowTicker.Stop()

	if err := e.collect(); err != nil {
		e.logger.Warn("initial metric collection failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			e.PublishWindow()
			return ctx.Err()
		case <-collectTicker.C:
			if err := e.collect(); err != nil {
				e.logger.Warn("metric collection failed", "error", err)
			}
		case <-windowTicker.C:
			e.PublishWindow()
		}
	}
}

// PublishWindow публикует и сбрасывает внутреннее окно. HTTP /metrics эту
// функцию не вызывает, поэтому дополнительные scrapers не крадут данные.
func (e *Exporter) PublishWindow() {
	snapshot := e.window.Snapshot()

	e.infoMu.RLock()
	infos := make([]GPUInfo, 0, len(e.activeGPUs))
	for _, info := range e.activeGPUs {
		infos = append(infos, info)
	}
	e.infoMu.RUnlock()

	for _, info := range infos {
		labels := labelsFor(info, e.hostname)
		gpuKey := stableGPUIdentifier(info)
		for metricKey, metric := range e.metrics.WindowMetrics {
			stats, ok := snapshot[windowKey{gpuIndex: gpuKey, metric: metricKey}]
			if !ok || stats.Count == 0 {
				metric.Max.Delete(labels)
				metric.Avg.Delete(labels)
				continue
			}
			metric.Max.With(labels).Set(stats.Max)
			metric.Avg.With(labels).Set(stats.Avg())
		}
	}
}

func (e *Exporter) collect() error {
	started := time.Now()
	batch, err := e.client.Samples()
	e.metrics.ExporterCollectionDuration.With(exporterLabelsFor(e.hostname)).Set(time.Since(started).Seconds())
	if err != nil {
		e.markCollectFailure()
		return err
	}

	e.reconcileGPUs(batch.Supported)
	e.markCollection(time.Now(), batch)
	e.publishGPUCollectionStatus(batch)

	for _, sample := range batch.Samples {
		e.collectSample(sample)
	}
	return nil
}

func (e *Exporter) collectSample(sample GPUSample) {
	labels := labelsFor(sample.Info, e.hostname)
	e.publishFieldStatus(labels, sample.Fields)
	e.processObservations(sample)
	e.resetUnavailableFields(sample.Info, sample.Fields)

	setGauge(e.metrics.GPUMemoryFree, labels, sample.MemoryFreeBytes)
	setGauge(e.metrics.GPUMemoryUsed, labels, sample.MemoryUsedBytes)
	setGauge(e.metrics.GPUMemoryTotal, labels, sample.MemoryTotalBytes)
	setGauge(e.metrics.GPUMemoryReserved, labels, sample.MemoryReservedBytes)
	setGauge(e.metrics.GPUBAR1MemoryFree, labels, sample.BAR1FreeBytes)
	setGauge(e.metrics.GPUBAR1MemoryUsed, labels, sample.BAR1UsedBytes)
	setGauge(e.metrics.GPUBAR1MemoryTotal, labels, sample.BAR1TotalBytes)
	setGauge(e.metrics.GPUUtilizationCurrent, labels, sample.Utilization)
	setGauge(e.metrics.GPUTemperature, labels, sample.Temperature)
	setGauge(e.metrics.GPUMemoryTemperature, labels, sample.MemoryTemperature)
	setGauge(e.metrics.GPUPowerDraw, labels, sample.PowerDrawWatts)
	setGauge(e.metrics.GPUPowerDrawInstant, labels, sample.PowerDrawInstantWatts)
	setGauge(e.metrics.GPUMemoryCopyUtil, labels, sample.MemoryCopyUtil)
	setGauge(e.metrics.GPUEncoderUtil, labels, sample.EncoderUtil)
	setGauge(e.metrics.GPUDecoderUtil, labels, sample.DecoderUtil)
	setGauge(e.metrics.GPUMemoryUsedPct, labels, sample.MemoryUsedPercent)
	setGauge(e.metrics.GPUPowerLimit, labels, sample.PowerLimitWatts)
	setGauge(e.metrics.GPUPowerEnforcedLimit, labels, sample.PowerEnforcedLimitWatts)
	setGauge(e.metrics.GPUSMClockHertz, labels, sample.SMClockHertz)
	setGauge(e.metrics.GPUMemoryClockHertz, labels, sample.MemoryClockHertz)
	setGauge(e.metrics.GPUPerformanceState, labels, sample.PerformanceState)
	setGauge(e.metrics.GPUFanSpeed, labels, sample.FanSpeedPercent)
	setGauge(e.metrics.GPUPCIeLinkGeneration, labels, sample.PCIeLinkGeneration)
	setGauge(e.metrics.GPUPCIeLinkWidth, labels, sample.PCIeLinkWidth)
	setGauge(e.metrics.GPUPCIeMaxLinkGeneration, labels, sample.PCIeMaxLinkGeneration)
	setGauge(e.metrics.GPUPCIeMaxLinkWidth, labels, sample.PCIeMaxLinkWidth)
	setGauge(e.metrics.GPUXIDLastCode, labels, sample.XIDLastCode)
	setGauge(e.metrics.GPURetiredPagesPending, labels, sample.RetiredPendingPages)
	setGauge(e.metrics.GPURowRemapFailure, labels, sample.RowRemapFailure)
	setGauge(e.metrics.GPURowRemapPending, labels, sample.RowRemapPending)

	if sample.TemperatureMax != nil {
		e.metrics.GPUTemperatureMaxOp.With(labels).Set(*sample.TemperatureMax)
		e.metrics.GPUTemperatureMaxOld.With(labels).Set(*sample.TemperatureMax)
	}
	if sample.MemoryTemperatureMax != nil {
		e.metrics.GPUMemoryTemperatureMaxOp.With(labels).Set(*sample.MemoryTemperatureMax)
		e.metrics.GPUMemoryTemperatureMaxOld.With(labels).Set(*sample.MemoryTemperatureMax)
	}
	if sample.ThrottleReasons != nil {
		e.metrics.GPUThrottleReason.With(labels).Set(*sample.ThrottleReasons)
		e.setClockEvents(labels, uint64(*sample.ThrottleReasons))
	}

	pcieTX := firstPointer(sample.ProfPCIeTXBytesPerSecond, sample.PCIeTXBytesPerSecond)
	pcieRX := firstPointer(sample.ProfPCIeRXBytesPerSecond, sample.PCIeRXBytesPerSecond)
	setGauge(e.metrics.GPUPCIeTransmitRate, labels, pcieTX)
	setGauge(e.metrics.GPUPCIeReceiveRate, labels, pcieRX)
	setGauge(e.metrics.GPUNVLinkTransmitRate, labels, sample.ProfNVLinkTXBytesPerSecond)
	setGauge(e.metrics.GPUNVLinkReceiveRate, labels, sample.ProfNVLinkRXBytesPerSecond)

	setGauge(e.metrics.GPUProfGraphicsEngineActive, labels, sample.ProfGraphicsEngineActive)
	setGauge(e.metrics.GPUProfSMActive, labels, sample.ProfSMActive)
	setGauge(e.metrics.GPUProfSMOccupancy, labels, sample.ProfSMOccupancy)
	setGauge(e.metrics.GPUProfDRAMActive, labels, sample.ProfDRAMActive)
	setGauge(e.metrics.GPUProfTensorPipe, labels, sample.ProfTensorActive)
	setGauge(e.metrics.GPUProfPipeFP64Active, labels, sample.ProfPipeFP64Active)
	setGauge(e.metrics.GPUProfPipeFP32Active, labels, sample.ProfPipeFP32Active)
	setGauge(e.metrics.GPUProfPipeFP16Active, labels, sample.ProfPipeFP16Active)
	setGauge(e.metrics.GPUProfPipeINTActive, labels, sample.ProfPipeINTActive)
	setGauge(e.metrics.GPUProfTensorHMMAActive, labels, sample.ProfTensorHMMAActive)
	setGauge(e.metrics.GPUProfTensorIMMAActive, labels, sample.ProfTensorIMMAActive)
	setGauge(e.metrics.GPUProfTensorDFMAActive, labels, sample.ProfTensorDFMAActive)

	e.observeReliabilityCounters(sample, labels)
}

func (e *Exporter) publishFieldStatus(labels prometheus.Labels, fields map[dcgm.Short]DCGMFieldStatus) {
	for fieldID, status := range fields {
		seriesLabels := labelsWith(labels, "field", fieldLabel(fieldID))
		e.metrics.GPUFieldSupported.With(seriesLabels).Set(boolFloat(status.Supported))
		e.metrics.GPUFieldAvailable.With(seriesLabels).Set(boolFloat(status.Available))
		if !status.LastSuccess.IsZero() {
			e.metrics.GPUFieldLastSuccessTimestamp.With(seriesLabels).Set(float64(status.LastSuccess.Unix()))
		}
	}
}

func (e *Exporter) resetUnavailableFields(info GPUInfo, fields map[dcgm.Short]DCGMFieldStatus) {
	for fieldID, status := range fields {
		if status.Supported && !status.Available {
			e.resetObservationTime(info, fieldID)
		}
	}
}

func (e *Exporter) resetObservationTime(info GPUInfo, fieldID dcgm.Short) {
	key := fmt.Sprintf("%s:%d", stableGPUIdentifier(info), fieldID)
	e.observationMu.Lock()
	delete(e.observationTimes, key)
	e.observationMu.Unlock()
}

func (e *Exporter) processObservations(sample GPUSample) {
	labels := labelsFor(sample.Info, e.hostname)
	for _, observation := range sample.Observations {
		metricKey, ok := aggregationForField(observation.FieldID)
		if !ok {
			continue
		}
		if isLegacyPCIeField(observation.FieldID) && dcpPCIeAvailable(sample.Fields, observation.FieldID) {
			continue
		}

		gpuKey := stableGPUIdentifier(sample.Info)
		e.window.Observe(gpuKey, metricKey, observation.Value)
		dt := e.observationInterval(sample.Info, observation.FieldID, observation.Timestamp)

		if observation.FieldID == dcgm.DCGM_FI_DEV_GPU_UTIL {
			if e.activity.Observe(gpuKey, observation.Value, observation.Timestamp) {
				e.metrics.GPURequestCount.With(labels).Inc()
				e.metrics.GPUActivityWindows.With(labels).Inc()
			}
			if dt > 0 && observation.Value > e.cfg.ActiveThreshold {
				e.metrics.GPUActiveSeconds.With(labels).Add(dt)
			}
		}

		if integral, exists := e.metrics.Integrals[metricKey]; exists && dt > 0 {
			fraction := ratioFraction(observation.Value)
			if integral.Percent {
				fraction = percentFraction(observation.Value)
			}
			integral.Weighted.With(labels).Add(fraction * dt)
			integral.Observed.With(labels).Add(dt)
		}
		if total, exists := e.metrics.RateTotals[metricKey]; exists && dt > 0 {
			total.With(labels).Add(observation.Value * dt)
		}
		if observation.FieldID == dcgm.DCGM_FI_DEV_POWER_USAGE && dt > 0 {
			e.metrics.GPUEnergyEstimated.With(labels).Add(observation.Value * dt)
		}
	}
}

func (e *Exporter) observeReliabilityCounters(sample GPUSample, labels prometheus.Labels) {
	if sample.TotalEnergyJoules != nil {
		e.observeHardwareCounter(hardwareCounterKey(sample.Info, "energy"), *sample.TotalEnergyJoules, e.metrics.GPUEnergyJoules, labels)
	}
	if sample.PCIeReplayCounter != nil {
		e.observeHardwareCounter(hardwareCounterKey(sample.Info, "pcie_replay"), *sample.PCIeReplayCounter, e.metrics.GPUPCIeReplayTotal, labels)
	}
	if sample.ECCSBEVolatileTotal != nil {
		e.observeHardwareCounter(hardwareCounterKey(sample.Info, "ecc", "volatile", "sbe"), *sample.ECCSBEVolatileTotal, e.metrics.GPUECCErrors, labelsWith(labels, "correctability", "single_bit", "persistence", "volatile", "location", "total"))
	}
	if sample.ECCDBEVolatileTotal != nil {
		e.observeHardwareCounter(hardwareCounterKey(sample.Info, "ecc", "volatile", "dbe"), *sample.ECCDBEVolatileTotal, e.metrics.GPUECCErrors, labelsWith(labels, "correctability", "double_bit", "persistence", "volatile", "location", "total"))
	}
	if sample.ECCSBEAggregateTotal != nil {
		e.observeHardwareCounter(hardwareCounterKey(sample.Info, "ecc", "aggregate", "sbe"), *sample.ECCSBEAggregateTotal, e.metrics.GPUECCErrors, labelsWith(labels, "correctability", "single_bit", "persistence", "aggregate", "location", "total"))
	}
	if sample.ECCDBEAggregateTotal != nil {
		e.observeHardwareCounter(hardwareCounterKey(sample.Info, "ecc", "aggregate", "dbe"), *sample.ECCDBEAggregateTotal, e.metrics.GPUECCErrors, labelsWith(labels, "correctability", "double_bit", "persistence", "aggregate", "location", "total"))
	}
	if sample.RetiredSBEPages != nil {
		e.observeHardwareCounter(hardwareCounterKey(sample.Info, "retired_pages", "sbe"), *sample.RetiredSBEPages, e.metrics.GPURetiredPages, labelsWith(labels, "cause", "sbe"))
	}
	if sample.RetiredDBEPages != nil {
		e.observeHardwareCounter(hardwareCounterKey(sample.Info, "retired_pages", "dbe"), *sample.RetiredDBEPages, e.metrics.GPURetiredPages, labelsWith(labels, "cause", "dbe"))
	}
	if sample.CorrectableRemappedRows != nil {
		e.observeHardwareCounter(hardwareCounterKey(sample.Info, "remapped_rows", "correctable"), *sample.CorrectableRemappedRows, e.metrics.GPURemappedRows, labelsWith(labels, "correctability", "correctable"))
	}
	if sample.UncorrectableRemappedRows != nil {
		e.observeHardwareCounter(hardwareCounterKey(sample.Info, "remapped_rows", "uncorrectable"), *sample.UncorrectableRemappedRows, e.metrics.GPURemappedRows, labelsWith(labels, "correctability", "uncorrectable"))
	}

	violations := []struct {
		reason string
		value  *float64
	}{
		{"power", sample.PowerViolationSeconds},
		{"thermal", sample.ThermalViolationSeconds},
		{"sync_boost", sample.SyncBoostViolationSeconds},
		{"board_limit", sample.BoardLimitViolationSeconds},
		{"low_utilization", sample.LowUtilViolationSeconds},
		{"reliability", sample.ReliabilityViolationSeconds},
		{"application_clocks", sample.AppClockViolationSeconds},
		{"base_clocks", sample.BaseClockViolationSeconds},
	}
	for _, violation := range violations {
		if violation.value == nil {
			continue
		}
		e.observeHardwareCounter(hardwareCounterKey(sample.Info, "clock_violation", violation.reason), *violation.value, e.metrics.GPUClockViolationSeconds, labelsWith(labels, "reason", violation.reason))
	}
}

func (e *Exporter) markCollection(now time.Time, batch SampleBatch) {
	failed := make(map[string]struct{})
	for _, failure := range batch.Failures {
		failed[stableGPUIdentifier(failure.Info)] = struct{}{}
	}
	complete := len(batch.Supported) > 0 && len(batch.Samples) == len(batch.Supported) && len(failed) == 0

	e.healthMu.Lock()
	if complete {
		e.lastSuccess = now
		e.lastGPUCount = len(batch.Supported)
	} else {
		e.lastGPUCount = 0
	}
	e.healthMu.Unlock()

	labels := exporterLabelsFor(e.hostname)
	e.metrics.ExporterUp.With(labels).Set(1)
	e.metrics.ExporterCollectSuccess.With(labels).Set(boolFloat(complete))
	e.metrics.ExporterDiscoveredGPUs.With(labels).Set(float64(len(batch.Supported)))
	e.metrics.ExporterCollectedGPUs.With(labels).Set(float64(len(batch.Samples)))
	e.metrics.ExporterFailedGPUs.With(labels).Set(float64(len(failed)))
	if complete {
		e.metrics.ExporterLastSuccessTimestamp.With(labels).Set(float64(now.Unix()))
	} else {
		e.metrics.ExporterCollectionErrors.With(labels).Inc()
	}
}

func (e *Exporter) markCollectFailure() {
	e.healthMu.Lock()
	e.lastGPUCount = 0
	e.healthMu.Unlock()

	labels := exporterLabelsFor(e.hostname)
	e.metrics.ExporterUp.With(labels).Set(0)
	e.metrics.ExporterCollectSuccess.With(labels).Set(0)
	e.metrics.ExporterDiscoveredGPUs.With(labels).Set(0)
	e.metrics.ExporterCollectedGPUs.With(labels).Set(0)
	e.metrics.ExporterFailedGPUs.With(labels).Set(0)
	e.metrics.ExporterCollectionErrors.With(labels).Inc()
}

func (e *Exporter) publishGPUCollectionStatus(batch SampleBatch) {
	failures := make(map[string][]GPUCollectionFailure)
	for _, failure := range batch.Failures {
		key := stableGPUIdentifier(failure.Info)
		failures[key] = append(failures[key], failure)
		labels := labelsWith(labelsFor(failure.Info, e.hostname), "reason", failure.Reason)
		e.metrics.GPUCollectionErrors.With(labels).Inc()
	}
	for _, info := range batch.Supported {
		labels := labelsFor(info, e.hostname)
		_, failed := failures[stableGPUIdentifier(info)]
		e.metrics.GPUCollectSuccess.With(labels).Set(boolFloat(!failed))
	}
}

func (e *Exporter) reconcileGPUs(current []GPUInfo) {
	next := make(map[string]GPUInfo, len(current))
	for _, info := range current {
		next[stableGPUIdentifier(info)] = info
	}

	e.infoMu.Lock()
	previous := e.activeGPUs
	e.activeGPUs = next
	e.infoMu.Unlock()

	for key, oldInfo := range previous {
		newInfo, exists := next[key]
		if exists && gpuInfoLabelsEqual(oldInfo, newInfo) {
			continue
		}
		e.metrics.DeleteGPU(labelsFor(oldInfo, e.hostname))
		e.deleteGPUState(key)
	}
}

func (e *Exporter) deleteGPUState(gpuKey string) {
	prefix := gpuKey + ":"
	e.hardwareCounterMu.Lock()
	for key := range e.hardwareCounters {
		if stringsHasPrefix(key, prefix) {
			delete(e.hardwareCounters, key)
		}
	}
	e.hardwareCounterMu.Unlock()

	e.observationMu.Lock()
	for key := range e.observationTimes {
		if stringsHasPrefix(key, prefix) {
			delete(e.observationTimes, key)
		}
	}
	e.observationMu.Unlock()
}

func (e *Exporter) observeHardwareCounter(key string, value float64, counter *prometheus.CounterVec, labels prometheus.Labels) {
	if value < 0 {
		return
	}
	e.hardwareCounterMu.Lock()
	previous, ok := e.hardwareCounters[key]
	delta := value
	if ok {
		if value >= previous {
			delta = value - previous
		} else {
			delta = value
		}
	}
	e.hardwareCounters[key] = value
	e.hardwareCounterMu.Unlock()

	// Add(0) намеренно создаёт серию для поддерживаемого нулевого counter.
	counter.With(labels).Add(delta)
}

func (e *Exporter) observationInterval(info GPUInfo, fieldID dcgm.Short, now time.Time) float64 {
	if now.IsZero() {
		return 0
	}
	key := fmt.Sprintf("%s:%d", stableGPUIdentifier(info), fieldID)
	e.observationMu.Lock()
	previous, ok := e.observationTimes[key]
	e.observationTimes[key] = now
	e.observationMu.Unlock()
	if !ok || !now.After(previous) {
		return 0
	}
	delta := now.Sub(previous)
	if delta > e.maxObservationGap(fieldID) {
		return 0
	}
	return delta.Seconds()
}

func (e *Exporter) maxObservationGap(fieldID dcgm.Short) time.Duration {
	interval := durationOrDefault(e.cfg.ScrapeInterval, defaultScrapeInterval)
	switch {
	case containsField(profFields(), fieldID), containsField(legacyRateFields(), fieldID):
		interval = durationOrDefault(e.cfg.ProfilingInterval, defaultProfilingInterval)
	case containsField(operationalFields(), fieldID):
		interval = durationOrDefault(e.cfg.OperationalInterval, defaultOperationalInterval)
	case containsField(reliabilityFields(), fieldID):
		interval = durationOrDefault(e.cfg.ReliabilityInterval, defaultReliabilityInterval)
	}
	return 10 * interval
}

func (e *Exporter) sampleInterval(gpuIndex string, now time.Time) float64 {
	if e.lastSeen == nil {
		e.lastSeen = make(map[string]time.Time)
	}
	previous, ok := e.lastSeen[gpuIndex]
	e.lastSeen[gpuIndex] = now
	if !ok || !now.After(previous) || now.Sub(previous) > e.maxSampleGap() {
		return 0
	}
	return now.Sub(previous).Seconds()
}

func (e *Exporter) maxSampleGap() time.Duration {
	return 10 * durationOrDefault(e.cfg.ScrapeInterval, defaultScrapeInterval)
}

func (e *Exporter) setClockEvents(labels prometheus.Labels, mask uint64) {
	for _, reason := range clockEventReasons {
		active := 0.0
		if mask&reason.mask != 0 {
			active = 1
		}
		e.metrics.GPUClockEventActive.With(labelsWith(labels, "reason", reason.label)).Set(active)
	}
}

func (e *Exporter) Ready(now time.Time) bool {
	e.healthMu.RLock()
	lastSuccess := e.lastSuccess
	gpuCount := e.lastGPUCount
	e.healthMu.RUnlock()
	if gpuCount <= 0 || lastSuccess.IsZero() {
		return false
	}
	maxAge := 5 * durationOrDefault(e.cfg.ScrapeInterval, defaultScrapeInterval)
	if maxAge < 5*time.Second {
		maxAge = 5 * time.Second
	}
	return now.Sub(lastSuccess) <= maxAge
}

func aggregationForField(fieldID dcgm.Short) (string, bool) {
	switch fieldID {
	case dcgm.DCGM_FI_DEV_GPU_UTIL:
		return aggUtilization, true
	case dcgm.DCGM_FI_DEV_MEM_COPY_UTIL:
		return aggMemoryCopyUtil, true
	case dcgm.DCGM_FI_DEV_FB_USED_PERCENT:
		return aggMemoryUsedPct, true
	case dcgm.DCGM_FI_DEV_POWER_USAGE:
		return aggPowerDraw, true
	case dcgm.DCGM_FI_DEV_GPU_TEMP:
		return aggTemperature, true
	case dcgm.DCGM_FI_PROF_GR_ENGINE_ACTIVE:
		return aggProfGraphics, true
	case dcgm.DCGM_FI_PROF_SM_ACTIVE:
		return aggProfSM, true
	case dcgm.DCGM_FI_PROF_SM_OCCUPANCY:
		return aggProfSMOccupancy, true
	case dcgm.DCGM_FI_PROF_DRAM_ACTIVE:
		return aggProfDRAM, true
	case dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE:
		return aggProfTensor, true
	case dcgm.DCGM_FI_PROF_PIPE_FP64_ACTIVE:
		return aggProfFP64, true
	case dcgm.DCGM_FI_PROF_PIPE_FP32_ACTIVE:
		return aggProfFP32, true
	case dcgm.DCGM_FI_PROF_PIPE_FP16_ACTIVE:
		return aggProfFP16, true
	case dcgm.DCGM_FI_PROF_PIPE_INT_ACTIVE:
		return aggProfINT, true
	case dcgm.DCGM_FI_PROF_PIPE_TENSOR_HMMA_ACTIVE:
		return aggProfTensorHMMA, true
	case dcgm.DCGM_FI_PROF_PIPE_TENSOR_IMMA_ACTIVE:
		return aggProfTensorIMMA, true
	case dcgm.DCGM_FI_PROF_PIPE_TENSOR_DFMA_ACTIVE:
		return aggProfTensorDFMA, true
	case dcgm.DCGM_FI_DEV_PCIE_TX_THROUGHPUT, dcgm.DCGM_FI_PROF_PCIE_TX_BYTES:
		return aggPCIeTransmit, true
	case dcgm.DCGM_FI_DEV_PCIE_RX_THROUGHPUT, dcgm.DCGM_FI_PROF_PCIE_RX_BYTES:
		return aggPCIeReceive, true
	case dcgm.DCGM_FI_PROF_NVLINK_TX_BYTES:
		return aggNVLinkTransmit, true
	case dcgm.DCGM_FI_PROF_NVLINK_RX_BYTES:
		return aggNVLinkReceive, true
	default:
		return "", false
	}
}

func dcpPCIeAvailable(fields map[dcgm.Short]DCGMFieldStatus, legacyField dcgm.Short) bool {
	dcpField := dcgm.DCGM_FI_PROF_PCIE_TX_BYTES
	if legacyField == dcgm.DCGM_FI_DEV_PCIE_RX_THROUGHPUT {
		dcpField = dcgm.DCGM_FI_PROF_PCIE_RX_BYTES
	}
	return fields[dcpField].Available
}

func isLegacyPCIeField(fieldID dcgm.Short) bool {
	return fieldID == dcgm.DCGM_FI_DEV_PCIE_TX_THROUGHPUT || fieldID == dcgm.DCGM_FI_DEV_PCIE_RX_THROUGHPUT
}

func containsField(fields []dcgm.Short, wanted dcgm.Short) bool {
	for _, fieldID := range fields {
		if fieldID == wanted {
			return true
		}
	}
	return false
}

func setGauge(metric *prometheus.GaugeVec, labels prometheus.Labels, value *float64) {
	if value != nil {
		metric.With(labels).Set(*value)
	}
}

func firstPointer(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func labelsWith(base prometheus.Labels, keyValues ...string) prometheus.Labels {
	labels := make(prometheus.Labels, len(base)+len(keyValues)/2)
	for key, value := range base {
		labels[key] = value
	}
	for i := 0; i+1 < len(keyValues); i += 2 {
		labels[keyValues[i]] = keyValues[i+1]
	}
	return labels
}

func hardwareCounterKey(info GPUInfo, name string, parts ...string) string {
	key := stableGPUIdentifier(info) + ":" + name
	for _, part := range parts {
		key += ":" + part
	}
	return key
}

func stableGPUIdentifier(info GPUInfo) string {
	if value := validDCGMString(info.UUID); value != "" && value != "unknown" {
		return value
	}
	if value := validDCGMString(info.PCIBusID); value != "" && value != "unknown" {
		return value
	}
	return firstNonEmpty(info.Index, fmt.Sprintf("gpu-%d", info.ID), "unknown")
}

func gpuInfoLabelsEqual(left, right GPUInfo) bool {
	return left.Index == right.Index && left.UUID == right.UUID && left.PCIBusID == right.PCIBusID && left.Name == right.Name
}

func percentFraction(value float64) float64 {
	if value <= 0 {
		return 0
	}
	if value >= 100 {
		return 1
	}
	return value / 100
}

func ratioFraction(value float64) float64 {
	if value < 0 || value > 1 {
		return 0
	}
	return value
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func stringsHasPrefix(value, prefix string) bool {
	return len(value) >= len(prefix) && value[:len(prefix)] == prefix
}
