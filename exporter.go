package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Exporter struct {
	cfg      Config
	client   *DCGMClient
	metrics  *Metrics
	window   *WindowAggregator
	activity *ActivityTracker
	hostname string
	logger   *slog.Logger
	lastSeen map[string]time.Time

	hardwareCounterMu sync.Mutex
	hardwareCounters   map[string]float64

	// infoMu защищает lastInfos - список GPU, закешированный последним
	// collect(). FlushWindow использует кэш вместо повторного
	// запроса к DCGM на каждый скрейп.
	infoMu    sync.RWMutex
	lastInfos []GPUInfo

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

func NewExporter(cfg Config, client *DCGMClient, metrics *Metrics, hostname string, logger *slog.Logger) *Exporter {
	return &Exporter{
		cfg:      cfg,
		client:   client,
		metrics:  metrics,
		window:   NewWindowAggregator(),
		activity: NewActivityTracker(cfg.ActiveThreshold, cfg.MinRequestTime),
		hostname: hostname,
		logger:   logger,
		lastSeen: make(map[string]time.Time),

		hardwareCounters: make(map[string]float64),
	}
}

// SetStaticMetrics публикует версии драйвера и CUDA один раз при старте.
func (e *Exporter) SetStaticMetrics() {
	driverVersion, cudaVersion := e.client.StaticInfo()

	e.metrics.GPUDriverVersion.WithLabelValues(driverVersion, e.hostname).Set(1)
	e.metrics.GPUCudaVersion.WithLabelValues(cudaVersion, e.hostname).Set(1)
}

// Run запускает цикл высокочастотного опроса DCGM до отмены контекста.
func (e *Exporter) Run(ctx context.Context) error {
	ticker := time.NewTicker(e.cfg.ScrapeInterval)
	defer ticker.Stop()

	if err := e.collect(); err != nil {
		e.logger.Warn("initial metric collection failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := e.collect(); err != nil {
				e.logger.Warn("metric collection failed", "error", err)
			}
		}
	}
}

// FlushWindow переводит статистику, накопленную с прошлого скрейпа, в
// *_max и *_avg gauge'ы. Вызывается из хендлера /metrics непосредственно
// перед сбором registry, поэтому каждый скрейп видит ровно одно окно.
//
// flush сбрасывает окно, exporter рассчитан на одного скрейпера.
func (e *Exporter) FlushWindow() {
	e.infoMu.RLock()
	infos := e.lastInfos
	e.infoMu.RUnlock()

	snapshot := e.window.Snapshot()

	for _, info := range infos {
		labels := labelsFor(info, e.hostname)
		idx := info.Index
		e.setWindowGauges(snapshot, idx, aggUtilization, labels, e.metrics.GPUUtilization, e.metrics.GPUUtilizationAvg)
		e.setWindowGauges(snapshot, idx, aggMemoryCopyUtil, labels, e.metrics.GPUMemoryCopyMax, e.metrics.GPUMemoryCopyAvg)
		e.setWindowGauges(snapshot, idx, aggMemoryUsedPct, labels, e.metrics.GPUMemoryUsedMax, e.metrics.GPUMemoryUsedAvg)
		e.setWindowGauges(snapshot, idx, aggPowerDraw, labels, e.metrics.GPUPowerDrawMax, e.metrics.GPUPowerDrawAvg)
		e.setWindowGauges(snapshot, idx, aggTemperature, labels, e.metrics.GPUTemperatureWin, e.metrics.GPUTemperatureAvg)
		e.setWindowGauges(snapshot, idx, aggProfSM, labels, e.metrics.GPUProfSMMax, e.metrics.GPUProfSMAvg)
		e.setWindowGauges(snapshot, idx, aggProfDRAM, labels, e.metrics.GPUProfDRAMMax, e.metrics.GPUProfDRAMAvg)
		e.setWindowGauges(snapshot, idx, aggProfTensor, labels, e.metrics.GPUProfTensorMax, e.metrics.GPUProfTensorAvg)
	}
}

