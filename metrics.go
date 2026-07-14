package main

import "github.com/prometheus/client_golang/prometheus"

type WindowMetric struct {
	Max *prometheus.GaugeVec
	Avg *prometheus.GaugeVec
}

type IntegralMetric struct {
	Weighted *prometheus.CounterVec
	Observed *prometheus.CounterVec
	Percent  bool
}

// Metrics содержит все экспортируемые серии и списки GPU-векторов,
// необходимые для удаления серий исчезнувших устройств.
type Metrics struct {
	GPUUtilizationCurrent       *prometheus.GaugeVec
	GPUMemoryFree               *prometheus.GaugeVec
	GPUMemoryUsed               *prometheus.GaugeVec
	GPUMemoryTotal              *prometheus.GaugeVec
	GPUMemoryReserved           *prometheus.GaugeVec
	GPUBAR1MemoryFree           *prometheus.GaugeVec
	GPUBAR1MemoryUsed           *prometheus.GaugeVec
	GPUBAR1MemoryTotal          *prometheus.GaugeVec
	GPUMemoryTemperature        *prometheus.GaugeVec
	GPUMemoryTemperatureMaxOp   *prometheus.GaugeVec
	GPUMemoryTemperatureMaxOld  *prometheus.GaugeVec
	GPUTemperature              *prometheus.GaugeVec
	GPUTemperatureMaxOp         *prometheus.GaugeVec
	GPUTemperatureMaxOld        *prometheus.GaugeVec
	GPUPowerDraw                *prometheus.GaugeVec
	GPUPowerDrawInstant         *prometheus.GaugeVec
	GPUPowerLimit               *prometheus.GaugeVec
	GPUPowerEnforcedLimit       *prometheus.GaugeVec
	GPUMemoryCopyUtil           *prometheus.GaugeVec
	GPUEncoderUtil              *prometheus.GaugeVec
	GPUDecoderUtil              *prometheus.GaugeVec
	GPUMemoryUsedPct            *prometheus.GaugeVec
	GPUSMClockHertz             *prometheus.GaugeVec
	GPUMemoryClockHertz         *prometheus.GaugeVec
	GPUPerformanceState         *prometheus.GaugeVec
	GPUFanSpeed                 *prometheus.GaugeVec
	GPUPCIeTransmitRate         *prometheus.GaugeVec
	GPUPCIeReceiveRate          *prometheus.GaugeVec
	GPUPCIeLinkGeneration       *prometheus.GaugeVec
	GPUPCIeLinkWidth            *prometheus.GaugeVec
	GPUPCIeMaxLinkGeneration    *prometheus.GaugeVec
	GPUPCIeMaxLinkWidth         *prometheus.GaugeVec
	GPUNVLinkTransmitRate       *prometheus.GaugeVec
	GPUNVLinkReceiveRate        *prometheus.GaugeVec
	GPUXIDLastCode              *prometheus.GaugeVec
	GPUThrottleReason           *prometheus.GaugeVec
	GPUClockEventActive         *prometheus.GaugeVec
	GPUProfGraphicsEngineActive *prometheus.GaugeVec
	GPUProfSMActive             *prometheus.GaugeVec
	GPUProfSMOccupancy          *prometheus.GaugeVec
	GPUProfDRAMActive           *prometheus.GaugeVec
	GPUProfTensorPipe           *prometheus.GaugeVec
	GPUProfPipeFP64Active       *prometheus.GaugeVec
	GPUProfPipeFP32Active       *prometheus.GaugeVec
	GPUProfPipeFP16Active       *prometheus.GaugeVec
	GPUProfPipeINTActive        *prometheus.GaugeVec
	GPUProfTensorHMMAActive     *prometheus.GaugeVec
	GPUProfTensorIMMAActive     *prometheus.GaugeVec
	GPUProfTensorDFMAActive     *prometheus.GaugeVec

	GPUFieldSupported            *prometheus.GaugeVec
	GPUFieldAvailable            *prometheus.GaugeVec
	GPUFieldLastSuccessTimestamp *prometheus.GaugeVec

	GPUDriverVersion  *prometheus.GaugeVec
	GPUCudaVersion    *prometheus.GaugeVec
	ExporterBuildInfo *prometheus.GaugeVec

	ExporterUp                   *prometheus.GaugeVec
	ExporterCollectSuccess       *prometheus.GaugeVec
	ExporterLastSuccessTimestamp *prometheus.GaugeVec
	ExporterCollectionDuration   *prometheus.GaugeVec
	ExporterCollectionErrors     *prometheus.CounterVec
	ExporterDiscoveredGPUs       *prometheus.GaugeVec
	ExporterCollectedGPUs        *prometheus.GaugeVec
	ExporterFailedGPUs           *prometheus.GaugeVec
	GPUCollectSuccess            *prometheus.GaugeVec
	GPUCollectionErrors          *prometheus.CounterVec

	GPURequestCount          *prometheus.CounterVec
	GPUActivityWindows       *prometheus.CounterVec
	GPUActiveSeconds         *prometheus.CounterVec
	GPUEnergyJoules          *prometheus.CounterVec
	GPUEnergyEstimated       *prometheus.CounterVec
	GPUPCIeReplayTotal       *prometheus.CounterVec
	GPUECCErrors             *prometheus.CounterVec
	GPURetiredPages          *prometheus.CounterVec
	GPURetiredPagesPending   *prometheus.GaugeVec
	GPURemappedRows          *prometheus.CounterVec
	GPURowRemapFailure       *prometheus.GaugeVec
	GPURowRemapPending       *prometheus.GaugeVec
	GPUClockViolationSeconds *prometheus.CounterVec

	WindowMetrics map[string]WindowMetric
	Integrals     map[string]IntegralMetric
	RateTotals    map[string]*prometheus.CounterVec

	gpuGauges   []*prometheus.GaugeVec
	gpuCounters []*prometheus.CounterVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		WindowMetrics: make(map[string]WindowMetric),
		Integrals:     make(map[string]IntegralMetric),
		RateTotals:    make(map[string]*prometheus.CounterVec),
	}

	gg := func(name, help string, extra ...string) *prometheus.GaugeVec {
		metric := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, gpuLabelsWith(extra...))
		reg.MustRegister(metric)
		m.gpuGauges = append(m.gpuGauges, metric)
		return metric
	}
	gc := func(name, help string, extra ...string) *prometheus.CounterVec {
		metric := prometheus.NewCounterVec(prometheus.CounterOpts{Name: name, Help: help}, gpuLabelsWith(extra...))
		reg.MustRegister(metric)
		m.gpuCounters = append(m.gpuCounters, metric)
		return metric
	}
	eg := func(name, help string) *prometheus.GaugeVec {
		metric := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: name, Help: help}, exporterLabels())
		reg.MustRegister(metric)
		return metric
	}
	ec := func(name, help string) *prometheus.CounterVec {
		metric := prometheus.NewCounterVec(prometheus.CounterOpts{Name: name, Help: help}, exporterLabels())
		reg.MustRegister(metric)
		return metric
	}

	m.GPUUtilizationCurrent = gg("gpu_utilization_percent_current", "Latest valid DCGM GPU utilization percentage.")
	m.GPUMemoryFree = gg("gpu_memory_free_bytes", "Free GPU framebuffer memory in bytes.")
	m.GPUMemoryUsed = gg("gpu_memory_used_bytes", "Used GPU framebuffer memory in bytes.")
	m.GPUMemoryTotal = gg("gpu_memory_total_bytes", "Total GPU framebuffer memory in bytes.")
	m.GPUMemoryReserved = gg("gpu_framebuffer_memory_reserved_bytes", "Reserved GPU framebuffer memory in bytes.")
	m.GPUBAR1MemoryFree = gg("gpu_bar1_memory_free_bytes", "Free GPU BAR1 memory in bytes.")
	m.GPUBAR1MemoryUsed = gg("gpu_bar1_memory_used_bytes", "Used GPU BAR1 memory in bytes.")
	m.GPUBAR1MemoryTotal = gg("gpu_bar1_memory_total_bytes", "Total GPU BAR1 memory in bytes.")
	m.GPUMemoryTemperature = gg("gpu_memory_temperature_celsius", "Current GPU memory temperature in Celsius.")
	m.GPUMemoryTemperatureMaxOp = gg("gpu_memory_temperature_max_operating_celsius", "Maximum operating memory temperature; slowdown occurs above this threshold.")
	m.GPUMemoryTemperatureMaxOld = gg("gpu_memory_temperature_max_celsius", "Deprecated alias for gpu_memory_temperature_max_operating_celsius.")
	m.GPUTemperature = gg("gpu_temperature_celsius", "Current GPU temperature in Celsius.")
	m.GPUTemperatureMaxOp = gg("gpu_temperature_max_operating_celsius", "Maximum operating GPU temperature; slowdown occurs above this threshold.")
	m.GPUTemperatureMaxOld = gg("gpu_temperature_max_celsius", "Deprecated alias for gpu_temperature_max_operating_celsius.")
	m.GPUPowerDraw = gg("gpu_power_draw_watts", "GPU power draw in watts.")
	m.GPUPowerDrawInstant = gg("gpu_power_draw_instant_watts", "Instantaneous GPU power draw in watts.")
	m.GPUPowerLimit = gg("gpu_power_limit_watts", "GPU power management limit in watts.")
	m.GPUPowerEnforcedLimit = gg("gpu_power_enforced_limit_watts", "Enforced GPU power limit in watts.")
	m.GPUMemoryCopyUtil = gg("gpu_memory_copy_utilization_percent", "GPU memory-copy utilization percentage.")
	m.GPUEncoderUtil = gg("gpu_encoder_utilization_percent", "GPU encoder utilization percentage.")
	m.GPUDecoderUtil = gg("gpu_decoder_utilization_percent", "GPU decoder utilization percentage.")
	m.GPUMemoryUsedPct = gg("gpu_framebuffer_memory_used_percent", "Framebuffer memory used percentage.")
	m.GPUSMClockHertz = gg("gpu_sm_clock_hertz", "Streaming multiprocessor clock in hertz.")
	m.GPUMemoryClockHertz = gg("gpu_memory_clock_hertz", "Memory clock in hertz.")
	m.GPUPerformanceState = gg("gpu_performance_state", "Current GPU performance state.")
	m.GPUFanSpeed = gg("gpu_fan_speed_percent", "GPU fan speed percentage.")
	m.GPUPCIeTransmitRate = gg("gpu_pcie_transmit_bytes_per_second", "PCIe transmit rate from GPU to host; DCP is preferred with legacy DCGM fallback.")
	m.GPUPCIeReceiveRate = gg("gpu_pcie_receive_bytes_per_second", "PCIe receive rate from host to GPU; DCP is preferred with legacy DCGM fallback.")
	m.GPUPCIeLinkGeneration = gg("gpu_pcie_link_generation", "Current PCIe link generation.")
	m.GPUPCIeLinkWidth = gg("gpu_pcie_link_width", "Current PCIe link width.")
	m.GPUPCIeMaxLinkGeneration = gg("gpu_pcie_max_link_generation", "Maximum PCIe link generation.")
	m.GPUPCIeMaxLinkWidth = gg("gpu_pcie_max_link_width", "Maximum PCIe link width.")
	m.GPUNVLinkTransmitRate = gg("gpu_nvlink_transmit_bytes_per_second", "GPU-level DCP NVLink transmit rate in bytes per second.")
	m.GPUNVLinkReceiveRate = gg("gpu_nvlink_receive_bytes_per_second", "GPU-level DCP NVLink receive rate in bytes per second.")
	m.GPUXIDLastCode = gg("gpu_xid_last_code", "Last XID error code reported by the driver.")
	m.GPUThrottleReason = gg("gpu_clock_throttle_reasons", "Current DCGM clock-event reason bitmask.")
	m.GPUClockEventActive = gg("gpu_clock_event_active", "Decoded DCGM clock-event reason state, 0 or 1.", "reason")
	m.GPUProfGraphicsEngineActive = gg("gpu_prof_graphics_engine_active_ratio", "Graphics-engine active ratio in the strict 0-1 range.")
	m.GPUProfSMActive = gg("gpu_prof_sm_active_ratio", "Streaming-multiprocessor active ratio in the strict 0-1 range.")
	m.GPUProfSMOccupancy = gg("gpu_prof_sm_occupancy_ratio", "SM occupancy ratio in the strict 0-1 range.")
	m.GPUProfDRAMActive = gg("gpu_prof_dram_active_ratio", "Device-memory interface active ratio in the strict 0-1 range.")
	m.GPUProfTensorPipe = gg("gpu_prof_pipe_tensor_active_ratio", "Tensor-pipe active ratio in the strict 0-1 range.")
	m.GPUProfPipeFP64Active = gg("gpu_prof_pipe_fp64_active_ratio", "FP64-pipe active ratio in the strict 0-1 range.")
	m.GPUProfPipeFP32Active = gg("gpu_prof_pipe_fp32_active_ratio", "FP32-pipe active ratio in the strict 0-1 range.")
	m.GPUProfPipeFP16Active = gg("gpu_prof_pipe_fp16_active_ratio", "FP16-pipe active ratio in the strict 0-1 range.")
	m.GPUProfPipeINTActive = gg("gpu_prof_pipe_int_active_ratio", "Integer-pipe active ratio in the strict 0-1 range.")
	m.GPUProfTensorHMMAActive = gg("gpu_prof_pipe_tensor_hmma_active_ratio", "Tensor HMMA-pipe active ratio in the strict 0-1 range.")
	m.GPUProfTensorIMMAActive = gg("gpu_prof_pipe_tensor_imma_active_ratio", "Tensor IMMA-pipe active ratio in the strict 0-1 range.")
	m.GPUProfTensorDFMAActive = gg("gpu_prof_pipe_tensor_dfma_active_ratio", "Tensor DFMA-pipe active ratio in the strict 0-1 range.")

	m.GPUFieldSupported = gg("gpu_dcgm_field_supported", "Whether the DCGM field is supported and watched for this GPU.", "field")
	m.GPUFieldAvailable = gg("gpu_dcgm_field_available", "Whether the latest scheduled read returned a valid value for this DCGM field.", "field")
	m.GPUFieldLastSuccessTimestamp = gg("gpu_dcgm_field_last_success_timestamp_seconds", "Unix timestamp of the last valid value for this DCGM field.", "field")

	m.GPUDriverVersion = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "gpu_driver_version_info", Help: "GPU driver version info with value 1."}, []string{"gpu_driver_version", "hostname"})
	m.GPUCudaVersion = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "gpu_cuda_version_info", Help: "CUDA driver version info with value 1."}, []string{"gpu_cuda_version", "hostname"})
	m.ExporterBuildInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "gpu_exporter_build_info", Help: "GPU exporter build information with value 1."}, []string{"version"})
	reg.MustRegister(m.GPUDriverVersion, m.GPUCudaVersion, m.ExporterBuildInfo)

	m.ExporterUp = eg("gpu_exporter_up", "Whether the exporter could query the DCGM device list during the last collection.")
	m.ExporterCollectSuccess = eg("gpu_exporter_collect_success", "Whether every discovered GPU and scheduled watcher group succeeded during the last collection.")
	m.ExporterLastSuccessTimestamp = eg("gpu_exporter_last_success_timestamp_seconds", "Unix timestamp of the last complete successful collection.")
	m.ExporterCollectionDuration = eg("gpu_exporter_collection_duration_seconds", "Duration of the last collection in seconds.")
	m.ExporterCollectionErrors = ec("gpu_exporter_collection_errors_total", "Total non-complete DCGM collection attempts.")
	m.ExporterDiscoveredGPUs = eg("gpu_exporter_discovered_gpus", "Number of DCGM-supported GPUs discovered during the last collection.")
	m.ExporterCollectedGPUs = eg("gpu_exporter_collected_gpus", "Number of GPUs with a successful fast-field collection during the last collection.")
	m.ExporterFailedGPUs = eg("gpu_exporter_failed_gpus", "Number of GPUs with one or more scheduled collection failures during the last collection.")
	m.GPUCollectSuccess = gg("gpu_exporter_gpu_collect_success", "Whether the GPU had no scheduled collection failures in the last collection.")
	m.GPUCollectionErrors = gc("gpu_exporter_gpu_collection_errors_total", "Total GPU collection errors by watcher group or stage.", "reason")

	m.GPURequestCount = gc("gpu_request_count_total", "Deprecated alias for gpu_activity_windows_total.")
	m.GPUActivityWindows = gc("gpu_activity_windows_total", "Total inferred GPU activity windows detected by utilization threshold crossings.")
	m.GPUActiveSeconds = gc("gpu_active_seconds_total", "Observed seconds where GPU utilization exceeded the configured active threshold.")
	m.GPUEnergyJoules = gc("gpu_energy_joules_total", "Hardware energy counter converted from DCGM millijoules to joules.")
	m.GPUEnergyEstimated = gc("gpu_energy_estimated_joules_total", "Energy estimated by integrating observed power draw over time.")
	m.GPUPCIeReplayTotal = gc("gpu_pcie_replay_total", "Monotonic PCIe replay count with hardware resets handled.")
	m.GPUECCErrors = gc("gpu_ecc_errors_total", "ECC errors by correctability, persistence, and location.", "correctability", "persistence", "location")
	m.GPURetiredPages = gc("gpu_retired_pages_total", "Retired page count by cause.", "cause")
	m.GPURetiredPagesPending = gg("gpu_retired_pages_pending", "Pages pending retirement.")
	m.GPURemappedRows = gc("gpu_remapped_rows_total", "Remapped row count by correctability.", "correctability")
	m.GPURowRemapFailure = gg("gpu_row_remap_failure", "Whether DCGM reports a row-remapping failure.")
	m.GPURowRemapPending = gg("gpu_row_remap_pending", "Whether DCGM reports pending row remapping.")
	m.GPUClockViolationSeconds = gc("gpu_clock_violation_seconds_total", "Clock or policy violation duration converted from nanoseconds to seconds.", "reason")

	addWindow := func(key, maxName, avgName, subject string) {
		m.WindowMetrics[key] = WindowMetric{
			Max: gg(maxName, "Maximum "+subject+" in the last completed exporter aggregation window."),
			Avg: gg(avgName, "Average "+subject+" in the last completed exporter aggregation window."),
		}
	}
	addWindow(aggUtilization, "gpu_utilization_percent", "gpu_utilization_percent_avg", "GPU utilization percentage")
	addWindow(aggMemoryCopyUtil, "gpu_memory_copy_utilization_percent_max", "gpu_memory_copy_utilization_percent_avg", "memory-copy utilization percentage")
	addWindow(aggMemoryUsedPct, "gpu_framebuffer_memory_used_percent_max", "gpu_framebuffer_memory_used_percent_avg", "framebuffer utilization percentage")
	addWindow(aggPowerDraw, "gpu_power_draw_watts_max", "gpu_power_draw_watts_avg", "GPU power draw")
	addWindow(aggTemperature, "gpu_temperature_celsius_max", "gpu_temperature_celsius_avg", "GPU temperature")
	addWindow(aggProfGraphics, "gpu_prof_graphics_engine_active_ratio_max", "gpu_prof_graphics_engine_active_ratio_avg", "graphics-engine active ratio")
	addWindow(aggProfSM, "gpu_prof_sm_active_ratio_max", "gpu_prof_sm_active_ratio_avg", "SM active ratio")
	addWindow(aggProfSMOccupancy, "gpu_prof_sm_occupancy_ratio_max", "gpu_prof_sm_occupancy_ratio_avg", "SM occupancy ratio")
	addWindow(aggProfDRAM, "gpu_prof_dram_active_ratio_max", "gpu_prof_dram_active_ratio_avg", "DRAM active ratio")
	addWindow(aggProfTensor, "gpu_prof_pipe_tensor_active_ratio_max", "gpu_prof_pipe_tensor_active_ratio_avg", "tensor-pipe active ratio")
	addWindow(aggProfFP64, "gpu_prof_pipe_fp64_active_ratio_max", "gpu_prof_pipe_fp64_active_ratio_avg", "FP64-pipe active ratio")
	addWindow(aggProfFP32, "gpu_prof_pipe_fp32_active_ratio_max", "gpu_prof_pipe_fp32_active_ratio_avg", "FP32-pipe active ratio")
	addWindow(aggProfFP16, "gpu_prof_pipe_fp16_active_ratio_max", "gpu_prof_pipe_fp16_active_ratio_avg", "FP16-pipe active ratio")
	addWindow(aggProfINT, "gpu_prof_pipe_int_active_ratio_max", "gpu_prof_pipe_int_active_ratio_avg", "integer-pipe active ratio")
	addWindow(aggProfTensorHMMA, "gpu_prof_pipe_tensor_hmma_active_ratio_max", "gpu_prof_pipe_tensor_hmma_active_ratio_avg", "tensor HMMA-pipe active ratio")
	addWindow(aggProfTensorIMMA, "gpu_prof_pipe_tensor_imma_active_ratio_max", "gpu_prof_pipe_tensor_imma_active_ratio_avg", "tensor IMMA-pipe active ratio")
	addWindow(aggProfTensorDFMA, "gpu_prof_pipe_tensor_dfma_active_ratio_max", "gpu_prof_pipe_tensor_dfma_active_ratio_avg", "tensor DFMA-pipe active ratio")
	addWindow(aggPCIeTransmit, "gpu_pcie_transmit_bytes_per_second_max", "gpu_pcie_transmit_bytes_per_second_avg", "PCIe transmit rate")
	addWindow(aggPCIeReceive, "gpu_pcie_receive_bytes_per_second_max", "gpu_pcie_receive_bytes_per_second_avg", "PCIe receive rate")
	addWindow(aggNVLinkTransmit, "gpu_nvlink_transmit_bytes_per_second_max", "gpu_nvlink_transmit_bytes_per_second_avg", "NVLink transmit rate")
	addWindow(aggNVLinkReceive, "gpu_nvlink_receive_bytes_per_second_max", "gpu_nvlink_receive_bytes_per_second_avg", "NVLink receive rate")

	addIntegral := func(key, weightedName, observedName, subject string, percent bool) {
		m.Integrals[key] = IntegralMetric{
			Weighted: gc(weightedName, "Total observed "+subject+" fraction-seconds."),
			Observed: gc(observedName, "Total seconds for which "+subject+" had valid DCGM samples."),
			Percent:  percent,
		}
	}
	addIntegral(aggUtilization, "gpu_utilization_weighted_seconds_total", "gpu_utilization_observed_seconds_total", "GPU utilization", true)
	addIntegral(aggProfGraphics, "gpu_prof_graphics_engine_active_weighted_seconds_total", "gpu_prof_graphics_engine_active_observed_seconds_total", "graphics-engine activity", false)
	addIntegral(aggProfSM, "gpu_sm_active_weighted_seconds_total", "gpu_sm_active_observed_seconds_total", "SM activity", false)
	addIntegral(aggProfSMOccupancy, "gpu_prof_sm_occupancy_weighted_seconds_total", "gpu_prof_sm_occupancy_observed_seconds_total", "SM occupancy", false)
	addIntegral(aggProfDRAM, "gpu_dram_active_weighted_seconds_total", "gpu_dram_active_observed_seconds_total", "DRAM activity", false)
	addIntegral(aggProfTensor, "gpu_tensor_active_weighted_seconds_total", "gpu_tensor_active_observed_seconds_total", "tensor-pipe activity", false)
	addIntegral(aggProfFP64, "gpu_prof_pipe_fp64_active_weighted_seconds_total", "gpu_prof_pipe_fp64_active_observed_seconds_total", "FP64-pipe activity", false)
	addIntegral(aggProfFP32, "gpu_prof_pipe_fp32_active_weighted_seconds_total", "gpu_prof_pipe_fp32_active_observed_seconds_total", "FP32-pipe activity", false)
	addIntegral(aggProfFP16, "gpu_prof_pipe_fp16_active_weighted_seconds_total", "gpu_prof_pipe_fp16_active_observed_seconds_total", "FP16-pipe activity", false)
	addIntegral(aggProfINT, "gpu_prof_pipe_int_active_weighted_seconds_total", "gpu_prof_pipe_int_active_observed_seconds_total", "integer-pipe activity", false)
	addIntegral(aggProfTensorHMMA, "gpu_prof_pipe_tensor_hmma_active_weighted_seconds_total", "gpu_prof_pipe_tensor_hmma_active_observed_seconds_total", "tensor HMMA-pipe activity", false)
	addIntegral(aggProfTensorIMMA, "gpu_prof_pipe_tensor_imma_active_weighted_seconds_total", "gpu_prof_pipe_tensor_imma_active_observed_seconds_total", "tensor IMMA-pipe activity", false)
	addIntegral(aggProfTensorDFMA, "gpu_prof_pipe_tensor_dfma_active_weighted_seconds_total", "gpu_prof_pipe_tensor_dfma_active_observed_seconds_total", "tensor DFMA-pipe activity", false)

	m.RateTotals[aggPCIeTransmit] = gc("gpu_pcie_transmitted_bytes_total", "Estimated PCIe bytes transmitted from GPU to host, integrated from observed rates.")
	m.RateTotals[aggPCIeReceive] = gc("gpu_pcie_received_bytes_total", "Estimated PCIe bytes received by GPU from host, integrated from observed rates.")
	m.RateTotals[aggNVLinkTransmit] = gc("gpu_nvlink_transmitted_bytes_total", "Estimated GPU-level NVLink bytes transmitted, integrated from observed rates.")
	m.RateTotals[aggNVLinkReceive] = gc("gpu_nvlink_received_bytes_total", "Estimated GPU-level NVLink bytes received, integrated from observed rates.")

	return m
}

func (m *Metrics) DeleteGPU(labels prometheus.Labels) {
	for _, metric := range m.gpuGauges {
		metric.DeletePartialMatch(labels)
	}
	for _, metric := range m.gpuCounters {
		metric.DeletePartialMatch(labels)
	}
}

func gpuLabels() []string {
	return []string{"gpu_index", "gpu_uuid", "pci_bus_id", "gpu_name", "hostname"}
}

func gpuLabelsWith(extra ...string) []string {
	labels := append([]string{}, gpuLabels()...)
	return append(labels, extra...)
}

func exporterLabels() []string {
	return []string{"hostname"}
}

func exporterLabelsFor(hostname string) prometheus.Labels {
	return prometheus.Labels{"hostname": hostname}
}

func labelsFor(info GPUInfo, hostname string) prometheus.Labels {
	return prometheus.Labels{
		"gpu_index":  firstNonEmpty(info.Index, "unknown"),
		"gpu_uuid":   firstNonEmpty(info.UUID, "unknown"),
		"pci_bus_id": firstNonEmpty(info.PCIBusID, "unknown"),
		"gpu_name":   firstNonEmpty(info.Name, "unknown"),
		"hostname":   hostname,
	}
}
