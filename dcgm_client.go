package main

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NVIDIA/go-dcgm/pkg/dcgm"
)

const (
	watchMaxKeepAge     = 60.0
	watchMaxKeepSamples = int32(2048)
)

type DCGMClient struct {
	cleanup             func()
	mode                string
	fastInterval        time.Duration
	profilingInterval   time.Duration
	operationalInterval time.Duration
	reliabilityInterval time.Duration
	logger              *slog.Logger
	mu                  sync.Mutex
	devices             map[uint]GPUInfo
	watchers            map[uint]*gpuWatcher
}

type gpuWatcher struct {
	groups      []*fieldWatcher
	fieldStates map[dcgm.Short]DCGMFieldStatus
}

type fieldWatcher struct {
	kind         string
	fields       []dcgm.Short
	fieldGroupID dcgm.FieldHandle
	groupID      dcgm.GroupHandle
	interval     time.Duration
	since        time.Time
	lastPoll     time.Time
}

type DCGMFieldStatus struct {
	Supported   bool
	Available   bool
	LastSuccess time.Time
}

type FieldObservation struct {
	FieldID   dcgm.Short
	Timestamp time.Time
	Value     float64
}

type GPUCollectionFailure struct {
	Info   GPUInfo
	Reason string
	Err    error
}

type SampleBatch struct {
	Samples   []GPUSample
	Supported []GPUInfo
	Failures  []GPUCollectionFailure
}

type GPUInfo struct {
	ID               uint
	Index            string
	Name             string
	UUID             string
	PCIBusID         string
	Driver           string
	CUDAVersion      string
	MemoryTotalBytes float64
	PowerLimitWatts  *float64
}

type GPUSample struct {
	Info                        GPUInfo
	Utilization                 *float64
	MemoryCopyUtil              *float64
	EncoderUtil                 *float64
	DecoderUtil                 *float64
	MemoryFreeBytes             *float64
	MemoryUsedBytes             *float64
	MemoryTotalBytes            *float64
	MemoryReservedBytes         *float64
	BAR1FreeBytes               *float64
	BAR1UsedBytes               *float64
	BAR1TotalBytes              *float64
	MemoryUsedPercent           *float64
	Temperature                 *float64
	TemperatureMax              *float64
	MemoryTemperature           *float64
	MemoryTemperatureMax        *float64
	PowerDrawWatts              *float64
	PowerDrawInstantWatts       *float64
	PowerLimitWatts             *float64
	PowerEnforcedLimitWatts     *float64
	TotalEnergyJoules           *float64
	ThrottleReasons             *float64
	SMClockHertz                *float64
	MemoryClockHertz            *float64
	PerformanceState            *float64
	FanSpeedPercent             *float64
	PCIeTXBytesPerSecond        *float64
	PCIeRXBytesPerSecond        *float64
	PCIeReplayCounter           *float64
	PCIeLinkGeneration          *float64
	PCIeLinkWidth               *float64
	PCIeMaxLinkGeneration       *float64
	PCIeMaxLinkWidth            *float64
	XIDLastCode                 *float64
	ECCSBEVolatileTotal         *float64
	ECCDBEVolatileTotal         *float64
	ECCSBEAggregateTotal        *float64
	ECCDBEAggregateTotal        *float64
	RetiredSBEPages             *float64
	RetiredDBEPages             *float64
	RetiredPendingPages         *float64
	CorrectableRemappedRows     *float64
	UncorrectableRemappedRows   *float64
	RowRemapFailure             *float64
	RowRemapPending             *float64
	PowerViolationSeconds       *float64
	ThermalViolationSeconds     *float64
	SyncBoostViolationSeconds   *float64
	BoardLimitViolationSeconds  *float64
	LowUtilViolationSeconds     *float64
	ReliabilityViolationSeconds *float64
	AppClockViolationSeconds    *float64
	BaseClockViolationSeconds   *float64
	ProfGraphicsEngineActive    *float64
	ProfSMActive                *float64
	ProfSMOccupancy             *float64
	ProfDRAMActive              *float64
	ProfTensorActive            *float64
	ProfPCIeTXBytesPerSecond    *float64
	ProfPCIeRXBytesPerSecond    *float64
	ProfNVLinkTXBytesPerSecond  *float64
	ProfNVLinkRXBytesPerSecond  *float64
	ProfPipeFP64Active          *float64
	ProfPipeFP32Active          *float64
	ProfPipeFP16Active          *float64
	ProfPipeINTActive           *float64
	ProfTensorHMMAActive        *float64
	ProfTensorIMMAActive        *float64
	ProfTensorDFMAActive        *float64
	Fields                      map[dcgm.Short]DCGMFieldStatus
	Observations                []FieldObservation
}

func NewDCGMClient(cfg Config, logger *slog.Logger) (*DCGMClient, error) {
	cleanup, err := initDCGM(cfg.DCGMMode)
	if err != nil {
		return nil, err
	}

	return &DCGMClient{
		cleanup:             cleanup,
		mode:                cfg.DCGMMode,
		fastInterval:        durationOrDefault(cfg.ScrapeInterval, defaultScrapeInterval),
		profilingInterval:   durationOrDefault(cfg.ProfilingInterval, defaultProfilingInterval),
		operationalInterval: durationOrDefault(cfg.OperationalInterval, defaultOperationalInterval),
		reliabilityInterval: durationOrDefault(cfg.ReliabilityInterval, defaultReliabilityInterval),
		logger:              logger,
		devices:             make(map[uint]GPUInfo),
		watchers:            make(map[uint]*gpuWatcher),
	}, nil
}

func (c *DCGMClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for gpuID, watcher := range c.watchers {
		c.destroyWatcher(gpuID, watcher)
	}
	c.watchers = make(map[uint]*gpuWatcher)

	if c.cleanup != nil {
		c.cleanup()
	}
}

func (c *DCGMClient) StaticInfo() (driverVersion, cudaVersion string) {
	driverVersion = "unknown"
	cudaVersion = "unknown"

	values, err := dcgm.GetLatestValuesForFields(0, []dcgm.Short{
		dcgm.DCGM_FI_DRIVER_VERSION,
		dcgm.DCGM_FI_CUDA_DRIVER_VERSION,
	})
	if err != nil {
		c.logger.Warn("failed to query DCGM system info", "error", err)
		return driverVersion, cudaVersion
	}

	for _, value := range values {
		switch value.FieldID {
		case dcgm.DCGM_FI_DRIVER_VERSION:
			if parsed := validDCGMStringField(value); parsed != "" {
				driverVersion = parsed
			}
		case dcgm.DCGM_FI_CUDA_DRIVER_VERSION:
			if raw := int64Field(value); raw != nil {
				if parsed := formatCUDAVersion(*raw); parsed != "" {
					cudaVersion = parsed
				}
			}
		}
	}

	return driverVersion, cudaVersion
}

func (c *DCGMClient) Samples() (SampleBatch, error) {
	gpuIDs, err := dcgm.GetSupportedDevices()
	if err != nil {
		return SampleBatch{}, fmt.Errorf("list DCGM-supported GPUs: %w", err)
	}

	batch := SampleBatch{
		Samples:   make([]GPUSample, 0, len(gpuIDs)),
		Supported: make([]GPUInfo, 0, len(gpuIDs)),
	}
	now := time.Now()
	for _, gpuID := range gpuIDs {
		info, err := c.cachedGPUInfo(gpuID)
		if err != nil {
			c.logger.Warn("failed to query GPU identity", "gpu_id", gpuID, "error", err)
			info = GPUInfo{
				ID:          gpuID,
				Index:       fmt.Sprintf("%d", gpuID),
				Name:        "unknown",
				UUID:        "unknown",
				PCIBusID:    "unknown",
				Driver:      "unknown",
				CUDAVersion: "unknown",
			}
			batch.Failures = append(batch.Failures, GPUCollectionFailure{Info: info, Reason: "identity", Err: err})
		}
		batch.Supported = append(batch.Supported, info)

		watcher, err := c.ensureWatcher(gpuID)
		if err != nil {
			c.logger.Warn("failed to prepare DCGM watcher", "gpu_id", gpuID, "error", err)
			batch.Failures = append(batch.Failures, GPUCollectionFailure{Info: info, Reason: "watch", Err: err})
			continue
		}

		sample := GPUSample{
			Info:            info,
			PowerLimitWatts: info.PowerLimitWatts,
			Fields:          make(map[dcgm.Short]DCGMFieldStatus, len(watcher.fieldStates)),
		}
		if info.MemoryTotalBytes > 0 {
			total := info.MemoryTotalBytes
			sample.MemoryTotalBytes = &total
		}

		failedGroups := c.readWatcher(gpuID, watcher, &sample, now)
		fastFailed := false
		for _, failure := range failedGroups {
			batch.Failures = append(batch.Failures, GPUCollectionFailure{Info: info, Reason: failure.kind, Err: failure.err})
			if failure.kind == "fast" {
				fastFailed = true
			}
		}
		for fieldID, status := range watcher.fieldStates {
			sample.Fields[fieldID] = status
		}
		if sample.MemoryFreeBytes == nil && sample.MemoryTotalBytes != nil && sample.MemoryUsedBytes != nil {
			free := max(*sample.MemoryTotalBytes-*sample.MemoryUsedBytes, 0)
			sample.MemoryFreeBytes = &free
		}
		if fastFailed {
			continue
		}
		batch.Samples = append(batch.Samples, sample)
	}

	c.removeMissingGPUs(gpuIDs)
	return batch, nil
}

