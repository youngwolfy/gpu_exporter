package main

import "github.com/prometheus/client_golang/prometheus"

// Metrics содержит все экспортируемые серии.
type Metrics struct {
	GPUUtilization             *prometheus.GaugeVec
	GPUUtilizationCurrent      *prometheus.GaugeVec
	GPUMemoryFree              *prometheus.GaugeVec
	GPUMemoryUsed              *prometheus.GaugeVec
	GPUMemoryTotal             *prometheus.GaugeVec
	GPUMemoryReserved          *prometheus.GaugeVec
	GPUBAR1MemoryFree          *prometheus.GaugeVec
	GPUBAR1MemoryUsed          *prometheus.GaugeVec
	GPUBAR1MemoryTotal         *prometheus.GaugeVec
	GPUMemoryTemperature       *prometheus.GaugeVec
	GPUMemoryTemperatureMax    *prometheus.GaugeVec
	GPUTemperature             *prometheus.GaugeVec
	GPUTemperatureMax          *prometheus.GaugeVec
	GPUPowerDraw               *prometheus.GaugeVec
	GPUPowerDrawInstant        *prometheus.GaugeVec
	GPUPowerLimit              *prometheus.GaugeVec
	GPUPowerEnforcedLimit      *prometheus.GaugeVec
	GPUMemoryCopyUtil          *prometheus.GaugeVec
	GPUEncoderUtil             *prometheus.GaugeVec
	GPUDecoderUtil             *prometheus.GaugeVec
	GPUMemoryUsedPct           *prometheus.GaugeVec
	GPUSMClockHertz            *prometheus.GaugeVec
	GPUMemoryClockHertz        *prometheus.GaugeVec
	GPUPerformanceState        *prometheus.GaugeVec
	GPUFanSpeed                *prometheus.GaugeVec
	GPUPCIeTXBytesPerSecond    *prometheus.GaugeVec
	GPUPCIeRXBytesPerSecond    *prometheus.GaugeVec
	GPUPCIeTransmitBytesPerSecond *prometheus.GaugeVec
	GPUPCIeReceiveBytesPerSecond *prometheus.GaugeVec
	GPUPCIeLinkGeneration      *prometheus.GaugeVec
	GPUPCIeLinkWidth           *prometheus.GaugeVec
	GPUPCIeMaxLinkGeneration   *prometheus.GaugeVec
	GPUPCIeMaxLinkWidth        *prometheus.GaugeVec
	GPUNVLinkTXBytesPerSecond  *prometheus.GaugeVec
	GPUNVLinkRXBytesPerSecond  *prometheus.GaugeVec
	GPUNVLinkTransmitBytesPerSecond *prometheus.GaugeVec
	GPUNVLinkReceiveBytesPerSecond *prometheus.GaugeVec
	GPUXIDLastCode             *prometheus.GaugeVec
	GPUMemoryCopyMax           *prometheus.GaugeVec
	GPUMemoryUsedMax           *prometheus.GaugeVec
	GPUPowerDrawMax            *prometheus.GaugeVec
	GPUTemperatureWin          *prometheus.GaugeVec
	GPUProfSMMax               *prometheus.GaugeVec
	GPUProfDRAMMax             *prometheus.GaugeVec
	GPUProfTensorMax           *prometheus.GaugeVec
	GPUUtilizationAvg          *prometheus.GaugeVec
	GPUMemoryCopyAvg           *prometheus.GaugeVec
	GPUMemoryUsedAvg           *prometheus.GaugeVec
	GPUPowerDrawAvg            *prometheus.GaugeVec
	GPUTemperatureAvg          *prometheus.GaugeVec
	GPUProfSMAvg               *prometheus.GaugeVec
	GPUProfDRAMAvg             *prometheus.GaugeVec
	GPUProfTensorAvg           *prometheus.GaugeVec
	GPUThrottleReason          *prometheus.GaugeVec
	GPUClockEventActive        *prometheus.GaugeVec
	GPUProfSMActive            *prometheus.GaugeVec
	GPUProfGraphicsEngineActive *prometheus.GaugeVec
	GPUProfSMOccupancy         *prometheus.GaugeVec
	GPUProfDRAMActive          *prometheus.GaugeVec
	GPUProfTensorPipe          *prometheus.GaugeVec
	GPUProfPipeFP64Active      *prometheus.GaugeVec
	GPUProfPipeFP32Active      *prometheus.GaugeVec
	GPUProfPipeFP16Active      *prometheus.GaugeVec
	GPUProfPipeINTActive       *prometheus.GaugeVec
	GPUProfTensorHMMAActive    *prometheus.GaugeVec
	GPUProfTensorIMMAActive    *prometheus.GaugeVec
	GPUProfTensorDFMAActive    *prometheus.GaugeVec
	GPUDriverVersion           *prometheus.GaugeVec
	GPUCudaVersion             *prometheus.GaugeVec
	GPURequestCount            *prometheus.CounterVec
	GPUActivityWindows         *prometheus.CounterVec

	ExporterUp                   *prometheus.GaugeVec
	ExporterCollectSuccess       *prometheus.GaugeVec
	ExporterLastSuccessTimestamp *prometheus.GaugeVec
	ExporterCollectionDuration   *prometheus.GaugeVec
	ExporterCollectionErrors     *prometheus.CounterVec
	ExporterDiscoveredGPUs       *prometheus.GaugeVec
	GPUActiveSeconds              *prometheus.CounterVec
	GPUUtilizationWeightedSeconds *prometheus.CounterVec
	GPUSMActiveWeightedSeconds    *prometheus.CounterVec
	GPUDRAMActiveWeightedSeconds  *prometheus.CounterVec
	GPUTensorWeightedSeconds      *prometheus.CounterVec
	GPUEnergyJoules               *prometheus.CounterVec
	GPUEnergyEstimatedJoules      *prometheus.CounterVec
	GPUPCIeReplayTotal            *prometheus.CounterVec
	GPUNVLinkErrors               *prometheus.CounterVec
	GPUECCErrors                  *prometheus.CounterVec
	GPURetiredPages               *prometheus.CounterVec
	GPURetiredPagesPending        *prometheus.GaugeVec
	GPURemappedRows               *prometheus.CounterVec
	GPURowRemapFailure            *prometheus.GaugeVec
	GPURowRemapPending            *prometheus.GaugeVec
	GPUClockViolationSeconds      *prometheus.CounterVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		GPUUtilization: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_utilization_percent",
			Help: "Peak GPU utilization percentage between Prometheus scrapes.",
		}, gpuLabels()),
		GPUUtilizationCurrent: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_utilization_percent_current",
			Help: "Current DCGM_FI_DEV_GPU_UTIL GPU utilization percentage.",
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
		GPUMemoryReserved: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_framebuffer_memory_reserved_bytes",
			Help: "Reserved GPU framebuffer memory in bytes.",
		}, gpuLabels()),
		GPUBAR1MemoryFree: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_bar1_memory_free_bytes",
			Help: "Free GPU BAR1 memory in bytes.",
		}, gpuLabels()),
		GPUBAR1MemoryUsed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_bar1_memory_used_bytes",
			Help: "Used GPU BAR1 memory in bytes.",
		}, gpuLabels()),
		GPUBAR1MemoryTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_bar1_memory_total_bytes",
			Help: "Total GPU BAR1 memory in bytes.",
		}, gpuLabels()),
		GPUMemoryTemperature: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_temperature_celsius",
			Help: "DCGM_FI_DEV_MEMORY_TEMP memory temperature in Celsius.",
		}, gpuLabels()),
		GPUMemoryTemperatureMax: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_temperature_max_celsius",
			Help: "DCGM_FI_DEV_MEM_MAX_OP_TEMP maximum memory temperature in Celsius.",
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
		GPUPowerDrawInstant: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_power_draw_instant_watts",
			Help: "DCGM_FI_DEV_POWER_USAGE_INSTANT instantaneous GPU power draw in watts.",
		}, gpuLabels()),
		GPUPowerLimit: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_power_limit_watts",
			Help: "GPU power management limit in watts when available.",
		}, gpuLabels()),
		GPUPowerEnforcedLimit: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_power_enforced_limit_watts",
			Help: "DCGM_FI_DEV_ENFORCED_POWER_LIMIT enforced power limit in watts.",
		}, gpuLabels()),
		GPUMemoryCopyUtil: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_copy_utilization_percent",
			Help: "DCGM_FI_DEV_MEM_COPY_UTIL memory copy utilization percentage.",
		}, gpuLabels()),
		GPUEncoderUtil: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_encoder_utilization_percent",
			Help: "DCGM_FI_DEV_ENC_UTIL encoder utilization percentage.",
		}, gpuLabels()),
		GPUDecoderUtil: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_decoder_utilization_percent",
			Help: "DCGM_FI_DEV_DEC_UTIL decoder utilization percentage.",
		}, gpuLabels()),
		GPUMemoryUsedPct: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_framebuffer_memory_used_percent",
			Help: "DCGM_FI_DEV_FB_USED_PERCENT framebuffer memory used percentage.",
		}, gpuLabels()),
		GPUSMClockHertz: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_sm_clock_hertz",
			Help: "DCGM_FI_DEV_SM_CLOCK streaming multiprocessor clock in hertz.",
		}, gpuLabels()),
		GPUMemoryClockHertz: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_memory_clock_hertz",
			Help: "DCGM_FI_DEV_MEM_CLOCK memory clock in hertz.",
		}, gpuLabels()),
		GPUPerformanceState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_performance_state",
			Help: "DCGM_FI_DEV_PSTATE current GPU performance state.",
		}, gpuLabels()),
		GPUFanSpeed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_fan_speed_percent",
			Help: "DCGM_FI_DEV_FAN_SPEED fan speed percentage.",
		}, gpuLabels()),
		GPUPCIeTXBytesPerSecond: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_pcie_tx_bytes_per_second",
			Help: "DCGM_FI_DEV_PCIE_TX_THROUGHPUT PCIe transmit throughput in bytes per second.",
		}, gpuLabels()),
		GPUPCIeRXBytesPerSecond: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_pcie_rx_bytes_per_second",
			Help: "DCGM_FI_DEV_PCIE_RX_THROUGHPUT PCIe receive throughput in bytes per second.",
		}, gpuLabels()),
		GPUPCIeTransmitBytesPerSecond: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_pcie_transmit_bytes_per_second",
			Help: "DCGM_FI_PROF_PCIE_TX_BYTES PCIe transmit rate in bytes per second.",
		}, gpuLabels()),
		GPUPCIeReceiveBytesPerSecond: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_pcie_receive_bytes_per_second",
			Help: "DCGM_FI_PROF_PCIE_RX_BYTES PCIe receive rate in bytes per second.",
		}, gpuLabels()),
		GPUPCIeLinkGeneration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_pcie_link_generation",
			Help: "DCGM_FI_DEV_PCIE_LINK_GEN current PCIe link generation.",
		}, gpuLabels()),
		GPUPCIeLinkWidth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_pcie_link_width",
			Help: "DCGM_FI_DEV_PCIE_LINK_WIDTH current PCIe link width.",
		}, gpuLabels()),
		GPUPCIeMaxLinkGeneration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_pcie_max_link_generation",
			Help: "DCGM_FI_DEV_PCIE_MAX_LINK_GEN maximum PCIe link generation.",
		}, gpuLabels()),
		GPUPCIeMaxLinkWidth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_pcie_max_link_width",
			Help: "DCGM_FI_DEV_PCIE_MAX_LINK_WIDTH maximum PCIe link width.",
		}, gpuLabels()),
		GPUNVLinkTXBytesPerSecond: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_nvlink_tx_bytes_per_second",
			Help: "DCGM_FI_DEV_NVLINK_TX_BANDWIDTH_TOTAL NVLink transmit bandwidth in bytes per second.",
		}, gpuLabels()),
		GPUNVLinkRXBytesPerSecond: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_nvlink_rx_bytes_per_second",
			Help: "DCGM_FI_DEV_NVLINK_RX_BANDWIDTH_TOTAL NVLink receive bandwidth in bytes per second.",
		}, gpuLabels()),
		GPUNVLinkTransmitBytesPerSecond: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_nvlink_transmit_bytes_per_second",
			Help: "DCGM_FI_PROF_NVLINK_TX_BYTES NVLink transmit rate in bytes per second.",
		}, gpuLabels()),
		GPUNVLinkReceiveBytesPerSecond: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_nvlink_receive_bytes_per_second",
			Help: "DCGM_FI_PROF_NVLINK_RX_BYTES NVLink receive rate in bytes per second.",
		}, gpuLabels()),
		GPUXIDLastCode: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_xid_last_code",
			Help: "DCGM_FI_DEV_XID_ERRORS last XID error code reported by the driver.",
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
		GPUClockEventActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_clock_event_active",
			Help: "Decoded DCGM clock event reason state, exposed as 0 or 1 by reason.",
		}, gpuLabelsWith("reason")),
		GPUProfGraphicsEngineActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_graphics_engine_active_ratio",
			Help: "DCGM_FI_PROF_GR_ENGINE_ACTIVE ratio of time the graphics engine was active.",
		}, gpuLabels()),
		GPUProfSMActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_sm_active_ratio",
			Help: "DCGM_FI_PROF_SM_ACTIVE ratio of time streaming multiprocessors were active.",
		}, gpuLabels()),
		GPUProfSMOccupancy: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_sm_occupancy_ratio",
			Help: "DCGM_FI_PROF_SM_OCCUPANCY ratio of active warps to theoretical maximum warps.",
		}, gpuLabels()),
		GPUProfDRAMActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_dram_active_ratio",
			Help: "DCGM_FI_PROF_DRAM_ACTIVE ratio of time the device memory interface was active.",
		}, gpuLabels()),
		GPUProfTensorPipe: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_pipe_tensor_active_ratio",
			Help: "DCGM_FI_PROF_PIPE_TENSOR_ACTIVE ratio of time the tensor pipe was active.",
		}, gpuLabels()),
		GPUProfPipeFP64Active: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_pipe_fp64_active_ratio",
			Help: "DCGM_FI_PROF_PIPE_FP64_ACTIVE ratio of time the FP64 pipe was active.",
		}, gpuLabels()),
		GPUProfPipeFP32Active: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_pipe_fp32_active_ratio",
			Help: "DCGM_FI_PROF_PIPE_FP32_ACTIVE ratio of time the FP32 pipe was active.",
		}, gpuLabels()),
		GPUProfPipeFP16Active: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_pipe_fp16_active_ratio",
			Help: "DCGM_FI_PROF_PIPE_FP16_ACTIVE ratio of time the FP16 pipe was active.",
		}, gpuLabels()),
		GPUProfPipeINTActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_pipe_int_active_ratio",
			Help: "DCGM_FI_PROF_PIPE_INT_ACTIVE ratio of time the integer pipe was active.",
		}, gpuLabels()),
		GPUProfTensorHMMAActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_pipe_tensor_hmma_active_ratio",
			Help: "DCGM_FI_PROF_PIPE_TENSOR_HMMA_ACTIVE ratio of time the tensor HMMA pipe was active.",
		}, gpuLabels()),
		GPUProfTensorIMMAActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_pipe_tensor_imma_active_ratio",
			Help: "DCGM_FI_PROF_PIPE_TENSOR_IMMA_ACTIVE ratio of time the tensor IMMA pipe was active.",
		}, gpuLabels()),
		GPUProfTensorDFMAActive: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_prof_pipe_tensor_dfma_active_ratio",
			Help: "DCGM_FI_PROF_PIPE_TENSOR_DFMA_ACTIVE ratio of time the tensor DFMA pipe was active.",
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
			Help: "Deprecated alias for gpu_activity_windows_total.",
		}, gpuLabels()),
		GPUActivityWindows: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_activity_windows_total",
			Help: "Total inferred GPU activity windows detected by utilization threshold crossings.",
		}, gpuLabels()),
		ExporterUp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_exporter_up",
			Help: "Whether the last exporter DCGM collection succeeded.",
		}, exporterLabels()),
		ExporterCollectSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_exporter_collect_success",
			Help: "Whether the last exporter DCGM collection succeeded.",
		}, exporterLabels()),
		ExporterLastSuccessTimestamp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_exporter_last_success_timestamp_seconds",
			Help: "Unix timestamp of the last successful DCGM collection.",
		}, exporterLabels()),
		ExporterCollectionDuration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_exporter_collection_duration_seconds",
			Help: "Duration of the last DCGM collection in seconds.",
		}, exporterLabels()),
		ExporterCollectionErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_exporter_collection_errors_total",
			Help: "Total failed DCGM collection attempts.",
		}, exporterLabels()),
		ExporterDiscoveredGPUs: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_exporter_discovered_gpus",
			Help: "Number of GPUs returned by the last successful DCGM collection.",
		}, exporterLabels()),
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
			Help: "DCGM_FI_DEV_TOTAL_ENERGY_CONSUMPTION hardware energy counter in joules when available.",
		}, gpuLabels()),
		GPUEnergyEstimatedJoules: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_energy_estimated_joules_total",
			Help: "Total GPU energy estimate in joules, computed as power_watts * elapsed_seconds.",
		}, gpuLabels()),
		GPUPCIeReplayTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_pcie_replay_total",
			Help: "DCGM_FI_DEV_PCIE_REPLAY_COUNTER hardware PCIe replay counter.",
		}, gpuLabels()),
		GPUNVLinkErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_nvlink_errors_total",
			Help: "DCGM NVLink error and retry counters by type.",
		}, gpuLabelsWith("type")),
		GPUECCErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_ecc_errors_total",
			Help: "DCGM ECC error counters by correctability, persistence, and location.",
		}, gpuLabelsWith("correctability", "persistence", "location")),
		GPURetiredPages: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_retired_pages_total",
			Help: "DCGM retired page counters by cause.",
		}, gpuLabelsWith("cause")),
		GPURetiredPagesPending: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_retired_pages_pending",
			Help: "DCGM pages pending retirement.",
		}, gpuLabels()),
		GPURemappedRows: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_remapped_rows_total",
			Help: "DCGM remapped row counters by correctability.",
		}, gpuLabelsWith("correctability")),
		GPURowRemapFailure: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_row_remap_failure",
			Help: "Whether DCGM reports a row remapping failure.",
		}, gpuLabels()),
		GPURowRemapPending: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "gpu_row_remap_pending",
			Help: "Whether DCGM reports pending row remapping.",
		}, gpuLabels()),
		GPUClockViolationSeconds: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "gpu_clock_violation_seconds_total",
			Help: "Total DCGM clock or policy violation duration in seconds by reason.",
		}, gpuLabelsWith("reason")),
	}

	reg.MustRegister(
		m.GPUUtilization,
		m.GPUUtilizationCurrent,
		m.GPUMemoryFree,
		m.GPUMemoryUsed,
		m.GPUMemoryTotal,
		m.GPUMemoryReserved,
		m.GPUBAR1MemoryFree,
		m.GPUBAR1MemoryUsed,
		m.GPUBAR1MemoryTotal,
		m.GPUMemoryTemperature,
		m.GPUMemoryTemperatureMax,
		m.GPUTemperature,
		m.GPUTemperatureMax,
		m.GPUPowerDraw,
		m.GPUPowerDrawInstant,
		m.GPUPowerLimit,
		m.GPUPowerEnforcedLimit,
		m.GPUMemoryCopyUtil,
		m.GPUEncoderUtil,
		m.GPUDecoderUtil,
		m.GPUMemoryUsedPct,
		m.GPUSMClockHertz,
		m.GPUMemoryClockHertz,
		m.GPUPerformanceState,
		m.GPUFanSpeed,
		m.GPUPCIeTXBytesPerSecond,
		m.GPUPCIeRXBytesPerSecond,
		m.GPUPCIeTransmitBytesPerSecond,
		m.GPUPCIeReceiveBytesPerSecond,
		m.GPUPCIeLinkGeneration,
		m.GPUPCIeLinkWidth,
		m.GPUPCIeMaxLinkGeneration,
		m.GPUPCIeMaxLinkWidth,
		m.GPUNVLinkTXBytesPerSecond,
		m.GPUNVLinkRXBytesPerSecond,
		m.GPUNVLinkTransmitBytesPerSecond,
		m.GPUNVLinkReceiveBytesPerSecond,
		m.GPUXIDLastCode,
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
		m.GPUClockEventActive,
		m.GPUProfGraphicsEngineActive,
		m.GPUProfSMActive,
		m.GPUProfSMOccupancy,
		m.GPUProfDRAMActive,
		m.GPUProfTensorPipe,
		m.GPUProfPipeFP64Active,
		m.GPUProfPipeFP32Active,
		m.GPUProfPipeFP16Active,
		m.GPUProfPipeINTActive,
		m.GPUProfTensorHMMAActive,
		m.GPUProfTensorIMMAActive,
		m.GPUProfTensorDFMAActive,
		m.GPUDriverVersion,
		m.GPUCudaVersion,
		m.GPURequestCount,
		m.GPUActivityWindows,
		m.ExporterUp,
		m.ExporterCollectSuccess,
		m.ExporterLastSuccessTimestamp,
		m.ExporterCollectionDuration,
		m.ExporterCollectionErrors,
		m.ExporterDiscoveredGPUs,
		m.GPUActiveSeconds,
		m.GPUUtilizationWeightedSeconds,
		m.GPUSMActiveWeightedSeconds,
		m.GPUDRAMActiveWeightedSeconds,
		m.GPUTensorWeightedSeconds,
		m.GPUEnergyJoules,
		m.GPUEnergyEstimatedJoules,
		m.GPUPCIeReplayTotal,
		m.GPUNVLinkErrors,
		m.GPUECCErrors,
		m.GPURetiredPages,
		m.GPURetiredPagesPending,
		m.GPURemappedRows,
		m.GPURowRemapFailure,
		m.GPURowRemapPending,
		m.GPUClockViolationSeconds,
	)

	return m
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
