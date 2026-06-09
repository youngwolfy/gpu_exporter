package main

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type Exporter struct {
	cfg      Config
	client   *DCGMClient
	metrics  *Metrics
	peaks    *PeakTracker
	activity *ActivityTracker
	hostname string
	logger   *slog.Logger
}

func NewExporter(cfg Config, client *DCGMClient, metrics *Metrics, hostname string, logger *slog.Logger) *Exporter {
	return &Exporter{
		cfg:      cfg,
		client:   client,
		metrics:  metrics,
		peaks:    NewPeakTracker(),
		activity: NewActivityTracker(cfg.ActiveThreshold, cfg.MinRequestTime),
		hostname: hostname,
		logger:   logger,
	}
}

func (e *Exporter) SetStaticMetrics() {
	driverVersion, cudaVersion := e.client.StaticInfo()

	e.metrics.GPUDriverVersion.WithLabelValues(driverVersion, e.hostname).Set(1)
	e.metrics.GPUCudaVersion.WithLabelValues(cudaVersion, e.hostname).Set(1)
}

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

func (e *Exporter) FlushPeaks() error {
	samples, err := e.client.Samples()
	if err != nil {
		return err
	}

	for _, sample := range samples {
		labels := labelsFor(sample.Info, e.hostname)
		gpuIndex := sample.Info.Index
		// FlushPeaks is called from the /metrics handler, so each scrape gets
		// the peak values observed since the previous scrape.
		e.metrics.GPUUtilization.With(labels).Set(e.peaks.GetAndReset(peakKey(gpuIndex, "utilization")))
		e.metrics.GPUMemoryCopyMax.With(labels).Set(e.peaks.GetAndReset(peakKey(gpuIndex, "memory_copy_util")))
		e.metrics.GPUMemoryUsedMax.With(labels).Set(e.peaks.GetAndReset(peakKey(gpuIndex, "memory_used_percent")))
		e.metrics.GPUPowerDrawMax.With(labels).Set(e.peaks.GetAndReset(peakKey(gpuIndex, "power_draw")))
		e.metrics.GPUTemperatureWin.With(labels).Set(e.peaks.GetAndReset(peakKey(gpuIndex, "temperature")))
		e.metrics.GPUProfSMMax.With(labels).Set(e.peaks.GetAndReset(peakKey(gpuIndex, "prof_sm")))
		e.metrics.GPUProfDRAMMax.With(labels).Set(e.peaks.GetAndReset(peakKey(gpuIndex, "prof_dram")))
		e.metrics.GPUProfTensorMax.With(labels).Set(e.peaks.GetAndReset(peakKey(gpuIndex, "prof_tensor")))
	}

	return nil
}

func (e *Exporter) collect() error {
	samples, err := e.client.Samples()
	if err != nil {
		return err
	}

	now := time.Now()
	for _, sample := range samples {
		labels := labelsFor(sample.Info, e.hostname)
		gpuIndex := sample.Info.Index
		// Current samples update normal gauges, while selected fields are also
		// tracked as scrape-window peaks.
		e.peaks.Update(peakKey(gpuIndex, "utilization"), sample.Utilization)
		e.peaks.Update(peakKey(gpuIndex, "power_draw"), sample.PowerDrawWatts)
		e.peaks.Update(peakKey(gpuIndex, "temperature"), sample.Temperature)

		e.metrics.GPUMemoryFree.With(labels).Set(sample.MemoryFreeBytes)
		e.metrics.GPUMemoryUsed.With(labels).Set(sample.MemoryUsedBytes)
		e.metrics.GPUMemoryTotal.With(labels).Set(sample.MemoryTotalBytes)
		e.metrics.GPUTemperature.With(labels).Set(sample.Temperature)
		e.metrics.GPUPowerDraw.With(labels).Set(sample.PowerDrawWatts)

		if sample.MemoryCopyUtil != nil {
			e.metrics.GPUMemoryCopyUtil.With(labels).Set(*sample.MemoryCopyUtil)
			e.peaks.Update(peakKey(gpuIndex, "memory_copy_util"), *sample.MemoryCopyUtil)
		} else {
			e.metrics.GPUMemoryCopyUtil.With(labels).Set(0)
		}
		if sample.MemoryUsedPercent != nil {
			e.metrics.GPUMemoryUsedPct.With(labels).Set(*sample.MemoryUsedPercent)
			e.peaks.Update(peakKey(gpuIndex, "memory_used_percent"), *sample.MemoryUsedPercent)
		} else {
			e.metrics.GPUMemoryUsedPct.With(labels).Set(0)
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
			e.peaks.Update(peakKey(gpuIndex, "prof_sm"), *sample.ProfSMActive)
		} else {
			e.metrics.GPUProfSMActive.With(labels).Set(0)
		}
		if sample.ProfDRAMActive != nil {
			e.metrics.GPUProfDRAMActive.With(labels).Set(*sample.ProfDRAMActive)
			e.peaks.Update(peakKey(gpuIndex, "prof_dram"), *sample.ProfDRAMActive)
		} else {
			e.metrics.GPUProfDRAMActive.With(labels).Set(0)
		}
		if sample.ProfTensorActive != nil {
			e.metrics.GPUProfTensorPipe.With(labels).Set(*sample.ProfTensorActive)
			e.peaks.Update(peakKey(gpuIndex, "prof_tensor"), *sample.ProfTensorActive)
		} else {
			e.metrics.GPUProfTensorPipe.With(labels).Set(0)
		}
		if sample.ProcessCount != nil {
			e.metrics.GPUProcesses.With(labels).Set(*sample.ProcessCount)
		} else if sample.ProcessCountError != nil && !errors.Is(sample.ProcessCountError, context.Canceled) {
			e.logger.Debug("GPU process count unavailable", "gpu_index", sample.Info.Index, "error", sample.ProcessCountError)
		}

		if e.activity.Observe(sample.Info.Index, sample.Utilization, now) {
			e.metrics.GPURequestCount.With(labels).Inc()
		}
	}

	return nil
}

func peakKey(gpuIndex, metric string) string {
	return gpuIndex + ":" + metric
}
