package main

import (
	"sync"
	"time"
)

// Названия агрегируемых метрик. Используются как часть windowKey.
const (
	aggUtilization     = "utilization"
	aggMemoryCopyUtil  = "memory_copy_util"
	aggMemoryUsedPct   = "memory_used_percent"
	aggPowerDraw       = "power_draw"
	aggTemperature     = "temperature"
	aggProfSM          = "prof_sm"
	aggProfGraphics    = "prof_graphics"
	aggProfSMOccupancy = "prof_sm_occupancy"
	aggProfDRAM        = "prof_dram"
	aggProfTensor      = "prof_tensor"
	aggProfFP64        = "prof_fp64"
	aggProfFP32        = "prof_fp32"
	aggProfFP16        = "prof_fp16"
	aggProfINT         = "prof_int"
	aggProfTensorHMMA  = "prof_tensor_hmma"
	aggProfTensorIMMA  = "prof_tensor_imma"
	aggProfTensorDFMA  = "prof_tensor_dfma"
	aggPCIeTransmit    = "pcie_transmit"
	aggPCIeReceive     = "pcie_receive"
	aggNVLinkTransmit  = "nvlink_transmit"
	aggNVLinkReceive   = "nvlink_receive"
)

// windowKey идентифицирует одну серию внутри fixed aggregation window.
// Struct-ключ вместо конкатенации строк ("0:utilization") избавляет от
// аллокации строки на каждый DCGM-сэмпл.
type windowKey struct {
	gpuIndex string
	metric   string
}

// WindowStats накапливает статистику одной метрики внутри окна. Один
// максимум не отличает короткий всплеск от
// постоянной нагрузки, поэтому дополнительно считаем sum/count.
type WindowStats struct {
	Max   float64
	Sum   float64
	Count int
}

// Avg возвращает среднее по окну или 0, если в окне не было сэмплов.
func (s WindowStats) Avg() float64 {
	if s.Count == 0 {
		return 0
	}
	return s.Sum / float64(s.Count)
}

// WindowAggregator копит все значения из DCGM history. Snapshot()
// атомарно завершает fixed window и начинает следующее.
type WindowAggregator struct {
	mu      sync.Mutex
	windows map[windowKey]WindowStats
}

func NewWindowAggregator() *WindowAggregator {
	return &WindowAggregator{windows: make(map[windowKey]WindowStats)}
}

// Observe записывает один сэмпл в текущее окно.
func (a *WindowAggregator) Observe(gpuIndex, metric string, value float64) {
	key := windowKey{gpuIndex: gpuIndex, metric: metric}

	a.mu.Lock()
	defer a.mu.Unlock()

	stats := a.windows[key]
	if stats.Count == 0 || value > stats.Max {
		stats.Max = value
	}
	stats.Sum += value
	stats.Count++
	a.windows[key] = stats
}

// Snapshot атомарно возвращает накопленную статистику и начинает новое окно.
// Один захват мьютекса и подмена всей map гарантируют консистентность: даже
// если collect() выполняется параллельно с публикацией, все метрики снапшота
// принадлежат одному и тому же моменту времени.
func (a *WindowAggregator) Snapshot() map[windowKey]WindowStats {
	a.mu.Lock()
	defer a.mu.Unlock()

	snapshot := a.windows
	a.windows = make(map[windowKey]WindowStats, len(snapshot))
	return snapshot
}

// ActivityTracker определяет "запросы" к GPU по фронтам загрузки: переход
// через порог вверх — начало активности, вниз — конец. Если активность
// длилась не меньше minRequestTime, она засчитывается как один запрос.
type ActivityTracker struct {
	mu              sync.Mutex
	states          map[string]gpuActivity
	activeThreshold float64
	minRequestTime  time.Duration
}

type gpuActivity struct {
	active     bool
	activeFrom time.Time
}

func NewActivityTracker(activeThreshold float64, minRequestTime time.Duration) *ActivityTracker {
	return &ActivityTracker{
		states:          make(map[string]gpuActivity),
		activeThreshold: activeThreshold,
		minRequestTime:  minRequestTime,
	}
}

// Observe принимает очередной сэмпл загрузки и возвращает true, если на
// этом сэмпле завершился "запрос" достаточной длительности.
func (t *ActivityTracker) Observe(gpuIndex string, util float64, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	state := t.states[gpuIndex]
	isActive := util > t.activeThreshold

	switch {
	case isActive && !state.active:
		// Запоминаем, когда началась активность GPU.
		state.active = true
		state.activeFrom = now
	case !isActive && state.active:
		// Засчитываем запрос, только если активность
		// продлилась достаточно долго.
		wasRequest := now.Sub(state.activeFrom) >= t.minRequestTime
		state.active = false
		t.states[gpuIndex] = state
		return wasRequest
	}

	t.states[gpuIndex] = state
	return false
}