func durationOrDefault(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func (c *DCGMClient) removeMissingGPUs(current []uint) {
	present := make(map[uint]struct{}, len(current))
	for _, gpuID := range current {
		present[gpuID] = struct{}{}
	}

	type staleWatcher struct {
		gpuID   uint
		watcher *gpuWatcher
	}
	var stale []staleWatcher
	c.mu.Lock()
	for gpuID, watcher := range c.watchers {
		if _, ok := present[gpuID]; ok {
			continue
		}
		stale = append(stale, staleWatcher{gpuID: gpuID, watcher: watcher})
		delete(c.watchers, gpuID)
		delete(c.devices, gpuID)
	}
	c.mu.Unlock()

	for _, item := range stale {
		c.destroyWatcher(item.gpuID, item.watcher)
	}
}

func (c *DCGMClient) cachedGPUInfo(gpuID uint) (GPUInfo, error) {
	c.mu.Lock()
	info, ok := c.devices[gpuID]
	c.mu.Unlock()
	if ok {
		return info, nil
	}

	info, err := c.gpuInfo(gpuID)
	if err != nil {
		return GPUInfo{}, err
	}

	c.mu.Lock()
	c.devices[gpuID] = info
	c.mu.Unlock()

	return info, nil
}

func (c *DCGMClient) gpuInfo(gpuID uint) (GPUInfo, error) {
	device, err := dcgm.GetDeviceInfo(gpuID)
	if err != nil {
		return GPUInfo{}, fmt.Errorf("get device info for GPU %d: %w", gpuID, err)
	}

	index := fmt.Sprintf("%d", gpuID)
	values, err := dcgm.GetLatestValuesForFields(gpuID, []dcgm.Short{dcgm.DCGM_FI_DEV_NVML_INDEX})
	if err != nil {
		c.logger.Debug("failed to query NVML index field", "gpu_id", gpuID, "error", err)
	} else {
		for _, value := range values {
			if value.FieldID == dcgm.DCGM_FI_DEV_NVML_INDEX {
				if parsed := int64Field(value); parsed != nil {
					index = fmt.Sprintf("%d", *parsed)
				}
			}
		}
	}

	info := GPUInfo{
		ID:               gpuID,
		Index:            index,
		Name:             firstNonEmpty(device.Identifiers.Model, device.Identifiers.Brand, "unknown"),
		UUID:             firstNonEmpty(device.UUID, "unknown"),
		PCIBusID:         firstNonEmpty(device.PCI.BusID, "unknown"),
		Driver:           firstNonEmpty(device.Identifiers.DriverVersion, "unknown"),
		CUDAVersion:      "unknown",
		MemoryTotalBytes: float64(device.PCI.FBTotal) * 1024 * 1024,
	}

	if device.Power > 0 {
		powerLimit := float64(device.Power)
		info.PowerLimitWatts = &powerLimit
	}

	return info, nil
}

func fastRequiredFields() []dcgm.Short {
	return []dcgm.Short{
		dcgm.DCGM_FI_DEV_GPU_UTIL,
	}
}

func fastOptionalFields() []dcgm.Short {
	return []dcgm.Short{
		dcgm.DCGM_FI_DEV_MEM_COPY_UTIL,
		dcgm.DCGM_FI_DEV_POWER_USAGE,
		dcgm.DCGM_FI_DEV_FB_USED_PERCENT,
		dcgm.DCGM_FI_DEV_ENC_UTIL,
		dcgm.DCGM_FI_DEV_DEC_UTIL,
	}
}

func operationalFields() []dcgm.Short {
	return []dcgm.Short{
		dcgm.DCGM_FI_DEV_SM_CLOCK,
		dcgm.DCGM_FI_DEV_MEM_CLOCK,
		dcgm.DCGM_FI_DEV_GPU_TEMP,
		dcgm.DCGM_FI_DEV_GPU_MAX_OP_TEMP,
		dcgm.DCGM_FI_DEV_MEMORY_TEMP,
		dcgm.DCGM_FI_DEV_MEM_MAX_OP_TEMP,
		dcgm.DCGM_FI_DEV_FB_FREE,
		dcgm.DCGM_FI_DEV_FB_USED,
		dcgm.DCGM_FI_DEV_FB_TOTAL,
		dcgm.DCGM_FI_DEV_FB_RESERVED,
		dcgm.DCGM_FI_DEV_PSTATE,
		dcgm.DCGM_FI_DEV_FAN_SPEED,
		dcgm.DCGM_FI_DEV_BAR1_FREE,
		dcgm.DCGM_FI_DEV_BAR1_USED,
		dcgm.DCGM_FI_DEV_BAR1_TOTAL,
		dcgm.DCGM_FI_DEV_TOTAL_ENERGY_CONSUMPTION,
		dcgm.DCGM_FI_DEV_POWER_USAGE_INSTANT,
		dcgm.DCGM_FI_DEV_POWER_MGMT_LIMIT,
		dcgm.DCGM_FI_DEV_ENFORCED_POWER_LIMIT,
		dcgm.DCGM_FI_DEV_PCIE_LINK_GEN,
		dcgm.DCGM_FI_DEV_PCIE_LINK_WIDTH,
		dcgm.DCGM_FI_DEV_PCIE_MAX_LINK_GEN,
		dcgm.DCGM_FI_DEV_PCIE_MAX_LINK_WIDTH,
		dcgm.DCGM_FI_DEV_CLOCK_THROTTLE_REASONS,
	}
}

func legacyRateFields() []dcgm.Short {
	return []dcgm.Short{
		dcgm.DCGM_FI_DEV_PCIE_TX_THROUGHPUT,
		dcgm.DCGM_FI_DEV_PCIE_RX_THROUGHPUT,
	}
}

func reliabilityFields() []dcgm.Short {
	return []dcgm.Short{
		dcgm.DCGM_FI_DEV_PCIE_REPLAY_COUNTER,
		dcgm.DCGM_FI_DEV_POWER_VIOLATION,
		dcgm.DCGM_FI_DEV_THERMAL_VIOLATION,
		dcgm.DCGM_FI_DEV_SYNC_BOOST_VIOLATION,
		dcgm.DCGM_FI_DEV_BOARD_LIMIT_VIOLATION,
		dcgm.DCGM_FI_DEV_LOW_UTIL_VIOLATION,
		dcgm.DCGM_FI_DEV_RELIABILITY_VIOLATION,
		dcgm.DCGM_FI_DEV_TOTAL_APP_CLOCKS_VIOLATION,
		dcgm.DCGM_FI_DEV_TOTAL_BASE_CLOCKS_VIOLATION,
		dcgm.DCGM_FI_DEV_XID_ERRORS,
		dcgm.DCGM_FI_DEV_ECC_SBE_VOL_TOTAL,
		dcgm.DCGM_FI_DEV_ECC_DBE_VOL_TOTAL,
		dcgm.DCGM_FI_DEV_ECC_SBE_AGG_TOTAL,
		dcgm.DCGM_FI_DEV_ECC_DBE_AGG_TOTAL,
		dcgm.DCGM_FI_DEV_RETIRED_SBE,
		dcgm.DCGM_FI_DEV_RETIRED_DBE,
		dcgm.DCGM_FI_DEV_RETIRED_PENDING,
		dcgm.DCGM_FI_DEV_CORRECTABLE_REMAPPED_ROWS,
		dcgm.DCGM_FI_DEV_UNCORRECTABLE_REMAPPED_ROWS,
		dcgm.DCGM_FI_DEV_ROW_REMAP_FAILURE,
		dcgm.DCGM_FI_DEV_ROW_REMAP_PENDING,
	}
}

func monitoredFields() []dcgm.Short {
	fields := appendFields(nil, fastRequiredFields()...)
	fields = appendFields(fields, fastOptionalFields()...)
	fields = appendFields(fields, operationalFields()...)
	fields = appendFields(fields, legacyRateFields()...)
	fields = appendFields(fields, reliabilityFields()...)
	return appendFields(fields, profFields()...)
}

var dcgmFieldLabels = map[dcgm.Short]string{
	dcgm.DCGM_FI_DEV_GPU_UTIL:                    "DCGM_FI_DEV_GPU_UTIL",
	dcgm.DCGM_FI_DEV_MEM_COPY_UTIL:               "DCGM_FI_DEV_MEM_COPY_UTIL",
	dcgm.DCGM_FI_DEV_POWER_USAGE:                 "DCGM_FI_DEV_POWER_USAGE",
	dcgm.DCGM_FI_DEV_FB_USED_PERCENT:             "DCGM_FI_DEV_FB_USED_PERCENT",
	dcgm.DCGM_FI_DEV_ENC_UTIL:                    "DCGM_FI_DEV_ENC_UTIL",
	dcgm.DCGM_FI_DEV_DEC_UTIL:                    "DCGM_FI_DEV_DEC_UTIL",
	dcgm.DCGM_FI_DEV_SM_CLOCK:                    "DCGM_FI_DEV_SM_CLOCK",
	dcgm.DCGM_FI_DEV_MEM_CLOCK:                   "DCGM_FI_DEV_MEM_CLOCK",
	dcgm.DCGM_FI_DEV_GPU_TEMP:                    "DCGM_FI_DEV_GPU_TEMP",
	dcgm.DCGM_FI_DEV_GPU_MAX_OP_TEMP:             "DCGM_FI_DEV_GPU_MAX_OP_TEMP",
	dcgm.DCGM_FI_DEV_MEMORY_TEMP:                 "DCGM_FI_DEV_MEMORY_TEMP",
	dcgm.DCGM_FI_DEV_MEM_MAX_OP_TEMP:             "DCGM_FI_DEV_MEM_MAX_OP_TEMP",
	dcgm.DCGM_FI_DEV_FB_FREE:                     "DCGM_FI_DEV_FB_FREE",
	dcgm.DCGM_FI_DEV_FB_USED:                     "DCGM_FI_DEV_FB_USED",
	dcgm.DCGM_FI_DEV_FB_TOTAL:                    "DCGM_FI_DEV_FB_TOTAL",
	dcgm.DCGM_FI_DEV_FB_RESERVED:                 "DCGM_FI_DEV_FB_RESERVED",
	dcgm.DCGM_FI_DEV_PSTATE:                      "DCGM_FI_DEV_PSTATE",
	dcgm.DCGM_FI_DEV_FAN_SPEED:                   "DCGM_FI_DEV_FAN_SPEED",
	dcgm.DCGM_FI_DEV_BAR1_FREE:                   "DCGM_FI_DEV_BAR1_FREE",
	dcgm.DCGM_FI_DEV_BAR1_USED:                   "DCGM_FI_DEV_BAR1_USED",
	dcgm.DCGM_FI_DEV_BAR1_TOTAL:                  "DCGM_FI_DEV_BAR1_TOTAL",
	dcgm.DCGM_FI_DEV_TOTAL_ENERGY_CONSUMPTION:    "DCGM_FI_DEV_TOTAL_ENERGY_CONSUMPTION",
	dcgm.DCGM_FI_DEV_POWER_USAGE_INSTANT:         "DCGM_FI_DEV_POWER_USAGE_INSTANT",
	dcgm.DCGM_FI_DEV_POWER_MGMT_LIMIT:            "DCGM_FI_DEV_POWER_MGMT_LIMIT",
	dcgm.DCGM_FI_DEV_ENFORCED_POWER_LIMIT:        "DCGM_FI_DEV_ENFORCED_POWER_LIMIT",
	dcgm.DCGM_FI_DEV_PCIE_TX_THROUGHPUT:          "DCGM_FI_DEV_PCIE_TX_THROUGHPUT",
	dcgm.DCGM_FI_DEV_PCIE_RX_THROUGHPUT:          "DCGM_FI_DEV_PCIE_RX_THROUGHPUT",
	dcgm.DCGM_FI_DEV_PCIE_REPLAY_COUNTER:         "DCGM_FI_DEV_PCIE_REPLAY_COUNTER",
	dcgm.DCGM_FI_DEV_PCIE_LINK_GEN:               "DCGM_FI_DEV_PCIE_LINK_GEN",
	dcgm.DCGM_FI_DEV_PCIE_LINK_WIDTH:             "DCGM_FI_DEV_PCIE_LINK_WIDTH",
	dcgm.DCGM_FI_DEV_PCIE_MAX_LINK_GEN:           "DCGM_FI_DEV_PCIE_MAX_LINK_GEN",
	dcgm.DCGM_FI_DEV_PCIE_MAX_LINK_WIDTH:         "DCGM_FI_DEV_PCIE_MAX_LINK_WIDTH",
	dcgm.DCGM_FI_DEV_CLOCK_THROTTLE_REASONS:      "DCGM_FI_DEV_CLOCK_THROTTLE_REASONS",
	dcgm.DCGM_FI_DEV_POWER_VIOLATION:             "DCGM_FI_DEV_POWER_VIOLATION",
	dcgm.DCGM_FI_DEV_THERMAL_VIOLATION:           "DCGM_FI_DEV_THERMAL_VIOLATION",
	dcgm.DCGM_FI_DEV_SYNC_BOOST_VIOLATION:        "DCGM_FI_DEV_SYNC_BOOST_VIOLATION",
	dcgm.DCGM_FI_DEV_BOARD_LIMIT_VIOLATION:       "DCGM_FI_DEV_BOARD_LIMIT_VIOLATION",
	dcgm.DCGM_FI_DEV_LOW_UTIL_VIOLATION:          "DCGM_FI_DEV_LOW_UTIL_VIOLATION",
	dcgm.DCGM_FI_DEV_RELIABILITY_VIOLATION:       "DCGM_FI_DEV_RELIABILITY_VIOLATION",
	dcgm.DCGM_FI_DEV_TOTAL_APP_CLOCKS_VIOLATION:  "DCGM_FI_DEV_TOTAL_APP_CLOCKS_VIOLATION",
	dcgm.DCGM_FI_DEV_TOTAL_BASE_CLOCKS_VIOLATION: "DCGM_FI_DEV_TOTAL_BASE_CLOCKS_VIOLATION",
	dcgm.DCGM_FI_DEV_XID_ERRORS:                  "DCGM_FI_DEV_XID_ERRORS",
	dcgm.DCGM_FI_DEV_ECC_SBE_VOL_TOTAL:           "DCGM_FI_DEV_ECC_SBE_VOL_TOTAL",
	dcgm.DCGM_FI_DEV_ECC_DBE_VOL_TOTAL:           "DCGM_FI_DEV_ECC_DBE_VOL_TOTAL",
	dcgm.DCGM_FI_DEV_ECC_SBE_AGG_TOTAL:           "DCGM_FI_DEV_ECC_SBE_AGG_TOTAL",
	dcgm.DCGM_FI_DEV_ECC_DBE_AGG_TOTAL:           "DCGM_FI_DEV_ECC_DBE_AGG_TOTAL",
	dcgm.DCGM_FI_DEV_RETIRED_SBE:                 "DCGM_FI_DEV_RETIRED_SBE",
	dcgm.DCGM_FI_DEV_RETIRED_DBE:                 "DCGM_FI_DEV_RETIRED_DBE",
	dcgm.DCGM_FI_DEV_RETIRED_PENDING:             "DCGM_FI_DEV_RETIRED_PENDING",
	dcgm.DCGM_FI_DEV_CORRECTABLE_REMAPPED_ROWS:   "DCGM_FI_DEV_CORRECTABLE_REMAPPED_ROWS",
	dcgm.DCGM_FI_DEV_UNCORRECTABLE_REMAPPED_ROWS: "DCGM_FI_DEV_UNCORRECTABLE_REMAPPED_ROWS",
	dcgm.DCGM_FI_DEV_ROW_REMAP_FAILURE:           "DCGM_FI_DEV_ROW_REMAP_FAILURE",
	dcgm.DCGM_FI_DEV_ROW_REMAP_PENDING:           "DCGM_FI_DEV_ROW_REMAP_PENDING",
	dcgm.DCGM_FI_PROF_GR_ENGINE_ACTIVE:           "DCGM_FI_PROF_GR_ENGINE_ACTIVE",
	dcgm.DCGM_FI_PROF_SM_ACTIVE:                  "DCGM_FI_PROF_SM_ACTIVE",
	dcgm.DCGM_FI_PROF_SM_OCCUPANCY:               "DCGM_FI_PROF_SM_OCCUPANCY",
	dcgm.DCGM_FI_PROF_DRAM_ACTIVE:                "DCGM_FI_PROF_DRAM_ACTIVE",
	dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE:         "DCGM_FI_PROF_PIPE_TENSOR_ACTIVE",
	dcgm.DCGM_FI_PROF_PCIE_TX_BYTES:              "DCGM_FI_PROF_PCIE_TX_BYTES",
	dcgm.DCGM_FI_PROF_PCIE_RX_BYTES:              "DCGM_FI_PROF_PCIE_RX_BYTES",
	dcgm.DCGM_FI_PROF_NVLINK_TX_BYTES:            "DCGM_FI_PROF_NVLINK_TX_BYTES",
	dcgm.DCGM_FI_PROF_NVLINK_RX_BYTES:            "DCGM_FI_PROF_NVLINK_RX_BYTES",
	dcgm.DCGM_FI_PROF_PIPE_FP64_ACTIVE:           "DCGM_FI_PROF_PIPE_FP64_ACTIVE",
	dcgm.DCGM_FI_PROF_PIPE_FP32_ACTIVE:           "DCGM_FI_PROF_PIPE_FP32_ACTIVE",
	dcgm.DCGM_FI_PROF_PIPE_FP16_ACTIVE:           "DCGM_FI_PROF_PIPE_FP16_ACTIVE",
	dcgm.DCGM_FI_PROF_PIPE_INT_ACTIVE:            "DCGM_FI_PROF_PIPE_INT_ACTIVE",
	dcgm.DCGM_FI_PROF_PIPE_TENSOR_HMMA_ACTIVE:    "DCGM_FI_PROF_PIPE_TENSOR_HMMA_ACTIVE",
	dcgm.DCGM_FI_PROF_PIPE_TENSOR_IMMA_ACTIVE:    "DCGM_FI_PROF_PIPE_TENSOR_IMMA_ACTIVE",
	dcgm.DCGM_FI_PROF_PIPE_TENSOR_DFMA_ACTIVE:    "DCGM_FI_PROF_PIPE_TENSOR_DFMA_ACTIVE",
}

func fieldLabel(fieldID dcgm.Short) string {
	if label, ok := dcgmFieldLabels[fieldID]; ok {
		return label
	}
	return fmt.Sprintf("DCGM_FIELD_%d", fieldID)
}

func profFields() []dcgm.Short {
	return []dcgm.Short{
		dcgm.DCGM_FI_PROF_GR_ENGINE_ACTIVE,
		dcgm.DCGM_FI_PROF_SM_ACTIVE,
		dcgm.DCGM_FI_PROF_SM_OCCUPANCY,
		dcgm.DCGM_FI_PROF_DRAM_ACTIVE,
		dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE,
		dcgm.DCGM_FI_PROF_PCIE_TX_BYTES,
		dcgm.DCGM_FI_PROF_PCIE_RX_BYTES,
		dcgm.DCGM_FI_PROF_NVLINK_TX_BYTES,
		dcgm.DCGM_FI_PROF_NVLINK_RX_BYTES,
		dcgm.DCGM_FI_PROF_PIPE_FP64_ACTIVE,
		dcgm.DCGM_FI_PROF_PIPE_FP32_ACTIVE,
		dcgm.DCGM_FI_PROF_PIPE_FP16_ACTIVE,
		dcgm.DCGM_FI_PROF_PIPE_INT_ACTIVE,
		dcgm.DCGM_FI_PROF_PIPE_TENSOR_HMMA_ACTIVE,
		dcgm.DCGM_FI_PROF_PIPE_TENSOR_IMMA_ACTIVE,
		dcgm.DCGM_FI_PROF_PIPE_TENSOR_DFMA_ACTIVE,
	}
}

func (c *DCGMClient) ensureWatcher(gpuID uint) (*gpuWatcher, error) {
	c.mu.Lock()
	if watcher, ok := c.watchers[gpuID]; ok {
		c.mu.Unlock()
		return watcher, nil
	}
	c.mu.Unlock()

	watcher := &gpuWatcher{fieldStates: make(map[dcgm.Short]DCGMFieldStatus, len(monitoredFields()))}
	for _, fieldID := range monitoredFields() {
		watcher.fieldStates[fieldID] = DCGMFieldStatus{}
	}

	fast := c.watchableOptionalFields(gpuID, "fast", fastRequiredFields(), fastOptionalFields(), c.fastInterval)
	fastWatcher, err := c.createWatcher(gpuID, "fast", fast, c.fastInterval)
	if err != nil {
		return nil, err
	}
	watcher.groups = append(watcher.groups, fastWatcher)

	addOptionalGroup := func(kind string, candidates []dcgm.Short, interval time.Duration) {
		fields := c.watchableOptionalFields(gpuID, kind, nil, candidates, interval)
		if len(fields) == 0 {
			return
		}
		group, groupErr := c.createWatcher(gpuID, kind, fields, interval)
		if groupErr != nil {
			c.logger.Warn("DCGM watcher group unavailable", "gpu_id", gpuID, "kind", kind, "error", groupErr)
			return
		}
		watcher.groups = append(watcher.groups, group)
	}

	addOptionalGroup("operational", operationalFields(), c.operationalInterval)
	addOptionalGroup("profiling", legacyRateFields(), c.profilingInterval)
	addOptionalGroup("reliability", reliabilityFields(), c.reliabilityInterval)

	if prof := c.supportedProfilingFields(gpuID); len(prof) > 0 {
		group, groupErr := c.createWatcher(gpuID, "profiling", prof, c.profilingInterval)
		if groupErr != nil {
			c.logger.Warn("DCGM profiling fields unavailable, continuing without DCP fields", "gpu_id", gpuID, "error", groupErr)
		} else {
			watcher.groups = append(watcher.groups, group)
		}
	}

	for _, group := range watcher.groups {
		for _, fieldID := range group.fields {
			status := watcher.fieldStates[fieldID]
			status.Supported = true
			watcher.fieldStates[fieldID] = status
		}
	}

	c.mu.Lock()
	c.watchers[gpuID] = watcher
	c.mu.Unlock()

	return watcher, nil
}

func (c *DCGMClient) watchableOptionalFields(gpuID uint, kind string, required, candidates []dcgm.Short, interval time.Duration) []dcgm.Short {
	fields := appendFields(nil, required...)
	var addBatch func([]dcgm.Short)
	addBatch = func(batch []dcgm.Short) {
		if len(batch) == 0 {
			return
		}
		trialFields := appendFields(fields, batch...)
		probe, err := c.createWatcher(gpuID, kind+"-probe", trialFields, interval)
		if err == nil {
			c.destroyFieldWatcher(gpuID, probe)
			fields = trialFields
			return
		}
		if len(batch) == 1 {
			c.logger.Debug("skipping unsupported optional DCGM field", "gpu_id", gpuID, "field_id", batch[0], "error", err)
			return
		}
		middle := len(batch) / 2
		addBatch(batch[:middle])
		addBatch(batch[middle:])
	}
	addBatch(candidates)
	return fields
}

func (c *DCGMClient) supportedProfilingFields(gpuID uint) []dcgm.Short {
	requested := profFields()
	groups, err := dcgm.GetSupportedMetricGroups(gpuID)
	if err != nil {
		c.logger.Debug("failed to query supported DCGM profiling metric groups", "gpu_id", gpuID, "error", err)
		return nil
	}

	best := selectBestProfilingFields(requested, groups)

	if len(best) > 0 && len(best) < len(requested) {
		c.logger.Info("using partial DCGM profiling field set", "gpu_id", gpuID, "requested", len(requested), "selected", len(best))
	}
	return best
}

func selectBestProfilingFields(requested []dcgm.Short, groups []dcgm.MetricGroup) []dcgm.Short {
	var best []dcgm.Short
	for _, group := range groups {
		fields := requestedFieldsInGroup(requested, group)
		if len(fields) > len(best) {
			best = fields
		}
	}
	return best
}

func requestedFieldsInGroup(requested []dcgm.Short, group dcgm.MetricGroup) []dcgm.Short {
	supported := make(map[dcgm.Short]struct{}, len(group.FieldIds))
	for _, fieldID := range group.FieldIds {
		supported[dcgm.Short(fieldID)] = struct{}{}
	}

	fields := make([]dcgm.Short, 0, len(requested))
	for _, fieldID := range requested {
		if _, ok := supported[fieldID]; ok {
			fields = append(fields, fieldID)
		}
	}
	return fields
}

func appendFields(fields []dcgm.Short, extra ...dcgm.Short) []dcgm.Short {
	result := make([]dcgm.Short, 0, len(fields)+len(extra))
	result = append(result, fields...)
	for _, fieldID := range extra {
		exists := false
		for _, existing := range result {
			if existing == fieldID {
				exists = true
				break
			}
		}
		if !exists {
			result = append(result, fieldID)
		}
	}
	return result
}

func (c *DCGMClient) destroyWatcher(gpuID uint, watcher *gpuWatcher) {
	if watcher == nil {
		return
	}
	for _, group := range watcher.groups {
		c.destroyFieldWatcher(gpuID, group)
	}
}

func (c *DCGMClient) destroyFieldWatcher(gpuID uint, watcher *fieldWatcher) {
	if watcher == nil {
		return
	}
	if err := dcgm.DestroyGroup(watcher.groupID); err != nil {
		c.logger.Debug("failed to destroy DCGM watcher group", "gpu_id", gpuID, "error", err)
	}
	if err := dcgm.FieldGroupDestroy(watcher.fieldGroupID); err != nil {
		c.logger.Debug("failed to destroy DCGM watcher field group", "gpu_id", gpuID, "error", err)
	}
}

// createWatcher создает DCGM field group и подписку с собственной частотой.
func (c *DCGMClient) createWatcher(gpuID uint, kind string, fields []dcgm.Short, interval time.Duration) (*fieldWatcher, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("create %s DCGM watcher: empty field set", kind)
	}
	fieldGroupName := fmt.Sprintf("gpuExporterFields%d", rand.Uint64())
	fieldGroupID, err := dcgm.FieldGroupCreate(fieldGroupName, fields)
	if err != nil {
		return nil, fmt.Errorf("create DCGM field group: %w", err)
	}

	groupName := fmt.Sprintf("gpuExporterGroup%d", rand.Uint64())
	groupID, err := dcgm.CreateGroup(groupName)
	if err != nil {
		_ = dcgm.FieldGroupDestroy(fieldGroupID)
		return nil, fmt.Errorf("create DCGM device group: %w", err)
	}

	destroy := func() {
		_ = dcgm.DestroyGroup(groupID)
		_ = dcgm.FieldGroupDestroy(fieldGroupID)
	}

	if err := dcgm.AddToGroup(groupID, gpuID); err != nil {
		destroy()
		return nil, fmt.Errorf("add GPU %d to DCGM group: %w", gpuID, err)
	}

	err = dcgm.WatchFieldsWithGroupEx(fieldGroupID, groupID,
		interval.Microseconds(), watchMaxKeepAge, watchMaxKeepSamples)
	if err != nil {
		destroy()
		return nil, fmt.Errorf("watch DCGM fields: %w", err)
	}

	return &fieldWatcher{
		kind:         kind,
		fields:       fields,
		fieldGroupID: fieldGroupID,
		groupID:      groupID,
		interval:     interval,
		since:        time.Now().Add(-interval),
	}, nil
}

