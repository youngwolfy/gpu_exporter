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

	// infoMu защищает lastInfos - список GPU, закешированный последним
	// collect(). FlushWindow использует кэш вместо повторного
	// запроса к DCGM на каждый скрейп.
	infoMu    sync.RWMutex
	lastInfos []GPUInfo
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
	samples, err := e.client.Samples()
	if err != nil {
		return err
	}

	now := time.Now()
	infos := make([]GPUInfo, 0, len(samples))
	for _, sample := range samples {
		infos = append(infos, sample.Info)
		labels := labelsFor(sample.Info, e.hostname)
		gpuIndex := sample.Info.Index
		dtSeconds := e.sampleInterval(gpuIndex, now)

		e.metrics.GPUMemoryFree.With(labels).Set(sample.MemoryFreeBytes)
		e.metrics.GPUMemoryUsed.With(labels).Set(sample.MemoryUsedBytes)
		e.metrics.GPUMemoryTotal.With(labels).Set(sample.MemoryTotalBytes)

		if sample.Utilization != nil {
			e.window.Observe(gpuIndex, aggUtilization, *sample.Utilization)
			if dtSeconds > 0 {
				e.metrics.GPUUtilizationWeightedSeconds.With(labels).Add(percentFraction(*sample.Utilization) * dtSeconds)
				if *sample.Utilization > e.cfg.ActiveThreshold {
					e.metrics.GPUActiveSeconds.With(labels).Add(dtSeconds)
				}
			}

			if e.activity.Observe(gpuIndex, *sample.Utilization, now) {
				e.metrics.GPURequestCount.With(labels).Inc()
			}
		}
		if sample.Temperature != nil {
			e.metrics.GPUTemperature.With(labels).Set(*sample.Temperature)
			e.window.Observe(gpuIndex, aggTemperature, *sample.Temperature)
		}
		if sample.PowerDrawWatts != nil {
			e.metrics.GPUPowerDraw.With(labels).Set(*sample.PowerDrawWatts)
			e.window.Observe(gpuIndex, aggPowerDraw, *sample.PowerDrawWatts)
			if dtSeconds > 0 {
				e.metrics.GPUEnergyJoules.With(labels).Add(*sample.PowerDrawWatts * dtSeconds)
			}
		}
		if sample.MemoryCopyUtil != nil {
			e.metrics.GPUMemoryCopyUtil.With(labels).Set(*sample.MemoryCopyUtil)
			e.window.Observe(gpuIndex, aggMemoryCopyUtil, *sample.MemoryCopyUtil)
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
		if sample.ThrottleReasons != nil {
			e.metrics.GPUThrottleReason.With(labels).Set(*sample.ThrottleReasons)
		} else {
			e.metrics.GPUThrottleReason.With(labels).Set(0)
		}
		if sample.ProfSMActive != nil {
			e.metrics.GPUProfSMActive.With(labels).Set(*sample.ProfSMActive)
			e.window.Observe(gpuIndex, aggProfSM, *sample.ProfSMActive)
			if dtSeconds > 0 {
				e.metrics.GPUSMActiveWeightedSeconds.With(labels).Add(ratioFraction(*sample.ProfSMActive) * dtSeconds)
			}
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
	}

	// Кэшируем identity GPU для FlushWindow: скрейп не должен ходить в DCGM.
	e.infoMu.Lock()
	e.lastInfos = infos
	e.infoMu.Unlock()

	return nil
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
