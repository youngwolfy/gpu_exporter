# GPU Exporter

Language: [Русский](#русский) | [English](#english)

## Русский

GPU Exporter — это локальный экспортер метрик NVIDIA GPU. Приложение написано на Go и использует NVIDIA DCGM через [`github.com/NVIDIA/go-dcgm`](https://github.com/NVIDIA/go-dcgm).

Экспортер рассчитан на локальную работу и не требует доступа к Интернету во время работы. Интернет нужен только для скачивания Go-модулей или установки системных пакетов.

### Возможности

- Метрики NVIDIA GPU через локальный endpoint `/metrics`.
- Сбор метрик через DCGM и `go-dcgm`.
- Локальный HTTP endpoint `127.0.0.1:9990` по умолчанию.
- Раздельные интервалы для fast, profiling, operational и reliability-полей; история читается через `GetValuesSince`, а пики (`_max`) и средние (`_avg`) считаются по фактическим DCGM-сэмплам.
- Структурированные JSON-логи с настраиваемым уровнем логирования.
- Метрики используют префикс `gpu_`.

### Требования

- Go `1.26.4` или новее
- Драйвер NVIDIA
- Runtime- и development-библиотеки NVIDIA DCGM
- Linux с доступом к NVIDIA GPU

Пакеты DCGM распространяются через репозитории NVIDIA. Названия пакетов зависят от дистрибутива и версии CUDA — см. [документацию DCGM](https://docs.nvidia.com/datacenter/dcgm/latest/gpu-telemetry/dcgm-exporter.html) и [инструкции по установке](https://developer.nvidia.com/dcgm).

### Сборка и запуск

```bash
go build ./...
go run .
```

Экспортер будет доступен по адресу:

```text
http://127.0.0.1:9990/metrics
```

### Сборка релизного бинарника

Так как проект использует CGO и DCGM, самый надежный вариант — собирать бинарник на том же семействе Linux-дистрибутивов, что и целевой сервер.

Собирайте на сервере или build-хосте, где установлены драйвер NVIDIA и development-библиотеки DCGM:

```bash
go build -trimpath -ldflags="-s -w -X main.version=0.5.0" -o gpu-exporter .
```

Проверить, с какими библиотеками слинкован бинарник:

```bash
ldd ./gpu-exporter
```

На целевом сервере должна быть доступна `libdcgm.so.4`. Путь зависит от дистрибутива, например:

```text
/usr/lib64/libdcgm.so.4
/usr/lib/x86_64-linux-gnu/libdcgm.so.4
```

Не стоит собирать бинарник на значительно более новой системе, чем целевая. CGO-бинарники линкуются с glibc, и бинарник, собранный на новой glibc, может не запуститься на сервере со старой.

### Структура проекта

| Файл | Ответственность |
| --- | --- |
| `main.go` | Точка входа приложения. Загружает конфигурацию, настраивает логирование, запускает DCGM, создает объекты metrics/exporter/server, обрабатывает сигналы завершения и корректно останавливает HTTP-сервер. |
| `config.go` | Читает переменные окружения и применяет значения по умолчанию для адреса, интервала сбора, порогов детекции запросов, режима DCGM и уровня логирования. |
| `dcgm_client.go` | Низкоуровневая интеграция с DCGM. Создаёт отдельные fast/profiling/operational/reliability watchers, читает всю историю через `GetValuesSince`, проверяет status/type и очищает DCGM-группы. |
| `metrics.go` | Описывает все экспортируемые `gpu_`-метрики и их лейблы, затем регистрирует их в отдельном registry. |
| `exporter.go` | Основной цикл сбора. Обновляет мгновенные метрики, availability, интегральные counters и публикует fixed aggregation windows независимо от HTTP-запросов. |
| `tracker.go` | Потокобезопасные хелперы: оконная агрегация max/avg и детекция активности GPU. |
| `server.go` | Read-only HTTP-слой для `/metrics`, `/health` и `/ready` с graceful shutdown. |

### Настройка

Настройка выполняется через переменные окружения.

| Переменная | Значение по умолчанию | Описание |
| --- | --- | --- |
| `GPU_EXPORTER_ADDR` | `127.0.0.1:9990` | Адрес HTTP-сервера. |
| `GPU_EXPORTER_SCRAPE_INTERVAL` | `100ms` | Интервал fast watcher: utilization, power, memory-copy и framebuffer ratio. |
| `GPU_EXPORTER_PROFILING_INTERVAL` | `500ms` | Интервал DCP profiling и PCIe fallback watchers. |
| `GPU_EXPORTER_OPERATIONAL_INTERVAL` | `1s` | Интервал clocks, temperatures, memory, limits и link state. |
| `GPU_EXPORTER_RELIABILITY_INTERVAL` | `10s` | Интервал ECC, XID, retired/remapped pages и violation counters. |
| `GPU_EXPORTER_WINDOW_INTERVAL` | `15s` | Независимое от HTTP окно публикации `_max`/`_avg`. |
| `GPU_EXPORTER_ACTIVE_THRESHOLD` | `1.0` | Порог GPU utilization (в процентах, неотрицательный) для определения активной работы. |
| `GPU_EXPORTER_MIN_REQUEST_TIME` | `50ms` | Минимальная длительность активного окна, которое считается запросом. |
| `GPU_EXPORTER_DCGM_MODE` | `embedded` | Режим DCGM: `embedded`, `standalone` или `start-hostengine`. |
| `GPU_EXPORTER_LOG_LEVEL` | `info` | Уровень логирования: `debug`, `info`, `warn` или `error`. |

Пример:

```bash
GPU_EXPORTER_ADDR=0.0.0.0:9990 GPU_EXPORTER_LOG_LEVEL=debug go run .
```

### Метрики

Примеры экспортируемых метрик:

- Utilization/activity: `gpu_utilization_percent`, `gpu_utilization_percent_current`, `gpu_memory_copy_utilization_percent`, `gpu_encoder_utilization_percent`, `gpu_decoder_utilization_percent`, `gpu_activity_windows_total`, `gpu_active_seconds_total`, `gpu_utilization_weighted_seconds_total`.
- Memory/temperature: `gpu_memory_free_bytes`, `gpu_memory_used_bytes`, `gpu_memory_total_bytes`, `gpu_framebuffer_memory_reserved_bytes`, `gpu_bar1_memory_free_bytes`, `gpu_bar1_memory_used_bytes`, `gpu_bar1_memory_total_bytes`, `gpu_framebuffer_memory_used_percent`, `gpu_temperature_celsius`, `gpu_temperature_max_operating_celsius`, `gpu_memory_temperature_celsius`, `gpu_memory_temperature_max_operating_celsius`.
- Power/clocks: `gpu_power_draw_watts`, `gpu_power_draw_instant_watts`, `gpu_power_limit_watts`, `gpu_power_enforced_limit_watts`, `gpu_energy_joules_total`, `gpu_energy_estimated_joules_total`, `gpu_sm_clock_hertz`, `gpu_memory_clock_hertz`, `gpu_performance_state`, `gpu_fan_speed_percent`, `gpu_clock_throttle_reasons`, `gpu_clock_event_active`, `gpu_clock_violation_seconds_total`.
- PCIe/NVLink/reliability: `gpu_pcie_transmit_bytes_per_second`, `gpu_pcie_receive_bytes_per_second`, `gpu_pcie_transmitted_bytes_total`, `gpu_pcie_received_bytes_total`, `gpu_pcie_link_generation`, `gpu_pcie_link_width`, `gpu_pcie_replay_total`, `gpu_nvlink_transmit_bytes_per_second`, `gpu_nvlink_receive_bytes_per_second`, `gpu_nvlink_transmitted_bytes_total`, `gpu_nvlink_received_bytes_total`, `gpu_xid_last_code`, `gpu_ecc_errors_total`, `gpu_retired_pages_total`, `gpu_remapped_rows_total`.
- Profiling: `gpu_prof_*_ratio`, соответствующие `*_max`/`*_avg`, `*_weighted_seconds_total` и `*_observed_seconds_total`. Все ratio имеют строгий диапазон `0–1`.
- Availability/health: `gpu_dcgm_field_supported`, `gpu_dcgm_field_available`, `gpu_dcgm_field_last_success_timestamp_seconds`, `gpu_exporter_collect_success`, `gpu_exporter_discovered_gpus`, `gpu_exporter_collected_gpus`, `gpu_exporter_failed_gpus`, `gpu_exporter_gpu_collect_success`.

Основные GPU-метрики размечаются лейблами `gpu_index`, `gpu_uuid`, `pci_bus_id`, `gpu_name` и `hostname`. Для стабильной идентификации GPU между перезагрузками используйте `gpu_uuid` или `pci_bus_id`, а не только `gpu_index`.

`/health` проверяет, что HTTP-процесс жив. `/ready` возвращает `200`, только если последний свежий сбор завершился полностью для всех обнаруженных GPU; partial collect возвращает `503`.

`gpu_activity_windows_total` считает выведенные окна активности GPU по переходам utilization через порог. Старое имя `gpu_request_count_total` оставлено как compatibility alias и не означает реальные application requests.

Быстро меняющиеся метрики имеют варианты `_max` и `_avg`. Они считаются по всем значениям DCGM из `GetValuesSince` в фиксированном окне `GPU_EXPORTER_WINDOW_INTERVAL`; `/metrics` только читает registry и не сбрасывает окно. `gpu_utilization_percent_current` показывает последнее валидное значение, а историческое `gpu_utilization_percent` — максимум завершённого окна.

Для средней utilization при наличии пропусков используйте `increase(gpu_utilization_weighted_seconds_total[$__range]) / increase(gpu_utilization_observed_seconds_total[$__range]) * 100`. Аналогичные `observed_seconds_total` являются корректным denominator для DCP ratios. Rate-поля дополнительно интегрируются в `gpu_pcie_*_bytes_total` и `gpu_nvlink_*_bytes_total`.

`gpu_energy_joules_total` использует аппаратный DCGM-счетчик `DCGM_FI_DEV_TOTAL_ENERGY_CONSUMPTION`, когда он доступен. `gpu_energy_estimated_joules_total` остается оценкой по формуле `power_watts * elapsed_seconds`.

Профилирующие метрики (`gpu_prof_*`) требуют GPU с поддержкой DCP (Volta и новее). Экспортер выбирает совместимый набор profiling-полей из supported metric groups; если DCP недоступен, он откатывается на базовые поля, и эти серии отсутствуют.

GPU-level NVLink публикуется только из DCP-полей. Per-link PLR и NVSwitch fields намеренно не читаются через GPU entity; для них нужен отдельный link/NVSwitch collector.

Если DCGM не отдаёт значение, Gauge сохраняет последнее значение, но `gpu_dcgm_field_available` становится `0`, а timestamp показывает возраст последнего валидного сэмпла. Поддерживаемый hardware counter со значением ноль публикуется как нулевая серия; unsupported field отличается через `gpu_dcgm_field_supported=0`.

### Примеры

В каталоге [`examples/`](examples/) лежат готовые к адаптации конфигурации:

- [`examples/alloy/config.alloy`](examples/alloy/config.alloy) — конфигурация скрейпа для Grafana Alloy с интервалом 5с.
- [`examples/grafana/`](examples/grafana/) — единый Grafana dashboard для live-мониторинга GPU и отчётных KPI с CSV-выгрузкой.
- [`examples/systemd/gpu-exporter.service`](examples/systemd/gpu-exporter.service) — systemd-юнит для запуска на хосте; требует `libdcgm.so.4` на хосте.
- [`examples/docker/Dockerfile`](examples/docker/Dockerfile) — контейнерный образ с DCGM внутри; на хосте нужны только драйвер NVIDIA и NVIDIA Container Toolkit.
- [`examples/docker/compose.yaml`](examples/docker/compose.yaml) — запуск того же образа через Docker Compose, включая вариант с готовым образом для окружений без доступа в Интернет.
- [`examples/docker/compose.stack.yaml`](examples/docker/compose.stack.yaml) — полный локальный стек: exporter, Prometheus, Grafana и Grafana MCP.
- [`examples/docker/WINDOWS.md`](examples/docker/WINDOWS.md) — пошаговый запуск полного стека в Windows через Docker Desktop.

### Docker-образ для офлайн-установки

Релиз `0.5.0` для закрытого контура состоит из двух архивов:

- `distr/gpu-exporter-image-0.5.0-cuda12.tar.gz`
- `distr/gpu-exporter-image-0.5.0-cuda13.tar.gz`

Файл `distr/SHA256SUMS` содержит контрольные суммы обоих архивов. Каталог `distr/` исключён из Git и Docker build context.

Единственное намеренное отличие между ними — runtime-пакет DCGM: `datacenter-gpu-manager-4-cuda12` или `datacenter-gpu-manager-4-cuda13`. Оба варианта ставятся с recommended-пакетами. Это важно для DCGM-модулей, которые не входят в open-source часть DCGM; без них DCGM может отвечать ошибкой вида `This request is serviced by a module of DCGM that is not currently loaded`.

На машине с Интернетом и Docker соберите оба архива одной командой:

```bash
./scripts/build-offline-images.sh
```

На закрытом сервере ничего скачивать или устанавливать не нужно. Перенесите нужный архив и загрузите образ:

```bash
docker load -i gpu-exporter-image-0.5.0-cuda12.tar.gz
docker run -d --name gpu-exporter --restart unless-stopped \
  --gpus all --cap-add SYS_ADMIN \
  -p 127.0.0.1:9990:9990 \
  gpu-exporter:0.5.0-cuda12
```

Для хоста с драйвером `570.133.20` и CUDA `12.8` нужен образ `cuda12`. Для хостов, где `nvidia-smi` показывает CUDA `13.x`, собирайте и загружайте образ `cuda13`. DCGM на хосте в Docker-сценарии не обязателен: экспортер по умолчанию использует embedded DCGM внутри контейнера, а NVIDIA Container Toolkit прокидывает драйверные библиотеки с хоста.

### Разработка

```bash
go build ./...
go test ./...
go vet ./...
go fmt ./...
```

---

## English

GPU Exporter is a local exporter for NVIDIA GPU metrics. It is written in Go and uses NVIDIA DCGM through [`github.com/NVIDIA/go-dcgm`](https://github.com/NVIDIA/go-dcgm).

The exporter is designed to run locally and does not require Internet access at runtime. Internet access is only needed when downloading Go modules or installing system packages.

### Features

- NVIDIA GPU metrics exposed through a local `/metrics` endpoint.
- DCGM-based collection via `go-dcgm`.
- Local HTTP endpoint on `127.0.0.1:9990` by default.
- Separate fast, profiling, operational, and reliability intervals; history is read with `GetValuesSince`, and `_max`/`_avg` use actual DCGM samples.
- Structured JSON logs with configurable log level.
- Metrics use the `gpu_` prefix.

### Requirements

- Go `1.26.4` or newer
- NVIDIA driver
- NVIDIA DCGM runtime and development libraries
- Linux with access to the NVIDIA GPU

DCGM packages are distributed through NVIDIA repositories. Package names depend on the distribution and CUDA version — see the [DCGM documentation](https://docs.nvidia.com/datacenter/dcgm/latest/gpu-telemetry/dcgm-exporter.html) and [installation instructions](https://developer.nvidia.com/dcgm).

### Build and Run

```bash
go build ./...
go run .
```

The exporter starts on:

```text
http://127.0.0.1:9990/metrics
```

### Building a Release Binary

Because this project uses CGO and DCGM, the safest option is to build on the same Linux distribution family as the target server.

Build on a server or build host with the NVIDIA driver and DCGM development libraries installed:

```bash
go build -trimpath -ldflags="-s -w -X main.version=0.5.0" -o gpu-exporter .
```

Check runtime library links:

```bash
ldd ./gpu-exporter
```

The target server must have `libdcgm.so.4` available. The path depends on the distribution, for example:

```text
/usr/lib64/libdcgm.so.4
/usr/lib/x86_64-linux-gnu/libdcgm.so.4
```

Avoid building on a much newer Linux system than the target. CGO binaries link against glibc, and a binary built on a newer glibc may not run on a server with an older one.

### Project Structure

| File | Responsibility |
| --- | --- |
| `main.go` | Application entry point. Loads configuration, initializes logging, starts DCGM, creates metrics/exporter/server objects, handles shutdown signals, and stops the HTTP server gracefully. |
| `config.go` | Reads environment variables and applies defaults for listen address, collection interval, request detection thresholds, DCGM mode, and log level. |
| `dcgm_client.go` | Low-level DCGM integration. Creates separate fast/profiling/operational/reliability watchers, reads history with `GetValuesSince`, validates status/type, and cleans up DCGM groups. |
| `metrics.go` | Defines all exported `gpu_` metrics and their labels, then registers them in a dedicated registry. |
| `exporter.go` | Main collection loop. Updates current metrics, availability and integral counters, and publishes fixed aggregation windows independently of HTTP requests. |
| `tracker.go` | Thread-safe max/avg window aggregation and GPU activity detection. |
| `server.go` | Read-only HTTP layer for `/metrics`, `/health`, and `/ready`, with graceful shutdown. |

### Configuration

Configuration is done through environment variables.

| Variable | Default | Description |
| --- | --- | --- |
| `GPU_EXPORTER_ADDR` | `127.0.0.1:9990` | HTTP listen address. |
| `GPU_EXPORTER_SCRAPE_INTERVAL` | `100ms` | Fast watcher interval for utilization, power, memory-copy, and framebuffer ratio. |
| `GPU_EXPORTER_PROFILING_INTERVAL` | `500ms` | DCP profiling and PCIe fallback watcher interval. |
| `GPU_EXPORTER_OPERATIONAL_INTERVAL` | `1s` | Clocks, temperatures, memory, limits, and link-state interval. |
| `GPU_EXPORTER_RELIABILITY_INTERVAL` | `10s` | ECC, XID, retired/remapped pages, and violation-counter interval. |
| `GPU_EXPORTER_WINDOW_INTERVAL` | `15s` | HTTP-independent `_max`/`_avg` publication window. |
| `GPU_EXPORTER_ACTIVE_THRESHOLD` | `1.0` | GPU utilization threshold (percent, non-negative) used to infer request activity. |
| `GPU_EXPORTER_MIN_REQUEST_TIME` | `50ms` | Minimum active window duration counted as a request. |
| `GPU_EXPORTER_DCGM_MODE` | `embedded` | DCGM mode: `embedded`, `standalone`, or `start-hostengine`. |
| `GPU_EXPORTER_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, or `error`. |

Example:

```bash
GPU_EXPORTER_ADDR=0.0.0.0:9990 GPU_EXPORTER_LOG_LEVEL=debug go run .
```

### Metrics

Examples of exported metrics:

- Utilization/activity: `gpu_utilization_percent`, `gpu_utilization_percent_current`, `gpu_memory_copy_utilization_percent`, `gpu_encoder_utilization_percent`, `gpu_decoder_utilization_percent`, `gpu_activity_windows_total`, `gpu_active_seconds_total`, `gpu_utilization_weighted_seconds_total`.
- Memory/temperature: `gpu_memory_free_bytes`, `gpu_memory_used_bytes`, `gpu_memory_total_bytes`, `gpu_framebuffer_memory_reserved_bytes`, `gpu_bar1_memory_free_bytes`, `gpu_bar1_memory_used_bytes`, `gpu_bar1_memory_total_bytes`, `gpu_framebuffer_memory_used_percent`, `gpu_temperature_celsius`, `gpu_temperature_max_operating_celsius`, `gpu_memory_temperature_celsius`, `gpu_memory_temperature_max_operating_celsius`.
- Power/clocks: `gpu_power_draw_watts`, `gpu_power_draw_instant_watts`, `gpu_power_limit_watts`, `gpu_power_enforced_limit_watts`, `gpu_energy_joules_total`, `gpu_energy_estimated_joules_total`, `gpu_sm_clock_hertz`, `gpu_memory_clock_hertz`, `gpu_performance_state`, `gpu_fan_speed_percent`, `gpu_clock_throttle_reasons`, `gpu_clock_event_active`, `gpu_clock_violation_seconds_total`.
- PCIe/NVLink/reliability: `gpu_pcie_transmit_bytes_per_second`, `gpu_pcie_receive_bytes_per_second`, `gpu_pcie_transmitted_bytes_total`, `gpu_pcie_received_bytes_total`, `gpu_pcie_link_generation`, `gpu_pcie_link_width`, `gpu_pcie_replay_total`, `gpu_nvlink_transmit_bytes_per_second`, `gpu_nvlink_receive_bytes_per_second`, `gpu_nvlink_transmitted_bytes_total`, `gpu_nvlink_received_bytes_total`, `gpu_xid_last_code`, `gpu_ecc_errors_total`, `gpu_retired_pages_total`, and `gpu_remapped_rows_total`.
- Profiling: `gpu_prof_*_ratio` with matching `*_max`/`*_avg`, `*_weighted_seconds_total`, and `*_observed_seconds_total`. Every ratio is strictly in the `0-1` range.
- Availability/health: `gpu_dcgm_field_supported`, `gpu_dcgm_field_available`, `gpu_dcgm_field_last_success_timestamp_seconds`, `gpu_exporter_collect_success`, `gpu_exporter_discovered_gpus`, `gpu_exporter_collected_gpus`, `gpu_exporter_failed_gpus`, and `gpu_exporter_gpu_collect_success`.

Core GPU metrics are labeled with `gpu_index`, `gpu_uuid`, `pci_bus_id`, `gpu_name`, and `hostname`. Use `gpu_uuid` or `pci_bus_id`, rather than `gpu_index` alone, when you need stable GPU identity across reboots.

`/health` only checks that the HTTP process is alive. `/ready` returns `200` only when the latest recent collection completed for every discovered GPU; a partial collection returns `503`.

`gpu_activity_windows_total` counts inferred GPU activity windows from utilization threshold crossings. The old `gpu_request_count_total` name remains as a compatibility alias and does not represent real application requests.

Fast-changing metrics have `_max` and `_avg` variants computed from every DCGM value returned by `GetValuesSince` in the fixed `GPU_EXPORTER_WINDOW_INTERVAL`. `/metrics` only reads the registry and never resets the window. `gpu_utilization_percent_current` is the latest valid value; historical `gpu_utilization_percent` is the completed-window maximum.

For average utilization with data gaps, use `increase(gpu_utilization_weighted_seconds_total[$__range]) / increase(gpu_utilization_observed_seconds_total[$__range]) * 100`. Matching `observed_seconds_total` metrics are the correct denominator for DCP ratios. Rate fields are also integrated into `gpu_pcie_*_bytes_total` and `gpu_nvlink_*_bytes_total`.

`gpu_energy_joules_total` uses the `DCGM_FI_DEV_TOTAL_ENERGY_CONSUMPTION` hardware counter when available. `gpu_energy_estimated_joules_total` remains the `power_watts * elapsed_seconds` estimate.

Profiling metrics (`gpu_prof_*`) require a GPU with DCP support (Volta or newer). The exporter selects a compatible profiling field set from supported metric groups; if DCP is unavailable, it falls back to basic fields and these series are absent.

GPU-level NVLink metrics come only from DCP fields. Per-link PLR and NVSwitch fields are deliberately not read through a GPU entity; they require a separate link/NVSwitch collector.

When DCGM stops reporting a value, the Gauge keeps its last value while `gpu_dcgm_field_available` becomes `0`; the timestamp exposes the age of the last valid sample. A supported zero-valued hardware counter is emitted as a zero series, while unsupported fields are explicit through `gpu_dcgm_field_supported=0`.

### Examples

The [`examples/`](examples/) directory contains ready-to-adapt configs:

- [`examples/alloy/config.alloy`](examples/alloy/config.alloy) — Grafana Alloy scrape config with a 5s interval.
- [`examples/grafana/`](examples/grafana/) — ready-to-import Grafana dashboard for live GPU monitoring and report KPI CSV exports.
- [`examples/systemd/gpu-exporter.service`](examples/systemd/gpu-exporter.service) — systemd unit for bare-metal installs; requires `libdcgm.so.4` on the host.
- [`examples/docker/Dockerfile`](examples/docker/Dockerfile) — container image with DCGM bundled inside; the host only needs the NVIDIA driver and the NVIDIA Container Toolkit.
- [`examples/docker/compose.yaml`](examples/docker/compose.yaml) — the same image run via Docker Compose, including a prebuilt-image option for air-gapped environments.
- [`examples/docker/compose.stack.yaml`](examples/docker/compose.stack.yaml) — complete local stack with the exporter, Prometheus, Grafana, and Grafana MCP.
- [`examples/docker/WINDOWS.md`](examples/docker/WINDOWS.md) — Windows Docker Desktop walkthrough for the complete stack.

### Docker Image for Air-Gapped Installation

The `0.5.0` air-gapped release consists of two archives:

- `distr/gpu-exporter-image-0.5.0-cuda12.tar.gz`
- `distr/gpu-exporter-image-0.5.0-cuda13.tar.gz`

`distr/SHA256SUMS` contains checksums for both archives. The entire `distr/` directory is excluded from Git and the Docker build context.

The only intentional difference between them is the DCGM runtime package: `datacenter-gpu-manager-4-cuda12` or `datacenter-gpu-manager-4-cuda13`. Both variants are installed with recommended packages. This matters for DCGM modules that are not part of the open-source DCGM package set; without them DCGM can return errors such as `This request is serviced by a module of DCGM that is not currently loaded`.

On an Internet-connected Docker build machine, create both archives with:

```bash
./scripts/build-offline-images.sh
```

On the air-gapped host, do not download or install anything. Transfer the matching archive and load the image:

```bash
docker load -i gpu-exporter-image-0.5.0-cuda12.tar.gz
docker run -d --name gpu-exporter --restart unless-stopped \
  --gpus all --cap-add SYS_ADMIN \
  -p 127.0.0.1:9990:9990 \
  gpu-exporter:0.5.0-cuda12
```

For a host with driver `570.133.20` and CUDA `12.8`, use the `cuda12` image. For hosts where `nvidia-smi` reports CUDA `13.x`, build and load the `cuda13` image. Host-side DCGM is not required for the Docker path: by default, the exporter uses embedded DCGM inside the container, while NVIDIA Container Toolkit mounts the host driver libraries into the container.

### Development

```bash
go build ./...
go test ./...
go vet ./...
go fmt ./...
```