type watcherFailure struct {
	kind string
	err  error
}

// readWatcher читает все DCGM-сэмплы со времени предыдущего чтения.
// Группы опрашиваются только с их собственной configured frequency.
func (c *DCGMClient) readWatcher(gpuID uint, watcher *gpuWatcher, sample *GPUSample, now time.Time) []watcherFailure {
	var failures []watcherFailure
	for _, group := range watcher.groups {
		if !group.lastPoll.IsZero() && now.Sub(group.lastPoll) < group.interval {
			continue
		}
		group.lastPoll = now

		values, next, err := dcgm.GetValuesSince(group.groupID, group.fieldGroupID, group.since)
		if err != nil {
			for _, fieldID := range group.fields {
				status := watcher.fieldStates[fieldID]
				status.Available = false
				watcher.fieldStates[fieldID] = status
			}
			failures = append(failures, watcherFailure{kind: group.kind, err: err})
			continue
		}
		if !next.IsZero() {
			group.since = next
		}

		sort.SliceStable(values, func(i, j int) bool { return values[i].TS < values[j].TS })
		seen := make(map[dcgm.Short]bool, len(group.fields))
		for _, raw := range values {
			if raw.EntityID != gpuID {
				continue
			}
			value := dcgm.FieldValue_v1{
				Version: raw.Version, FieldID: raw.FieldID, FieldType: raw.FieldType,
				Status: raw.Status, TS: raw.TS, Value: raw.Value,
			}
			seen[value.FieldID] = true
			parsed, ok := applyFieldValue(value, sample)
			status := watcher.fieldStates[value.FieldID]
			status.Available = ok
			if ok {
				timestamp := time.UnixMicro(value.TS)
				status.LastSuccess = timestamp
				sample.Observations = append(sample.Observations, FieldObservation{FieldID: value.FieldID, Timestamp: timestamp, Value: parsed})
			}
			watcher.fieldStates[value.FieldID] = status
		}

		for _, fieldID := range group.fields {
			if seen[fieldID] {
				continue
			}
			status := watcher.fieldStates[fieldID]
			if status.LastSuccess.IsZero() || now.Sub(status.LastSuccess) > 2*group.interval {
				status.Available = false
				watcher.fieldStates[fieldID] = status
			}
		}
	}
	return failures
}

