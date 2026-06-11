package main

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/NVIDIA/go-dcgm/pkg/dcgm"
)

const (
	watchMaxKeepAge     = 0
	watchMaxKeepSamples = 1
)

type DCGMClient struct {
	cleanup        func()
	mode           string
	updateInterval time.Duration
	logger         *slog.Logger
	mu             sync.Mutex
	devices        map[uint]GPUInfo
	watchers       map[uint]*gpuWatcher
}

type gpuWatcher struct {
	fields       []dcgm.Short
	fieldGroupID dcgm.FieldHandle
	groupID      dcgm.GroupHandle
}

type GPUInfo struct {
	ID               uint
	Index            string
	Name             string
	UUID             string
	Driver           string
	CUDAVersion      string
	MemoryTotalBytes float64
	PowerLimitWatts  *float64
}

type GPUSample struct {
	Info              GPUInfo
	Utilization       *float64
	MemoryCopyUtil    *float64
	MemoryFreeBytes   float64
	MemoryUsedBytes   float64
	MemoryTotalBytes  float64
	MemoryUsedPercent *float64
	Temperature       *float64
	TemperatureMax    *float64
	PowerDrawWatts    *float64
	PowerLimitWatts   *float64
	ThrottleReasons   *float64
	ProfSMActive      *float64
	ProfDRAMActive    *float64
	ProfTensorActive  *float64
}

func NewDCGMClient(cfg Config, logger *slog.Logger) (*DCGMClient, error) {
	cleanup, err := initDCGM(cfg.DCGMMode)
	if err != nil {
		return nil, err
	}

	return &DCGMClient{
		cleanup:        cleanup,
		mode:           cfg.DCGMMode,
		updateInterval: cfg.ScrapeInterval,
		logger:         logger,
		devices:        make(map[uint]GPUInfo),
		watchers:       make(map[uint]*gpuWatcher),
	}, nil
}

func (c *DCGMClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for gpuID, watcher := range c.watchers {
		if err := dcgm.DestroyGroup(watcher.groupID); err != nil {
			c.logger.Debug("failed to destroy DCGM watcher group", "gpu_id", gpuID, "error", err)
		}
		if err := dcgm.FieldGroupDestroy(watcher.fieldGroupID); err != nil {
			c.logger.Debug("failed to destroy DCGM watcher field group", "gpu_id", gpuID, "error", err)
		}
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
			if parsed := validDCGMString(value.String()); parsed != "" {
				driverVersion = parsed
			}
		case dcgm.DCGM_FI_CUDA_DRIVER_VERSION:
			if parsed := formatCUDAVersion(value.Int64()); parsed != "" {
				cudaVersion = parsed
			}
		}
	}

	return driverVersion, cudaVersion
}