// setWindowGauges публикует max/avg одной метрики. Если в окне не было
// сэмплов (ошибка DCGM, неподдерживаемое prof-поле), предыдущее значение
// сохраняется.
func (e *Exporter) setWindowGauges(snapshot map[windowKey]WindowStats, gpuIndex, metric string, labels prometheus.Labels, maxGauge, avgGauge *prometheus.GaugeVec) {
	stats, ok := snapshot[windowKey{gpuIndex: gpuIndex, metric: metric}]
	if !ok || stats.Count == 0 {
		return
	}
	maxGauge.With(labels).Set(stats.Max)
	avgGauge.With(labels).Set(stats.Avg())
}

// collect снимает один сэмпл со всех GPU: обновляет мгновенные gauge'ы,
// добавляет выбранные поля в оконную статистику и детектирует запросы.
func (e *Exporter) collect() error {
	started := time.Now()
	samples, err := e.client.Samples()
	e.metrics.ExporterCollectionDuration.With(exporterLabelsFor(e.hostname)).Set(time.Since(started).Seconds())
	if err != nil {
		e.markCollectFailure()
		return err
	}

	now := time.Now()
	e.markCollectSuccess(now, len(samples))
	infos := make([]GPUInfo, 0, len(samples))
	for _, sample := range samples {
		infos = append(infos, sample.Info)
		labels := labelsFor(sample.Info, e.hostname)
		gpuIndex := sample.Info.Index
		dtSeconds := e.sampleInterval(gpuIndex, now)

		if sample.MemoryFreeBytes != nil {
			e.metrics.GPUMemoryFree.With(labels).Set(*sample.MemoryFreeBytes)
		}
		if sample.MemoryUsedBytes != nil {
			e.metrics.GPUMemoryUsed.With(labels).Set(*sample.MemoryUsedBytes)
		}
		if sample.MemoryTotalBytes != nil {
			e.metrics.GPUMemoryTotal.With(labels).Set(*sample.MemoryTotalBytes)
		}
		if sample.MemoryReservedBytes != nil {
			e.metrics.GPUMemoryReserved.With(labels).Set(*sample.MemoryReservedBytes)
		}
		if sample.BAR1FreeBytes != nil {
			e.metrics.GPUBAR1MemoryFree.With(labels).Set(*sample.BAR1FreeBytes)
		}
		if sample.BAR1UsedBytes != nil {
			e.metrics.GPUBAR1MemoryUsed.With(labels).Set(*sample.BAR1UsedBytes)
		}
		if sample.BAR1TotalBytes != nil {
			e.metrics.GPUBAR1MemoryTotal.With(labels).Set(*sample.BAR1TotalBytes)
		}

		if sample.Utilization != nil {
			e.metrics.GPUUtilizationCurrent.With(labels).Set(*sample.Utilization)
			e.window.Observe(gpuIndex, aggUtilization, *sample.Utilization)
			if dtSeconds > 0 {
				e.metrics.GPUUtilizationWeightedSeconds.With(labels).Add(percentFraction(*sample.Utilization) * dtSeconds)
				if *sample.Utilization > e.cfg.ActiveThreshold {
					e.metrics.GPUActiveSeconds.With(labels).Add(dtSeconds)
				}
			}

			if e.activity.Observe(gpuIndex, *sample.Utilization, now) {
				e.metrics.GPURequestCount.With(labels).Inc()
				e.metrics.GPUActivityWindows.With(labels).Inc()
			}
		}
		if sample.Temperature != nil {
			e.metrics.GPUTemperature.With(labels).Set(*sample.Temperature)
			e.window.Observe(gpuIndex, aggTemperature, *sample.Temperature)
		}
		if sample.MemoryTemperature != nil {
			e.metrics.GPUMemoryTemperature.With(labels).Set(*sample.MemoryTemperature)
		}
		if sample.MemoryTemperatureMax != nil {
			e.metrics.GPUMemoryTemperatureMax.With(labels).Set(*sample.MemoryTemperatureMax)
		}
		if sample.PowerDrawWatts != nil {
			e.metrics.GPUPowerDraw.With(labels).Set(*sample.PowerDrawWatts)
			e.window.Observe(gpuIndex, aggPowerDraw, *sample.PowerDrawWatts)
			if dtSeconds > 0 {
				e.metrics.GPUEnergyEstimatedJoules.With(labels).Add(*sample.PowerDrawWatts * dtSeconds)
			}
		}
		if sample.PowerDrawInstantWatts != nil {
			e.metrics.GPUPowerDrawInstant.With(labels).Set(*sample.PowerDrawInstantWatts)
		}
		if sample.TotalEnergyJoules != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "energy"), *sample.TotalEnergyJoules, e.metrics.GPUEnergyJoules, labels)
		}
		if sample.MemoryCopyUtil != nil {
			e.metrics.GPUMemoryCopyUtil.With(labels).Set(*sample.MemoryCopyUtil)
			e.window.Observe(gpuIndex, aggMemoryCopyUtil, *sample.MemoryCopyUtil)
		}
		if sample.EncoderUtil != nil {
			e.metrics.GPUEncoderUtil.With(labels).Set(*sample.EncoderUtil)
		}
		if sample.DecoderUtil != nil {
			e.metrics.GPUDecoderUtil.With(labels).Set(*sample.DecoderUtil)
		}
		if sample.MemoryUsedPercent != nil {
			e.metrics.GPUMemoryUsedPct.With(labels).Set(*sample.MemoryUsedPercent)
			e.window.Observe(gpuIndex, aggMemoryUsedPct, *sample.MemoryUsedPercent)
		}
		if sample.TemperatureMax != nil {
			e.metrics.GPUTemperatureMax.With(labels).Set(*sample.TemperatureMax)
		}
		if sample.PowerLimitWatts != nil {
			e.metrics.GPUPowerLimit.With(labels).Set(*sample.PowerLimitWatts)
		}
		if sample.PowerEnforcedLimitWatts != nil {
			e.metrics.GPUPowerEnforcedLimit.With(labels).Set(*sample.PowerEnforcedLimitWatts)
		}
		if sample.ThrottleReasons != nil {
			e.metrics.GPUThrottleReason.With(labels).Set(*sample.ThrottleReasons)
			e.setClockEvents(labels, uint64(*sample.ThrottleReasons))
		}
		if sample.SMClockHertz != nil {
			e.metrics.GPUSMClockHertz.With(labels).Set(*sample.SMClockHertz)
		}
		if sample.MemoryClockHertz != nil {
			e.metrics.GPUMemoryClockHertz.With(labels).Set(*sample.MemoryClockHertz)
		}
		if sample.PerformanceState != nil {
			e.metrics.GPUPerformanceState.With(labels).Set(*sample.PerformanceState)
		}
		if sample.FanSpeedPercent != nil {
			e.metrics.GPUFanSpeed.With(labels).Set(*sample.FanSpeedPercent)
		}
		if sample.PCIeTXBytesPerSecond != nil {
			e.metrics.GPUPCIeTXBytesPerSecond.With(labels).Set(*sample.PCIeTXBytesPerSecond)
		}
		if sample.PCIeRXBytesPerSecond != nil {
			e.metrics.GPUPCIeRXBytesPerSecond.With(labels).Set(*sample.PCIeRXBytesPerSecond)
		}
		if sample.ProfPCIeTXBytesPerSecond != nil {
			e.metrics.GPUPCIeTransmitBytesPerSecond.With(labels).Set(*sample.ProfPCIeTXBytesPerSecond)
		}
		if sample.ProfPCIeRXBytesPerSecond != nil {
			e.metrics.GPUPCIeReceiveBytesPerSecond.With(labels).Set(*sample.ProfPCIeRXBytesPerSecond)
		}
		if sample.PCIeReplayCounter != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "pcie_replay"), *sample.PCIeReplayCounter, e.metrics.GPUPCIeReplayTotal, labels)
		}
		if sample.PCIeLinkGeneration != nil {
			e.metrics.GPUPCIeLinkGeneration.With(labels).Set(*sample.PCIeLinkGeneration)
		}
		if sample.PCIeLinkWidth != nil {
			e.metrics.GPUPCIeLinkWidth.With(labels).Set(*sample.PCIeLinkWidth)
		}
		if sample.PCIeMaxLinkGeneration != nil {
			e.metrics.GPUPCIeMaxLinkGeneration.With(labels).Set(*sample.PCIeMaxLinkGeneration)
		}
		if sample.PCIeMaxLinkWidth != nil {
			e.metrics.GPUPCIeMaxLinkWidth.With(labels).Set(*sample.PCIeMaxLinkWidth)
		}
		if sample.NVLinkTXBytesPerSecond != nil {
			e.metrics.GPUNVLinkTXBytesPerSecond.With(labels).Set(*sample.NVLinkTXBytesPerSecond)
		}
		if sample.NVLinkRXBytesPerSecond != nil {
			e.metrics.GPUNVLinkRXBytesPerSecond.With(labels).Set(*sample.NVLinkRXBytesPerSecond)
		}
		if sample.ProfNVLinkTXBytesPerSecond != nil {
			e.metrics.GPUNVLinkTransmitBytesPerSecond.With(labels).Set(*sample.ProfNVLinkTXBytesPerSecond)
		}
		if sample.ProfNVLinkRXBytesPerSecond != nil {
			e.metrics.GPUNVLinkReceiveBytesPerSecond.With(labels).Set(*sample.ProfNVLinkRXBytesPerSecond)
		}
		if sample.XIDLastCode != nil {
			e.metrics.GPUXIDLastCode.With(labels).Set(*sample.XIDLastCode)
		}
		if sample.NVLinkReceiveCodeErrorsTotal != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "nvlink_error", "receive_code"), *sample.NVLinkReceiveCodeErrorsTotal, e.metrics.GPUNVLinkErrors, labelsWith(labels, "type", "receive_code"))
		}
		if sample.NVLinkReceiveUncorrectableCodesTotal != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "nvlink_error", "receive_uncorrectable_code"), *sample.NVLinkReceiveUncorrectableCodesTotal, e.metrics.GPUNVLinkErrors, labelsWith(labels, "type", "receive_uncorrectable_code"))
		}
		if sample.NVLinkTransmitRetryCodesTotal != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "nvlink_error", "transmit_retry_code"), *sample.NVLinkTransmitRetryCodesTotal, e.metrics.GPUNVLinkErrors, labelsWith(labels, "type", "transmit_retry_code"))
		}
		if sample.NVLinkTransmitRetryEventsTotal != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "nvlink_error", "transmit_retry_event"), *sample.NVLinkTransmitRetryEventsTotal, e.metrics.GPUNVLinkErrors, labelsWith(labels, "type", "transmit_retry_event"))
		}
		if sample.NVLinkLinkDownTotal != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "nvlink_error", "physical_link_down"), *sample.NVLinkLinkDownTotal, e.metrics.GPUNVLinkErrors, labelsWith(labels, "type", "physical_link_down"))
		}
		if sample.ECCSBEVolatileTotal != nil {
			seriesLabels := labelsWith(labels, "correctability", "single_bit", "persistence", "volatile", "location", "total")
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "ecc", "volatile", "sbe"), *sample.ECCSBEVolatileTotal, e.metrics.GPUECCErrors, seriesLabels)
		}
		if sample.ECCDBEVolatileTotal != nil {
			seriesLabels := labelsWith(labels, "correctability", "double_bit", "persistence", "volatile", "location", "total")
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "ecc", "volatile", "dbe"), *sample.ECCDBEVolatileTotal, e.metrics.GPUECCErrors, seriesLabels)
		}
		if sample.ECCSBEAggregateTotal != nil {
			seriesLabels := labelsWith(labels, "correctability", "single_bit", "persistence", "aggregate", "location", "total")
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "ecc", "aggregate", "sbe"), *sample.ECCSBEAggregateTotal, e.metrics.GPUECCErrors, seriesLabels)
		}
		if sample.ECCDBEAggregateTotal != nil {
			seriesLabels := labelsWith(labels, "correctability", "double_bit", "persistence", "aggregate", "location", "total")
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "ecc", "aggregate", "dbe"), *sample.ECCDBEAggregateTotal, e.metrics.GPUECCErrors, seriesLabels)
		}
		if sample.RetiredSBEPages != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "retired_pages", "sbe"), *sample.RetiredSBEPages, e.metrics.GPURetiredPages, labelsWith(labels, "cause", "sbe"))
		}
		if sample.RetiredDBEPages != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "retired_pages", "dbe"), *sample.RetiredDBEPages, e.metrics.GPURetiredPages, labelsWith(labels, "cause", "dbe"))
		}
		if sample.RetiredPendingPages != nil {
			e.metrics.GPURetiredPagesPending.With(labels).Set(*sample.RetiredPendingPages)
		}
		if sample.CorrectableRemappedRows != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "remapped_rows", "correctable"), *sample.CorrectableRemappedRows, e.metrics.GPURemappedRows, labelsWith(labels, "correctability", "correctable"))
		}
		if sample.UncorrectableRemappedRows != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "remapped_rows", "uncorrectable"), *sample.UncorrectableRemappedRows, e.metrics.GPURemappedRows, labelsWith(labels, "correctability", "uncorrectable"))
		}
		if sample.RowRemapFailure != nil {
			e.metrics.GPURowRemapFailure.With(labels).Set(*sample.RowRemapFailure)
		}
		if sample.RowRemapPending != nil {
			e.metrics.GPURowRemapPending.With(labels).Set(*sample.RowRemapPending)
		}
		if sample.PowerViolationSeconds != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "clock_violation", "power"), *sample.PowerViolationSeconds, e.metrics.GPUClockViolationSeconds, labelsWith(labels, "reason", "power"))
		}
		if sample.ThermalViolationSeconds != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "clock_violation", "thermal"), *sample.ThermalViolationSeconds, e.metrics.GPUClockViolationSeconds, labelsWith(labels, "reason", "thermal"))
		}
		if sample.SyncBoostViolationSeconds != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "clock_violation", "sync_boost"), *sample.SyncBoostViolationSeconds, e.metrics.GPUClockViolationSeconds, labelsWith(labels, "reason", "sync_boost"))
		}
		if sample.BoardLimitViolationSeconds != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "clock_violation", "board_limit"), *sample.BoardLimitViolationSeconds, e.metrics.GPUClockViolationSeconds, labelsWith(labels, "reason", "board_limit"))
		}
		if sample.LowUtilViolationSeconds != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "clock_violation", "low_utilization"), *sample.LowUtilViolationSeconds, e.metrics.GPUClockViolationSeconds, labelsWith(labels, "reason", "low_utilization"))
		}
		if sample.ReliabilityViolationSeconds != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "clock_violation", "reliability"), *sample.ReliabilityViolationSeconds, e.metrics.GPUClockViolationSeconds, labelsWith(labels, "reason", "reliability"))
		}
		if sample.AppClockViolationSeconds != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "clock_violation", "application_clocks"), *sample.AppClockViolationSeconds, e.metrics.GPUClockViolationSeconds, labelsWith(labels, "reason", "application_clocks"))
		}
		if sample.BaseClockViolationSeconds != nil {
			e.observeHardwareCounter(hardwareCounterKey(sample.Info, "clock_violation", "base_clocks"), *sample.BaseClockViolationSeconds, e.metrics.GPUClockViolationSeconds, labelsWith(labels, "reason", "base_clocks"))
		}
		if sample.ProfGraphicsEngineActive != nil {
			e.metrics.GPUProfGraphicsEngineActive.With(labels).Set(*sample.ProfGraphicsEngineActive)
		}
		if sample.ProfSMActive != nil {
			e.metrics.GPUProfSMActive.With(labels).Set(*sample.ProfSMActive)
			e.window.Observe(gpuIndex, aggProfSM, *sample.ProfSMActive)
			if dtSeconds > 0 {
				e.metrics.GPUSMActiveWeightedSeconds.With(labels).Add(ratioFraction(*sample.ProfSMActive) * dtSeconds)
			}
		}
		if sample.ProfSMOccupancy != nil {
			e.metrics.GPUProfSMOccupancy.With(labels).Set(*sample.ProfSMOccupancy)
		}
		if sample.ProfDRAMActive != nil {
			e.metrics.GPUProfDRAMActive.With(labels).Set(*sample.ProfDRAMActive)
			e.window.Observe(gpuIndex, aggProfDRAM, *sample.ProfDRAMActive)
			if dtSeconds > 0 {
				e.metrics.GPUDRAMActiveWeightedSeconds.With(labels).Add(ratioFraction(*sample.ProfDRAMActive) * dtSeconds)
			}
		}
		if sample.ProfTensorActive != nil {
			e.metrics.GPUProfTensorPipe.With(labels).Set(*sample.ProfTensorActive)
			e.window.Observe(gpuIndex, aggProfTensor, *sample.ProfTensorActive)
			if dtSeconds > 0 {
				e.metrics.GPUTensorWeightedSeconds.With(labels).Add(ratioFraction(*sample.ProfTensorActive) * dtSeconds)
			}
		}
		if sample.ProfPipeFP64Active != nil {
			e.metrics.GPUProfPipeFP64Active.With(labels).Set(*sample.ProfPipeFP64Active)
		}
		if sample.ProfPipeFP32Active != nil {
			e.metrics.GPUProfPipeFP32Active.With(labels).Set(*sample.ProfPipeFP32Active)
		}
		if sample.ProfPipeFP16Active != nil {
			e.metrics.GPUProfPipeFP16Active.With(labels).Set(*sample.ProfPipeFP16Active)
		}
		if sample.ProfPipeINTActive != nil {
			e.metrics.GPUProfPipeINTActive.With(labels).Set(*sample.ProfPipeINTActive)
		}
		if sample.ProfTensorHMMAActive != nil {
			e.metrics.GPUProfTensorHMMAActive.With(labels).Set(*sample.ProfTensorHMMAActive)
		}
		if sample.ProfTensorIMMAActive != nil {
			e.metrics.GPUProfTensorIMMAActive.With(labels).Set(*sample.ProfTensorIMMAActive)
		}
		if sample.ProfTensorDFMAActive != nil {
			e.metrics.GPUProfTensorDFMAActive.With(labels).Set(*sample.ProfTensorDFMAActive)
		}
	}

	// Кэшируем identity GPU для FlushWindow: скрейп не должен ходить в DCGM.
	e.infoMu.Lock()
	e.lastInfos = infos
	e.infoMu.Unlock()

	return nil
}