// applyFieldValues декодирует искусственные или уже полученные v1-значения;
// функция оставлена отдельно для table-driven regression tests.
func applyFieldValues(values []dcgm.FieldValue_v1, sample *GPUSample) []FieldObservation {
	sort.SliceStable(values, func(i, j int) bool { return values[i].TS < values[j].TS })
	observations := make([]FieldObservation, 0, len(values))
	for _, value := range values {
		if parsed, ok := applyFieldValue(value, sample); ok {
			observations = append(observations, FieldObservation{FieldID: value.FieldID, Timestamp: time.UnixMicro(value.TS), Value: parsed})
		}
	}
	return observations
}

func applyFieldValue(value dcgm.FieldValue_v1, sample *GPUSample) (float64, bool) {
	if !fieldSuccessful(value) {
		return 0, false
	}

	target := sampleFieldTarget(sample, value.FieldID)
	if target == nil {
		return 0, false
	}
	*target = nil

	switch value.FieldID {
	case dcgm.DCGM_FI_DEV_GPU_UTIL:
		if parsed := int64Field(value); parsed != nil {
			sample.Utilization = percentPointer(*parsed)
		}
	case dcgm.DCGM_FI_DEV_MEM_COPY_UTIL:
		if parsed := int64Field(value); parsed != nil {
			sample.MemoryCopyUtil = percentPointer(*parsed)
		}
	case dcgm.DCGM_FI_DEV_ENC_UTIL:
		sample.EncoderUtil = percentNumberPointer(value)
	case dcgm.DCGM_FI_DEV_DEC_UTIL:
		sample.DecoderUtil = percentNumberPointer(value)
	case dcgm.DCGM_FI_DEV_POWER_USAGE:
		if parsed := float64Field(value); parsed != nil {
			sample.PowerDrawWatts = powerDrawPointer(*parsed)
		}
	case dcgm.DCGM_FI_DEV_POWER_USAGE_INSTANT:
		sample.PowerDrawInstantWatts = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_POWER_MGMT_LIMIT:
		if parsed := float64Field(value); parsed != nil {
			if limit := wattsPointer(*parsed); limit != nil {
				sample.PowerLimitWatts = limit
			}
		} else if parsed := int64Field(value); parsed != nil && *parsed < 1000000 {
			limit := float64(*parsed)
			sample.PowerLimitWatts = &limit
		}
	case dcgm.DCGM_FI_DEV_ENFORCED_POWER_LIMIT:
		sample.PowerEnforcedLimitWatts = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_TOTAL_ENERGY_CONSUMPTION:
		sample.TotalEnergyJoules = milliJoulesToJoulesPointer(value)
	case dcgm.DCGM_FI_DEV_GPU_TEMP:
		if parsed := int64Field(value); parsed != nil {
			sample.Temperature = temperaturePointer(*parsed)
		}
	case dcgm.DCGM_FI_DEV_GPU_MAX_OP_TEMP:
		if parsed := int64Field(value); parsed != nil {
			tempMax := float64(*parsed)
			sample.TemperatureMax = &tempMax
		}
	case dcgm.DCGM_FI_DEV_MEMORY_TEMP:
		sample.MemoryTemperature = temperatureNumberPointer(value)
	case dcgm.DCGM_FI_DEV_MEM_MAX_OP_TEMP:
		sample.MemoryTemperatureMax = temperatureNumberPointer(value)
	case dcgm.DCGM_FI_DEV_FB_FREE:
		if parsed := int64Field(value); parsed != nil {
			sample.MemoryFreeBytes = mibToBytes(*parsed)
		}
	case dcgm.DCGM_FI_DEV_FB_USED:
		if parsed := int64Field(value); parsed != nil {
			sample.MemoryUsedBytes = mibToBytes(*parsed)
		}
	case dcgm.DCGM_FI_DEV_FB_TOTAL:
		if parsed := int64Field(value); parsed != nil {
			sample.MemoryTotalBytes = mibToBytes(*parsed)
		}
	case dcgm.DCGM_FI_DEV_FB_RESERVED:
		if parsed := int64Field(value); parsed != nil {
			sample.MemoryReservedBytes = mibToBytes(*parsed)
		}
	case dcgm.DCGM_FI_DEV_BAR1_FREE:
		if parsed := int64Field(value); parsed != nil {
			sample.BAR1FreeBytes = mibToBytes(*parsed)
		}
	case dcgm.DCGM_FI_DEV_BAR1_USED:
		if parsed := int64Field(value); parsed != nil {
			sample.BAR1UsedBytes = mibToBytes(*parsed)
		}
	case dcgm.DCGM_FI_DEV_BAR1_TOTAL:
		if parsed := int64Field(value); parsed != nil {
			sample.BAR1TotalBytes = mibToBytes(*parsed)
		}
	case dcgm.DCGM_FI_DEV_FB_USED_PERCENT:
		if parsed := float64Field(value); parsed != nil {
			sample.MemoryUsedPercent = ratio01PercentPointer(*parsed)
		}
	case dcgm.DCGM_FI_DEV_CLOCK_THROTTLE_REASONS:
		if parsed := int64Field(value); parsed != nil {
			sample.ThrottleReasons = uint64Pointer(*parsed)
		}
	case dcgm.DCGM_FI_DEV_SM_CLOCK:
		sample.SMClockHertz = megahertzToHertzPointer(value)
	case dcgm.DCGM_FI_DEV_MEM_CLOCK:
		sample.MemoryClockHertz = megahertzToHertzPointer(value)
	case dcgm.DCGM_FI_DEV_PSTATE:
		sample.PerformanceState = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_FAN_SPEED:
		sample.FanSpeedPercent = percentNumberPointer(value)
	case dcgm.DCGM_FI_DEV_PCIE_TX_THROUGHPUT:
		sample.PCIeTXBytesPerSecond = kibPerSecondToBytesPerSecondPointer(value)
	case dcgm.DCGM_FI_DEV_PCIE_RX_THROUGHPUT:
		sample.PCIeRXBytesPerSecond = kibPerSecondToBytesPerSecondPointer(value)
	case dcgm.DCGM_FI_DEV_PCIE_REPLAY_COUNTER:
		sample.PCIeReplayCounter = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_PCIE_LINK_GEN:
		sample.PCIeLinkGeneration = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_PCIE_LINK_WIDTH:
		sample.PCIeLinkWidth = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_PCIE_MAX_LINK_GEN:
		sample.PCIeMaxLinkGeneration = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_PCIE_MAX_LINK_WIDTH:
		sample.PCIeMaxLinkWidth = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_XID_ERRORS:
		sample.XIDLastCode = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_ECC_SBE_VOL_TOTAL:
		sample.ECCSBEVolatileTotal = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_ECC_DBE_VOL_TOTAL:
		sample.ECCDBEVolatileTotal = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_ECC_SBE_AGG_TOTAL:
		sample.ECCSBEAggregateTotal = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_ECC_DBE_AGG_TOTAL:
		sample.ECCDBEAggregateTotal = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_RETIRED_SBE:
		sample.RetiredSBEPages = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_RETIRED_DBE:
		sample.RetiredDBEPages = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_RETIRED_PENDING:
		sample.RetiredPendingPages = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_CORRECTABLE_REMAPPED_ROWS:
		sample.CorrectableRemappedRows = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_UNCORRECTABLE_REMAPPED_ROWS:
		sample.UncorrectableRemappedRows = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_ROW_REMAP_FAILURE:
		sample.RowRemapFailure = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_ROW_REMAP_PENDING:
		sample.RowRemapPending = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_DEV_POWER_VIOLATION:
		sample.PowerViolationSeconds = nanosecondsToSecondsPointer(value)
	case dcgm.DCGM_FI_DEV_THERMAL_VIOLATION:
		sample.ThermalViolationSeconds = nanosecondsToSecondsPointer(value)
	case dcgm.DCGM_FI_DEV_SYNC_BOOST_VIOLATION:
		sample.SyncBoostViolationSeconds = nanosecondsToSecondsPointer(value)
	case dcgm.DCGM_FI_DEV_BOARD_LIMIT_VIOLATION:
		sample.BoardLimitViolationSeconds = nanosecondsToSecondsPointer(value)
	case dcgm.DCGM_FI_DEV_LOW_UTIL_VIOLATION:
		sample.LowUtilViolationSeconds = nanosecondsToSecondsPointer(value)
	case dcgm.DCGM_FI_DEV_RELIABILITY_VIOLATION:
		sample.ReliabilityViolationSeconds = nanosecondsToSecondsPointer(value)
	case dcgm.DCGM_FI_DEV_TOTAL_APP_CLOCKS_VIOLATION:
		sample.AppClockViolationSeconds = nanosecondsToSecondsPointer(value)
	case dcgm.DCGM_FI_DEV_TOTAL_BASE_CLOCKS_VIOLATION:
		sample.BaseClockViolationSeconds = nanosecondsToSecondsPointer(value)
	case dcgm.DCGM_FI_PROF_GR_ENGINE_ACTIVE:
		if parsed := float64Field(value); parsed != nil {
			sample.ProfGraphicsEngineActive = ratio01Pointer(*parsed)
		}
	case dcgm.DCGM_FI_PROF_SM_ACTIVE:
		if parsed := float64Field(value); parsed != nil {
			sample.ProfSMActive = ratio01Pointer(*parsed)
		}
	case dcgm.DCGM_FI_PROF_SM_OCCUPANCY:
		if parsed := float64Field(value); parsed != nil {
			sample.ProfSMOccupancy = ratio01Pointer(*parsed)
		}
	case dcgm.DCGM_FI_PROF_DRAM_ACTIVE:
		if parsed := float64Field(value); parsed != nil {
			sample.ProfDRAMActive = ratio01Pointer(*parsed)
		}
	case dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE:
		if parsed := float64Field(value); parsed != nil {
			sample.ProfTensorActive = ratio01Pointer(*parsed)
		}
	case dcgm.DCGM_FI_PROF_PCIE_TX_BYTES:
		sample.ProfPCIeTXBytesPerSecond = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_PROF_PCIE_RX_BYTES:
		sample.ProfPCIeRXBytesPerSecond = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_PROF_NVLINK_TX_BYTES:
		sample.ProfNVLinkTXBytesPerSecond = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_PROF_NVLINK_RX_BYTES:
		sample.ProfNVLinkRXBytesPerSecond = nonNegativeNumberPointer(value)
	case dcgm.DCGM_FI_PROF_PIPE_FP64_ACTIVE:
		if parsed := float64Field(value); parsed != nil {
			sample.ProfPipeFP64Active = ratio01Pointer(*parsed)
		}
	case dcgm.DCGM_FI_PROF_PIPE_FP32_ACTIVE:
		if parsed := float64Field(value); parsed != nil {
			sample.ProfPipeFP32Active = ratio01Pointer(*parsed)
		}
	case dcgm.DCGM_FI_PROF_PIPE_FP16_ACTIVE:
		if parsed := float64Field(value); parsed != nil {
			sample.ProfPipeFP16Active = ratio01Pointer(*parsed)
		}
	case dcgm.DCGM_FI_PROF_PIPE_INT_ACTIVE:
		if parsed := float64Field(value); parsed != nil {
			sample.ProfPipeINTActive = ratio01Pointer(*parsed)
		}
	case dcgm.DCGM_FI_PROF_PIPE_TENSOR_HMMA_ACTIVE:
		if parsed := float64Field(value); parsed != nil {
			sample.ProfTensorHMMAActive = ratio01Pointer(*parsed)
		}
	case dcgm.DCGM_FI_PROF_PIPE_TENSOR_IMMA_ACTIVE:
		if parsed := float64Field(value); parsed != nil {
			sample.ProfTensorIMMAActive = ratio01Pointer(*parsed)
		}
	case dcgm.DCGM_FI_PROF_PIPE_TENSOR_DFMA_ACTIVE:
		if parsed := float64Field(value); parsed != nil {
			sample.ProfTensorDFMAActive = ratio01Pointer(*parsed)
		}
	}
	if *target == nil {
		return 0, false
	}
	return **target, true
}