func (c *DCGMClient) Samples() ([]GPUSample, error) {
	gpuIDs, err := dcgm.GetSupportedDevices()
	if err != nil {
		return nil, fmt.Errorf("list DCGM-supported GPUs: %w", err)
	}

	samples := make([]GPUSample, 0, len(gpuIDs))
	for _, gpuID := range gpuIDs {
		info, err := c.cachedGPUInfo(gpuID)
		if err != nil {
			c.logger.Warn("failed to query GPU identity", "gpu_id", gpuID, "error", err)
			info = GPUInfo{
				ID:          gpuID,
				Index:       fmt.Sprintf("%d", gpuID),
				Name:        "unknown",
				UUID:        "unknown",
				Driver:      "unknown",
				CUDAVersion: "unknown",
			}
		}

		watcher, err := c.ensureWatcher(gpuID)
		if err != nil {
			c.logger.Warn("failed to prepare DCGM watcher", "gpu_id", gpuID, "error", err)
			continue
		}

		sample := GPUSample{
			Info:             info,
			MemoryTotalBytes: info.MemoryTotalBytes,
			PowerLimitWatts:  info.PowerLimitWatts,
		}

		c.applyFieldValues(gpuID, watcher.fields, &sample)
		if sample.MemoryFreeBytes == 0 && sample.MemoryTotalBytes > 0 && sample.MemoryUsedBytes > 0 {
			sample.MemoryFreeBytes = max(sample.MemoryTotalBytes-sample.MemoryUsedBytes, 0)
		}
		samples = append(samples, sample)
	}

	return samples, nil
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
				if parsed := int64Value(value.Int64()); parsed != nil {
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

func baseFields() []dcgm.Short {
	return []dcgm.Short{
		dcgm.DCGM_FI_DEV_GPU_UTIL,
		dcgm.DCGM_FI_DEV_MEM_COPY_UTIL,
		dcgm.DCGM_FI_DEV_POWER_USAGE,
		dcgm.DCGM_FI_DEV_POWER_MGMT_LIMIT,
		dcgm.DCGM_FI_DEV_GPU_TEMP,
		dcgm.DCGM_FI_DEV_GPU_MAX_OP_TEMP,
		dcgm.DCGM_FI_DEV_FB_FREE,
		dcgm.DCGM_FI_DEV_FB_USED,
		dcgm.DCGM_FI_DEV_FB_TOTAL,
		dcgm.DCGM_FI_DEV_FB_USED_PERCENT,
		dcgm.DCGM_FI_DEV_CLOCK_THROTTLE_REASONS,
	}
}

func profFields() []dcgm.Short {
	return []dcgm.Short{
		dcgm.DCGM_FI_PROF_SM_ACTIVE,
		dcgm.DCGM_FI_PROF_DRAM_ACTIVE,
		dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE,
	}
}

func (c *DCGMClient) ensureWatcher(gpuID uint) (*gpuWatcher, error) {
	c.mu.Lock()
	if watcher, ok := c.watchers[gpuID]; ok {
		c.mu.Unlock()
		return watcher, nil
	}
	c.mu.Unlock()

	// Сначала пытаемся подписаться на полный набор полей, включая
	// профилирующие. Если GPU/драйвер их не поддерживает, откатываемся
	// на базовый набор — exporter продолжит работать без prof-метрик.
	fields := append(baseFields(), profFields()...)
	watcher, err := c.createWatcher(gpuID, fields)
	if err != nil {
		c.logger.Warn("DCGM profiling fields unavailable, falling back to basic fields",
			"gpu_id", gpuID, "error", err)
		watcher, err = c.createWatcher(gpuID, baseFields())
		if err != nil {
			return nil, err
		}
	}

	c.mu.Lock()
	c.watchers[gpuID] = watcher
	c.mu.Unlock()

	return watcher, nil
}

// createWatcher создает DCGM field group и подписку на обновления полей.
func (c *DCGMClient) createWatcher(gpuID uint, fields []dcgm.Short) (*gpuWatcher, error) {
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
		c.updateInterval.Microseconds(), watchMaxKeepAge, watchMaxKeepSamples)
	if err != nil {
		destroy()
		return nil, fmt.Errorf("watch DCGM fields: %w", err)
	}

	return &gpuWatcher{
		fields:       fields,
		fieldGroupID: fieldGroupID,
		groupID:      groupID,
	}, nil
}

// applyFieldValues читает последние значения watch-полей. Hostengine в
// режиме AUTO сам обновляет их с частотой updateInterval.
func (c *DCGMClient) applyFieldValues(gpuID uint, fields []dcgm.Short, sample *GPUSample) {
	values, err := dcgm.GetLatestValuesForFields(gpuID, fields)
	if err != nil {
		c.logger.Debug("failed to query extended GPU fields", "gpu_id", gpuID, "error", err)
		return
	}

	for _, value := range values {
		switch value.FieldID {
		case dcgm.DCGM_FI_DEV_GPU_UTIL:
			sample.Utilization = percentPointer(value.Int64())
		case dcgm.DCGM_FI_DEV_MEM_COPY_UTIL:
			sample.MemoryCopyUtil = percentPointer(value.Int64())
		case dcgm.DCGM_FI_DEV_POWER_USAGE:
			sample.PowerDrawWatts = powerDrawPointer(value.Float64())
		case dcgm.DCGM_FI_DEV_POWER_MGMT_LIMIT:
			if limit := wattsPointer(value.Float64()); limit != nil {
				sample.PowerLimitWatts = limit
			} else if parsed := int64Value(value.Int64()); parsed != nil && *parsed < 1000000 {
				limit := float64(*parsed)
				sample.PowerLimitWatts = &limit
			}
		case dcgm.DCGM_FI_DEV_GPU_TEMP:
			sample.Temperature = temperaturePointer(value.Int64())
		case dcgm.DCGM_FI_DEV_GPU_MAX_OP_TEMP:
			if parsed := int64Value(value.Int64()); parsed != nil {
				tempMax := float64(*parsed)
				sample.TemperatureMax = &tempMax
			}
		case dcgm.DCGM_FI_DEV_FB_FREE:
			sample.MemoryFreeBytes = mibToBytes(value.Int64())
		case dcgm.DCGM_FI_DEV_FB_USED:
			sample.MemoryUsedBytes = mibToBytes(value.Int64())
		case dcgm.DCGM_FI_DEV_FB_TOTAL:
			sample.MemoryTotalBytes = mibToBytes(value.Int64())
		case dcgm.DCGM_FI_DEV_FB_USED_PERCENT:
			sample.MemoryUsedPercent = percentPointer(value.Int64())
		case dcgm.DCGM_FI_DEV_CLOCK_THROTTLE_REASONS:
			sample.ThrottleReasons = uint64Pointer(value.Int64())
		case dcgm.DCGM_FI_PROF_SM_ACTIVE:
			sample.ProfSMActive = ratioPointer(value.Float64())
		case dcgm.DCGM_FI_PROF_DRAM_ACTIVE:
			sample.ProfDRAMActive = ratioPointer(value.Float64())
		case dcgm.DCGM_FI_PROF_PIPE_TENSOR_ACTIVE:
			sample.ProfTensorActive = ratioPointer(value.Float64())
		}
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

func mibToBytes(value int64) float64 {
	parsed := int64Value(value)
	if parsed == nil || *parsed < 0 {
		return 0
	}
	return float64(*parsed) * 1024 * 1024
}

func int64Value(value int64) *int64 {
	if dcgm.IsInt64Blank(value) {
		return nil
	}
	return &value
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

func uint64Pointer(value int64) *float64 {
	parsed := int64Value(value)
	if parsed == nil || *parsed < 0 {
		return nil
	}
	result := float64(*parsed)
	return &result
}

func ratioPointer(value float64) *float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
		return nil
	}
	return &value
}

func temperaturePointer(value int64) *float64 {
	parsed := int64Value(value)
	if parsed == nil || *parsed < 0 || *parsed > 150 {
		return nil
	}
	result := float64(*parsed)
	return &result
}

func powerDrawPointer(value float64) *float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1000000 {
		return nil
	}
	return &value
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