func (e *Exporter) markCollectSuccess(now time.Time, gpuCount int) {
	e.healthMu.Lock()
	e.lastSuccess = now
	e.lastGPUCount = gpuCount
	e.healthMu.Unlock()

	labels := exporterLabelsFor(e.hostname)
	e.metrics.ExporterUp.With(labels).Set(1)
	e.metrics.ExporterCollectSuccess.With(labels).Set(1)
	e.metrics.ExporterLastSuccessTimestamp.With(labels).Set(float64(now.Unix()))
	e.metrics.ExporterDiscoveredGPUs.With(labels).Set(float64(gpuCount))
}

func (e *Exporter) markCollectFailure() {
	labels := exporterLabelsFor(e.hostname)
	e.metrics.ExporterUp.With(labels).Set(0)
	e.metrics.ExporterCollectSuccess.With(labels).Set(0)
	e.metrics.ExporterCollectionErrors.With(labels).Inc()
}

func (e *Exporter) observeHardwareCounter(key string, value float64, counter *prometheus.CounterVec, labels prometheus.Labels) {
	if value < 0 {
		return
	}

	e.hardwareCounterMu.Lock()
	if e.hardwareCounters == nil {
		e.hardwareCounters = make(map[string]float64)
	}
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

	if delta > 0 {
		counter.With(labels).Add(delta)
	}
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

func labelsWith(base prometheus.Labels, keyValues ...string) prometheus.Labels {
	labels := prometheus.Labels{}
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
	for _, value := range []string{info.UUID, info.PCIBusID, info.Index} {
		parsed := validDCGMString(value)
		if parsed != "" && parsed != "unknown" {
			return parsed
		}
	}
	return "unknown"
}

func (e *Exporter) Ready(now time.Time) bool {
	e.healthMu.RLock()
	lastSuccess := e.lastSuccess
	gpuCount := e.lastGPUCount
	e.healthMu.RUnlock()

	if lastSuccess.IsZero() || gpuCount == 0 {
		return false
	}
	return now.Sub(lastSuccess) <= e.maxSampleGap()
}

func (e *Exporter) sampleInterval(gpuIndex string, now time.Time) float64 {
	previous, ok := e.lastSeen[gpuIndex]
	e.lastSeen[gpuIndex] = now
	if !ok {
		return 0
	}
	elapsed := now.Sub(previous)
	if elapsed <= 0 {
		return 0
	}
	if elapsed > e.maxSampleGap() {
		return 0
	}
	return elapsed.Seconds()
}

func (e *Exporter) maxSampleGap() time.Duration {
	gap := e.cfg.ScrapeInterval * 10
	if gap < 5*time.Second {
		return 5 * time.Second
	}
	return gap
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
	if value <= 0 {
		return 0
	}
	if value <= 1 {
		return value
	}
	if value >= 100 {
		return 1
	}
	return value / 100
}