func sampleFieldTarget(sample *GPUSample, fieldID dcgm.Short) **float64 {
	switch fieldID {
	case dcgm.DCGM_FI_DEV_GPU_UTIL:
		return &sample.Utilization
	case dcgm.DCGM_FI_DEV_MEM_COPY_UTIL:
		return &sample.MemoryCopyUtil
	case dcgm.DCGM_FI_DEV_ENC_UTIL:
		return &sample.EncoderUtil
	case dcgm.DCGM_FI_DEV_DEC_UTIL:
		return &sample.DecoderUtil
	case dcgm.DCGM_FI_DEV_POWER_USAGE:
		return &sample.PowerDrawWatts
	case dcgm.DCGM_FI_DEV_POWER_USAGE_INSTANT:
		return &sample.PowerDrawInstantWatts
	case dcgm.DCGM_FI_DEV_POWER_MGMT_LIMIT:
		return &sample.PowerLimitWatts
	case dcgm.DCGM_FI_DEV_ENFORCED_POWER_LIMIT:
		return &sample.PowerEnforcedLimitWatts
	case dcgm.DCGM_FI_DEV_TOTAL_ENERGY_CONSUMPTION:
		return &sample.TotalEnergyJoules
	case dcgm.DCGM_FI_DEV_GPU_TEMP:
		return &sample.Temperature
	case dcgm.DCGM_FI_DEV_GPU_MAX_OP_TEMP:
		return &sample.TemperatureMax
	case dcgm.DCGM_FI_DEV_MEMORY_TEMP:
		return &sample.MemoryTemperature
	case dcgm.DCGM_FI_DEV_MEM_MAX_OP_TEMP:
		return &sample.MemoryTemperatureMax
	case dcgm.DCGM_FI_DEV_FB_FREE:
		return &sample.MemoryFreeBytes
	case dcgm.DCGM_FI_DEV_FB_USED:
		return &sample.MemoryUsedBytes
	case dcgm.DCGM_FI_DEV_FB_TOTAL:
		return &sample.MemoryTotalBytes
	case dcgm.DCGM_FI_DEV_FB_RESERVED:
		return &sample.MemoryReservedBytes
	case dcgm.DCGM_FI_DEV_BAR1_FREE:
		return &sample.BAR1FreeBytes
	case dcgm.DCGM_FI_DEV_BAR1_USED:
		return &sample.BAR1UsedBytes
	case dcgm.DCGM_FI_DEV_BAR1_TOTAL:
		return &sample.BAR1TotalBytes
	case dcgm.DCGM_FI_DEV_FB_USED_PERCENT:
		return &sample.MemoryUsedPercent
	case dcgm.DCGM_FI_DEV_CLOCK_THROTTLE_REASONS:
		return &sample.ThrottleReasons
	case dcgm.DCGM_FI_DEV_SM_CLOCK:
		return &sample.SMClockHertz
	case dcgm.DCGM_FI_DEV_MEM_CLOCK:
		return &sample.MemoryClockHertz
	case dcgm.DCGM_FI_DEV_PSTATE:
		return &sample.PerformanceState
	case dcgm.DCGM_FI_DEV_FAN_SPEED:
		return &sample.FanSpeedPercent
	case dcgm.DCGM_FI_DEV_PCIE_TX_THROUGHPUT:
		return &sample.PCIeTXBytesPerSecond
	case dcgm.DCGM_FI_DEV_PCIE_RX_THROUGHPUT:
		return &sample.PCIeRXBytesPerSecond
	case dcgm.DCGM_FI_DEV_PCIE_REPLAY_COUNTER:
		return &sample.PCIeReplayCounter
	case dcgm.DCGM_FI_DEV_PCIE_LINK_GEN:
		return &sample.PCIeLinkGeneration
	case dcgm.DCGM_FI_DEV_PCIE_LINK_WIDTH:
		return &sample.PCIeLinkWidth
	case dcgm.DCGM_FI_DEV_PCIE_MAX_LINK_GEN:
		return &sample.PCIeMaxLinkGeneration
	case dcgm.DCGM_FI_DEV_PCIE_MAX_LINK_WIDTH:
		return &sample.PCIeMaxLinkWidth
	case dcgm.DCGM_FI_DEV_XID_ERRORS:
		return &sample.XIDLastCode
	case dcgm.DCGM_FI_DEV_ECC_SBE_VOL_TOTAL:
		return &sample.ECCSBEVolatileTotal
	case dcgm.DCGM_FI_DEV_ECC_DBE_VOL_TOTAL:
		return &sample.ECCDBEVolatileTotal
	case dcgm.DCGM_FI_DEV_ECC_SBE_AGG_TOTAL:
		return &sample.ECCSBEAggregateTotal
	case dcgm.DCGM_FI_DEV_ECC_DBE_AGG_TOTAL:
		return &sample.ECCDBEAggregateTotal
	case dcgm.DCGM_FI_DEV_RETIRED_SBE:
		return &sample.RetiredSBEPages
	case dcgm.DCGM_FI_DEV_RETIRED_DBE:
		return &sample.RetiredDBEPages
	case dcgm.DCGM_FI_DEV_RETIRED_PENDING:
		return &sample.RetiredPendingPages
	case dcgm.DCGM_FI_DEV_CORRECTABLE_REMAPPED_ROWS:
		return &sample.CorrectableRemappedRows
	case dcgm.DCGM_FI_DEV_UNCORRECTABLE_REMAPPED_ROWS:
		return &sample.UncorrectableRemappedRows
	case dcgm.DCGM_FI_DEV_ROW_REMAP_FAILURE:
		return &sample.RowRemapFailure
	case dcgm.DCGM_FI_DEV_ROW_REMAP_PENDING:
		return &sample.RowRemapPending
	case dcgm.DCGM_FI_DEV_POWER_VIOLATION:
		return &sample.PowerViolationSeconds
	case dcgm.DCGM_FI_DEV_THERMAL_VIOLATION:
		return &sample.ThermalViolationSeconds
	case dcgm.DCGM_FI_DEV_SYNC_BOOST_VIOLATION:
		return &sample.SyncBoostViolationSeconds
	case dcgm.DCGM_FI_DEV_BOARD_LIMIT_VIOLATION:
		return &sample.BoardLimitViolationSeconds
	case dcgm.DCGM_FI_DEV_LOW_UTIL_VIOLATION:
		return &sample.LowUtilViolationSeconds
	case dcgm.DCGM_FI_DEV_RELIABILITY_VIOLATION:
		return &sample.ReliabilityViolationSeconds
	case dcgm.DCGM_FI_DEV_TOTAL_APP_CLOCKS_VIOLATION:
		return &sample.AppClockViolationSeconds
	case dcgm.DCGM_FI_DEV_TOTAL_BASE_CLOCKS_VIOLATION:
		return &sample.BaseClockViolationSeconds
	case dcgm.DCGM_FI_PROF_GR_ENGINE_ACTIVE:
		return &sample.ProfGraphicsEngineActive
	case dcgm.DCGM_FI_PROF_SM_ACTIVE:
		return &sample.ProfSMActive
	case dcgm.DCGM_FI_PROF_SM_OCCUPANCY:
		return &sample.ProfSMOccupancy
	case dcgm.DCGM_FI_PROF_DRAM_ACTIVE:
		return &sample.ProfDRAMActive
	case dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE:
		return &sample.ProfTensorActive
	case dcgm.DCGM_FI_PROF_PCIE_TX_BYTES:
		return &sample.ProfPCIeTXBytesPerSecond
	case dcgm.DCGM_FI_PROF_PCIE_RX_BYTES:
		return &sample.ProfPCIeRXBytesPerSecond
	case dcgm.DCGM_FI_PROF_NVLINK_TX_BYTES:
		return &sample.ProfNVLinkTXBytesPerSecond
	case dcgm.DCGM_FI_PROF_NVLINK_RX_BYTES:
		return &sample.ProfNVLinkRXBytesPerSecond
	case dcgm.DCGM_FI_PROF_PIPE_FP64_ACTIVE:
		return &sample.ProfPipeFP64Active
	case dcgm.DCGM_FI_PROF_PIPE_FP32_ACTIVE:
		return &sample.ProfPipeFP32Active
	case dcgm.DCGM_FI_PROF_PIPE_FP16_ACTIVE:
		return &sample.ProfPipeFP16Active
	case dcgm.DCGM_FI_PROF_PIPE_INT_ACTIVE:
		return &sample.ProfPipeINTActive
	case dcgm.DCGM_FI_PROF_PIPE_TENSOR_HMMA_ACTIVE:
		return &sample.ProfTensorHMMAActive
	case dcgm.DCGM_FI_PROF_PIPE_TENSOR_IMMA_ACTIVE:
		return &sample.ProfTensorIMMAActive
	case dcgm.DCGM_FI_PROF_PIPE_TENSOR_DFMA_ACTIVE:
		return &sample.ProfTensorDFMAActive
	default:
		return nil
	}
}

