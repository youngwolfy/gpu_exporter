package main

import "github.com/prometheus/client_golang/prometheus"

// Metrics содержит все экспортируемые серии.
type Metrics struct {
	GPUUtilization    *prometheus.GaugeVec
	GPUMemoryFree     *prometheus.GaugeVec
	GPUMemoryUsed     *prometheus.GaugeVec
	GPUMemoryTotal    *prometheus.GaugeVec
	GPUTemperature    *prometheus.GaugeVec
	GPUTemperatureMax *prometheus.GaugeVec
	GPUPowerDraw      *prometheus.GaugeVec
	GPUPowerLimit     *prometheus.GaugeVec
	GPUMemoryCopyUtil *prometheus.GaugeVec
	GPUMemoryUsedPct  *prometheus.GaugeVec
	GPUMemoryCopyMax  *prometheus.GaugeVec
	GPUMemoryUsedMax  *prometheus.GaugeVec
	GPUPowerDrawMax   *prometheus.GaugeVec
	GPUTemperatureWin *prometheus.GaugeVec
	GPUProfSMMax      *prometheus.GaugeVec
	GPUProfDRAMMax    *prometheus.GaugeVec
	GPUProfTensorMax  *prometheus.GaugeVec
	GPUUtilizationAvg *prometheus.GaugeVec
	GPUMemoryCopyAvg  *prometheus.GaugeVec
	GPUMemoryUsedAvg  *prometheus.GaugeVec
	GPUPowerDrawAvg   *prometheus.GaugeVec
	GPUTemperatureAvg *prometheus.GaugeVec
	GPUProfSMAvg      *prometheus.GaugeVec
	GPUProfDRAMAvg    *prometheus.GaugeVec
	GPUProfTensorAvg  *prometheus.GaugeVec
	GPUThrottleReason *prometheus.GaugeVec
	GPUProfSMActive   *prometheus.GaugeVec
	GPUProfDRAMActive *prometheus.GaugeVec
	GPUProfTensorPipe *prometheus.GaugeVec
	GPUDriverVersion  *prometheus.GaugeVec
	GPUCudaVersion    *prometheus.GaugeVec
	GPURequestCount   *prometheus.CounterVec

	GPUActiveSeconds              *prometheus.CounterVec
	GPUUtilizationWeightedSeconds *prometheus.CounterVec
	GPUSMActiveWeightedSeconds    *prometheus.CounterVec
	GPUDRAMActiveWeightedSeconds  *prometheus.CounterVec
	GPUTensorWeightedSeconds      *prometheus.CounterVec
	GPUEnergyJoules               *prometheus.CounterVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		GPUUtilization: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_utilization_percent",
			Help: "Peak GPU utilization percentage between Prometheus scrapes.",
		}, gpuLabels()),
		GPUMemoryFree: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_free_bytes",
			Help: "Free GPU framebuffer memory in bytes.",
		}, gpuLabels()),
		GPUMemoryUsed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_used_bytes",
			Help: "Used GPU framebuffer memory in bytes.",
		}, gpuLabels()),
		GPUMemoryTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_total_bytes",
			Help: "Total GPU framebuffer memory in bytes.",
		}, gpuLabels()),
		GPUTemperature: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_temperature_celsius",
			Help: "GPU temperature in Celsius.",
		}, gpuLabels()),
		GPUTemperatureMax: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_temperature_max_celsius",
			Help: "GPU maximum temperature in Celsius when available.",
		}, gpuLabels()),
		GPUPowerDraw: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_power_draw_watts",
			Help: "GPU power draw in watts.",
		}, gpuLabels()),
		GPUPowerLimit: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_power_limit_watts",
			Help: "GPU power management limit in watts when available.",
		}, gpuLabels()),
		GPUMemoryCopyUtil: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_copy_utilization_percent",
			Help: "DCGM_FI_DEV_MEM_COPY_UTIL memory copy utilization percentage.",
		}, gpuLabels()),
		GPUMemoryUsedPct: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_framebuffer_memory_used_percent",
			Help: "DCGM_FI_DEV_FB_USED_PERCENT framebuffer memory used percentage.",
		}, gpuLabels()),
		GPUMemoryCopyMax: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_copy_utilization_percent_max",
			Help: "Peak memory copy utilization percentage between Prometheus scrapes.",
		}, gpuLabels()),
		GPUMemoryUsedMax: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_framebuffer_memory_used_percent_max",
			Help: "Peak framebuffer memory used percentage between Prometheus scrapes.",
		}, gpuLabels()),
		GPUPowerDrawMax: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_power_draw_watts_max",
			Help: "Peak GPU power draw in watts between Prometheus scrapes.",
		}, gpuLabels()),
		GPUTemperatureWin: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_temperature_celsius_max",
			Help: "Peak GPU temperature in Celsius between Prometheus scrapes.",
		}, gpuLabels()),
		GPUProfSMMax: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_sm_active_ratio_max",
			Help: "Peak DCGM_FI_PROF_SM_ACTIVE ratio between Prometheus scrapes.",
		}, gpuLabels()),
		GPUProfDRAMMax: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_dram_active_ratio_max",
			Help: "Peak DCGM_FI_PROF_DRAM_ACTIVE ratio between Prometheus scrapes.",
		}, gpuLabels()),
		GPUProfTensorMax: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_pipe_tensor_active_ratio_max",
			Help: "Peak DCGM_FI_PROF_PIPE_TENSOR_ACTIVE ratio between Prometheus scrapes.",
		}, gpuLabels()),
		GPUUtilizationAvg: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_utilization_percent_avg",
			Help: "Average GPU utilization percentage between Prometheus scrapes.",
		}, gpuLabels()),
		GPUMemoryCopyAvg: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_copy_utilization_percent_avg",
			Help: "Average memory copy utilization percentage between Prometheus scrapes.",
		}, gpuLabels()),
		GPUMemoryUsedAvg: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_framebuffer_memory_used_percent_avg",
			Help: "Average framebuffer memory used percentage between Prometheus scrapes.",
		}, gpuLabels()),
		GPUPowerDrawAvg: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_power_draw_watts_avg",
			Help: "Average GPU power draw in watts between Prometheus scrapes.",
		}, gpuLabels()),
		GPUTemperatureAvg: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_temperature_celsius_avg",
			Help: "Average GPU temperature in Celsius between Prometheus scrapes.",
		}, gpuLabels()),
		GPUProfSMAvg: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_sm_active_ratio_avg",
			Help: "Average DCGM_FI_PROF_SM_ACTIVE ratio between Prometheus scrapes.",
		}, gpuLabels()),
		GPUProfDRAMAvg: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_dram_active_ratio_avg",
			Help: "Average DCGM_FI_PROF_DRAM_ACTIVE ratio between Prometheus scrapes.",
		}, gpuLabels()),
		GPUProfTensorAvg: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_pipe_tensor_active_ratio_avg",
			Help: "Average DCGM_FI_PROF_PIPE_TENSOR_ACTIVE ratio between Prometheus scrapes.",
		}, gpuLabels()),
		GPUThrottleReason: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_clock_throttle_reasons",
			Help: "DCGM_FI_DEV_CLOCK_THROTTLE_REASONS current clock throttle reason bitmask.",
		}, gpuLabels()),
		GPUProfSMActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_sm_active_ratio",
			Help: "DCGM_FI_PROF_SM_ACTIVE ratio of time streaming multiprocessors were active.",
		}, gpuLabels()),
		GPUProfDRAMActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_dram_active_ratio",
			Help: "DCGM_FI_PROF_DRAM_ACTIVE ratio of time the device memory interface was active.",
		}, gpuLabels()),
		GPUProfTensorPipe: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_pipe_tensor_active_ratio",
			Help: "DCGM_FI_PROF_PIPE_TENSOR_ACTIVE ratio of time the tensor pipe was active.",
		}, gpuLabels()),
		GPUDriverVersion: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_driver_version_info",
			Help: "GPU driver version info with value 1.",
		}, []string{"gpu_driver_version", "hostname"}),
		GPUCudaVersion: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_cuda_version_info",
			Help: "CUDA driver version info with value 1.",
		}, []string{"gpu_cuda_version", "hostname"}),
		GPURequestCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_request_count_total",
			Help: "Total inferred GPU work requests detected by utilization activity windows.",
		}, gpuLabels()),
		GPUActiveSeconds: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_active_seconds_total",
			Help: "Total seconds where GPU utilization was above the configured active threshold.",
		}, gpuLabels()),
		GPUUtilizationWeightedSeconds: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_utilization_weighted_seconds_total",
			Help: "Total GPU utilization fraction-seconds, computed as utilization_percent / 100 * elapsed_seconds.",
		}, gpuLabels()),
		GPUSMActiveWeightedSeconds: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_sm_active_weighted_seconds_total",
			Help: "Total DCGM SM active ratio-seconds, computed as normalized SM active ratio * elapsed_seconds.",
		}, gpuLabels()),
		GPUDRAMActiveWeightedSeconds: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_dram_active_weighted_seconds_total",
			Help: "Total DCGM DRAM active ratio-seconds, computed as normalized DRAM active ratio * elapsed_seconds.",
		}, gpuLabels()),
		GPUTensorWeightedSeconds: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_tensor_active_weighted_seconds_total",
			Help: "Total DCGM tensor pipe active ratio-seconds, computed as normalized tensor pipe active ratio * elapsed_seconds.",
		}, gpuLabels()),
		GPUEnergyJoules: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_energy_joules_total",
			Help: "Total GPU energy estimate in joules, computed as power_watts * elapsed_seconds.",
		}, gpuLabels()),
	}

	reg.MustRegister(
		m.GPUUtilization,
		m.GPUMemoryFree,
		m.GPUMemoryUsed,
		m.GPUMemoryTotal,
		m.GPUTemperature,
		m.GPUTemperatureMax,
		m.GPUPowerDraw,
		m.GPUPowerLimit,
		m.GPUMemoryCopyUtil,
		m.GPUMemoryUsedPct,
		m.GPUMemoryCopyMax,
		m.GPUMemoryUsedMax,
		m.GPUPowerDrawMax,
		m.GPUTemperatureWin,
		m.GPUProfSMMax,
		m.GPUProfDRAMMax,
		m.GPUProfTensorMax,
		m.GPUUtilizationAvg,
		m.GPUMemoryCopyAvg,
		m.GPUMemoryUsedAvg,
		m.GPUPowerDrawAvg,
		m.GPUTemperatureAvg,
		m.GPUProfSMAvg,
		m.GPUProfDRAMAvg,
		m.GPUProfTensorAvg,
		m.GPUThrottleReason,
		m.GPUProfSMActive,
		m.GPUProfDRAMActive,
		m.GPUProfTensorPipe,
		m.GPUDriverVersion,
		m.GPUCudaVersion,
		m.GPURequestCount,
		m.GPUActiveSeconds,
		m.GPUUtilizationWeightedSeconds,
		m.GPUSMActiveWeightedSeconds,
		m.GPUDRAMActiveWeightedSeconds,
		m.GPUTensorWeightedSeconds,
		m.GPUEnergyJoules,
	)

	return m
}

func gpuLabels() []string {
	return []string{"gpu_index", "gpu_name", "hostname"}
}

func labelsFor(info GPUInfo, hostname string) prometheus.Labels {
	return prometheus.Labels{
		"gpu_index": info.Index,
		"gpu_name":  info.Name,
		"hostname":  hostname,
	}
}