func initDCGM(value string) (func(), error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "embedded":
		return dcgm.Init(dcgm.Embedded)
	case "standalone":
		return dcgm.Init(dcgm.Standalone)
	case "start-hostengine", "starthostengine":
		return dcgm.Init(dcgm.StartHostengine)
	default:
		return nil, fmt.Errorf("unsupported DCGM mode %q", value)
	}
}

func formatCUDAVersion(raw int64) string {
	if parsed := int64Value(raw); parsed == nil || *parsed <= 0 {
		return ""
	} else {
		return fmt.Sprintf("%d.%d", *parsed/1000, (*parsed%1000)/10)
	}
}

func mibToBytes(value int64) *float64 {
	parsed := int64Value(value)
	if parsed == nil || *parsed < 0 {
		return nil
	}
	result := float64(*parsed) * 1024 * 1024
	return &result
}

func int64Value(value int64) *int64 {
	if dcgm.IsInt64Blank(value) {
		return nil
	}
	return &value
}

func float64Value(value float64) *float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value >= dcgm.DCGM_FT_FP64_BLANK {
		return nil
	}
	return &value
}

func fieldSuccessful(value dcgm.FieldValue_v1) bool {
	return value.Status == dcgm.DCGM_ST_OK
}

func int64Field(value dcgm.FieldValue_v1) *int64 {
	if !fieldSuccessful(value) || value.FieldType != dcgm.DCGM_FT_INT64 {
		return nil
	}
	return int64Value(value.Int64())
}

func float64Field(value dcgm.FieldValue_v1) *float64 {
	if !fieldSuccessful(value) || value.FieldType != dcgm.DCGM_FT_DOUBLE {
		return nil
	}
	return float64Value(value.Float64())
}

func numberField(value dcgm.FieldValue_v1) *float64 {
	if !fieldSuccessful(value) {
		return nil
	}
	switch value.FieldType {
	case dcgm.DCGM_FT_INT64:
		parsed := int64Value(value.Int64())
		if parsed == nil {
			return nil
		}
		result := float64(*parsed)
		return &result
	case dcgm.DCGM_FT_DOUBLE:
		return float64Value(value.Float64())
	default:
		return nil
	}
}

func validDCGMStringField(value dcgm.FieldValue_v1) string {
	if !fieldSuccessful(value) || value.FieldType != dcgm.DCGM_FT_STRING {
		return ""
	}
	return validDCGMString(value.String())
}

// percentPointer возвращает nil для blank-значений DCGM, чтобы отличать
// "нет данных" от нуля.
func percentPointer(value int64) *float64 {
	parsed := int64Value(value)
	if parsed == nil || *parsed < 0 || *parsed > 100 {
		return nil
	}
	result := float64(*parsed)
	return &result
}

func percentNumberPointer(value dcgm.FieldValue_v1) *float64 {
	parsed := nonNegativeNumberPointer(value)
	if parsed == nil || *parsed > 100 {
		return nil
	}
	return parsed
}

func nonNegativeNumberPointer(value dcgm.FieldValue_v1) *float64 {
	parsed := numberField(value)
	if parsed == nil || *parsed < 0 {
		return nil
	}
	return parsed
}

func ratio01PercentPointer(value float64) *float64 {
	parsed := float64Value(value)
	if parsed == nil || *parsed < 0 || *parsed > 1 {
		return nil
	}
	result := *parsed * 100
	return &result
}

func uint64Pointer(value int64) *float64 {
	parsed := int64Value(value)
	if parsed == nil || *parsed < 0 {
		return nil
	}
	result := float64(*parsed)
	return &result
}

func ratio01Pointer(value float64) *float64 {
	parsed := float64Value(value)
	if parsed == nil || *parsed < 0 || *parsed > 1 {
		return nil
	}
	return parsed
}

func temperaturePointer(value int64) *float64 {
	parsed := int64Value(value)
	if parsed == nil || *parsed < 0 || *parsed > 150 {
		return nil
	}
	result := float64(*parsed)
	return &result
}

func temperatureNumberPointer(value dcgm.FieldValue_v1) *float64 {
	parsed := nonNegativeNumberPointer(value)
	if parsed == nil || *parsed > 150 {
		return nil
	}
	return parsed
}

func megahertzToHertzPointer(value dcgm.FieldValue_v1) *float64 {
	parsed := nonNegativeNumberPointer(value)
	if parsed == nil {
		return nil
	}
	result := *parsed * 1000 * 1000
	return &result
}

func kibPerSecondToBytesPerSecondPointer(value dcgm.FieldValue_v1) *float64 {
	parsed := nonNegativeNumberPointer(value)
	if parsed == nil {
		return nil
	}
	result := *parsed * 1024
	return &result
}

func milliJoulesToJoulesPointer(value dcgm.FieldValue_v1) *float64 {
	parsed := nonNegativeNumberPointer(value)
	if parsed == nil {
		return nil
	}
	result := *parsed / 1000
	return &result
}

func nanosecondsToSecondsPointer(value dcgm.FieldValue_v1) *float64 {
	parsed := nonNegativeNumberPointer(value)
	if parsed == nil {
		return nil
	}
	result := *parsed / 1_000_000_000
	return &result
}

func powerDrawPointer(value float64) *float64 {
	parsed := float64Value(value)
	if parsed == nil || *parsed < 0 || *parsed > 1000000 {
		return nil
	}
	return parsed
}

func wattsPointer(value float64) *float64 {
	parsed := powerDrawPointer(value)
	if parsed == nil || *parsed <= 0 {
		return nil
	}
	return parsed
}

func validDCGMString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "<<<") {
		return ""
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if parsed := validDCGMString(value); parsed != "" {
			return parsed
		}
	}
	return ""
}
