# GPU Exporter

Language: [Русский](#русский) | [English](#english)

## Русский

GPU Exporter — это локальный экспортер метрик NVIDIA GPU. Приложение написано на Go и использует NVIDIA DCGM через [`github.com/NVIDIA/go-dcgm`](https://github.com/NVIDIA/go-dcgm).

Экспортер рассчитан на локальную работу и не требует доступа к Интернету во время работы. Интернет нужен только для скачивания Go-модулей или установки системных пакетов.

### Возможности

- Метрики NVIDIA GPU через локальный endpoint `/metrics`.
- Сбор метрик через DCGM и `go-dcgm`.
- Локальный HTTP endpoint `127.0.0.1:9990` по умолчанию.
- Быстрый внутренний сбор метрик (по умолчанию каждые 100мс) с агрегацией пиков (`_max`) и средних (`_avg`) между опросами внешнего агента (Prometheus, Grafana Alloy и т.п.).
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
go build -trimpath -ldflags="-s -w" -o gpu-exporter .
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
| `dcgm_client.go` | Низкоуровневая интеграция с DCGM. Инициализирует DCGM, находит GPU, кеширует статическую информацию о GPU, создает постоянные DCGM field watchers, читает сэмплы, конвертирует значения DCGM и очищает DCGM-группы при завершении. |
| `metrics.go` | Описывает все экспортируемые `gpu_`-метрики и их лейблы, затем регистрирует их в отдельном registry. |
| `exporter.go` | Основной цикл сбора. Периодически читает сэмплы DCGM, обновляет мгновенные метрики, копит оконную статистику, считает выведенные GPU-запросы и публикует оконные метрики при скрейпе `/metrics`. |
| `tracker.go` | Потокобезопасные хелперы: оконная агрегация (max/avg между скрейпами) и детекция активности/запросов GPU. |
| `server.go` | HTTP-слой. Экспортирует `/metrics` и `/health`, публикует оконную статистику перед отдачей метрик и поддерживает graceful shutdown. |

### Настройка

Настройка выполняется через переменные окружения.

| Переменная | Значение по умолчанию | Описание |
| --- | --- | --- |
| `GPU_EXPORTER_ADDR` | `127.0.0.1:9990` | Адрес HTTP-сервера. |
| `GPU_EXPORTER_SCRAPE_INTERVAL` | `100ms` | Внутренний интервал сбора метрик DCGM. Также используется как частота обновления DCGM field watch. |
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

- `gpu_utilization_percent`
- `gpu_memory_copy_utilization_percent`
- `gpu_framebuffer_memory_used_percent`
- `gpu_clock_throttle_reasons`
- `gpu_prof_sm_active_ratio`
- `gpu_prof_dram_active_ratio`
- `gpu_prof_pipe_tensor_active_ratio`
- `gpu_power_draw_watts`
- `gpu_temperature_celsius`
- `gpu_memory_used_bytes`
- `gpu_request_count_total`
- `gpu_active_seconds_total`
- `gpu_utilization_weighted_seconds_total`
- `gpu_tensor_active_weighted_seconds_total`
- `gpu_energy_joules_total`

Быстро меняющиеся метрики имеют варианты `_max` (пик) и `_avg` (среднее). Они считаются по внутренним 100мс-сэмплам, накопленным между внешними скрейпами, поэтому короткие всплески нагрузки не теряются даже при интервале опроса 5–15с. Сама `gpu_utilization_percent` — тоже пиковое значение (историческое имя).

Интегральные counter-метрики (`gpu_active_seconds_total`, `gpu_utilization_weighted_seconds_total`, `gpu_sm_active_weighted_seconds_total`, `gpu_dram_active_weighted_seconds_total`, `gpu_tensor_active_weighted_seconds_total`, `gpu_energy_joules_total`) предназначены для отчётов за период. Например, active hours считаются как `increase(gpu_active_seconds_total[$__range]) / 3600`, эквивалентные GPU-часы при 100% utilization — как `increase(gpu_utilization_weighted_seconds_total[$__range]) / 3600`, а энергия в kWh — как `increase(gpu_energy_joules_total[$__range]) / 3600000`.

**Важно:** окно агрегации сбрасывается при каждом запросе `/metrics`, поэтому экспортер должен опрашивать ровно один скрейпер. Второй скрейпер (HA-пара Prometheus, ручной `curl` при отладке) молча украдёт окно у первого.

Профилирующие метрики (`gpu_prof_*`) требуют GPU с поддержкой DCP (Volta и новее). На неподдерживаемых GPU экспортер автоматически откатывается на базовые поля, и эти серии отсутствуют.

Если DCGM не отдаёт значение (blank-поле), соответствующая серия просто не обновляется, а не выставляется в ноль — так отсутствие данных можно отличить от честного нуля.

### Примеры

В каталоге [`examples/`](examples/) лежат готовые к адаптации конфигурации:

- [`examples/alloy/config.alloy`](examples/alloy/config.alloy) — конфигурация скрейпа для Grafana Alloy (интервал 5с, ровно один скрейпер).
- [`examples/grafana/`](examples/grafana/) — единый Grafana dashboard для live-мониторинга GPU и отчётных KPI с CSV-выгрузкой.
- [`examples/systemd/gpu-exporter.service`](examples/systemd/gpu-exporter.service) — systemd-юнит для запуска на хосте; требует `libdcgm.so.4` на хосте.
- [`examples/docker/Dockerfile`](examples/docker/Dockerfile) — контейнерный образ с DCGM внутри; на хосте нужны только драйвер NVIDIA и NVIDIA Container Toolkit.
- [`examples/docker/compose.yaml`](examples/docker/compose.yaml) — запуск того же образа через Docker Compose, включая вариант с готовым образом для окружений без доступа в Интернет.

### Docker-образ для офлайн-установки

Релиз `0.4.0` для закрытого контура состоит из двух архивов:

- `dist/gpu-exporter-image-0.4.0-cuda12.tar.gz`
- `dist/gpu-exporter-image-0.4.0-cuda13.tar.gz`

Единственное намеренное отличие между ними — runtime-пакет DCGM: `datacenter-gpu-manager-4-cuda12` или `datacenter-gpu-manager-4-cuda13`. Оба варианта ставятся с recommended-пакетами. Это важно для DCGM-модулей, которые не входят в open-source часть DCGM; без них DCGM может отвечать ошибкой вида `This request is serviced by a module of DCGM that is not currently loaded`.

На машине с Интернетом и Docker соберите оба архива одной командой:

```bash
./scripts/build-offline-images.sh
```

На закрытом сервере ничего скачивать или устанавливать не нужно. Перенесите нужный архив и загрузите образ:

```bash
docker load -i gpu-exporter-image-0.4.0-cuda12.tar.gz
docker run -d --name gpu-exporter --restart unless-stopped \
  --gpus all --cap-add SYS_ADMIN \
  -p 127.0.0.1:9990:9990 \
  gpu-exporter:0.4.0-cuda12
```

Для хоста с драйвером `570.133.20` и CUDA `12.8` нужен образ `cuda12`. Для хостов, где `nvidia-smi` показывает CUDA `13.x`, собирайте и загружайте образ `cuda13`. DCGM на хосте в Docker-сценарии не обязателен: экспортер по умолчанию использует embedded DCGM внутри контейнера, а NVIDIA Container Toolkit прокидывает драйверные библиотеки с хоста.

### Разработка

```bash
go build ./...
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
- Fast internal sampling (100ms by default) with peak (`_max`) and average (`_avg`) aggregation between scrapes of an external agent (Prometheus, Grafana Alloy, etc.).
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
go build -trimpath -ldflags="-s -w" -o gpu-exporter .
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
| `dcgm_client.go` | Low-level DCGM integration. Initializes DCGM, discovers GPUs, caches static GPU identity, creates persistent DCGM field watchers, reads GPU samples, converts DCGM values, and cleans up DCGM groups on shutdown. |
| `metrics.go` | Defines all exported `gpu_` metrics and their labels, then registers them in a dedicated registry. |
| `exporter.go` | Main collection loop. Periodically reads DCGM samples, updates current metrics, accumulates per-window statistics, counts inferred GPU work requests, and flushes window metrics when `/metrics` is scraped. |
| `tracker.go` | Thread-safe helpers: window aggregation (max/avg between scrapes) and GPU activity/request detection. |
| `server.go` | HTTP layer. Exposes `/metrics` and `/health`, flushes window statistics before serving metrics, and supports graceful shutdown. |

### Configuration

Configuration is done through environment variables.

| Variable | Default | Description |
| --- | --- | --- |
| `GPU_EXPORTER_ADDR` | `127.0.0.1:9990` | HTTP listen address. |
| `GPU_EXPORTER_SCRAPE_INTERVAL` | `100ms` | Internal DCGM sampling interval. Also used as the DCGM field watch update frequency. |
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

- `gpu_utilization_percent`
- `gpu_memory_copy_utilization_percent`
- `gpu_framebuffer_memory_used_percent`
- `gpu_clock_throttle_reasons`
- `gpu_prof_sm_active_ratio`
- `gpu_prof_dram_active_ratio`
- `gpu_prof_pipe_tensor_active_ratio`
- `gpu_power_draw_watts`
- `gpu_temperature_celsius`
- `gpu_memory_used_bytes`
- `gpu_request_count_total`
- `gpu_active_seconds_total`
- `gpu_utilization_weighted_seconds_total`
- `gpu_tensor_active_weighted_seconds_total`
- `gpu_energy_joules_total`

Fast-changing metrics also have `_max` (peak) and `_avg` (average) variants. They are computed over the exporter's internal 100ms samples collected between external scrapes, so short load spikes are not lost even with a 5–15s scrape interval. Note that `gpu_utilization_percent` itself is a peak value (historical naming).

Integral counter metrics (`gpu_active_seconds_total`, `gpu_utilization_weighted_seconds_total`, `gpu_sm_active_weighted_seconds_total`, `gpu_dram_active_weighted_seconds_total`, `gpu_tensor_active_weighted_seconds_total`, `gpu_energy_joules_total`) are meant for period reports. For example, active hours are `increase(gpu_active_seconds_total[$__range]) / 3600`, equivalent 100%-utilization GPU-hours are `increase(gpu_utilization_weighted_seconds_total[$__range]) / 3600`, and energy in kWh is `increase(gpu_energy_joules_total[$__range]) / 3600000`.

**Important:** the aggregation window is reset on every `/metrics` request, so exactly one scraper must poll the exporter. A second scraper (an HA Prometheus pair, manual `curl` during debugging) would silently steal the window from the first one.

Profiling metrics (`gpu_prof_*`) require a GPU with DCP support (Volta or newer). On unsupported GPUs the exporter automatically falls back to basic fields and these series are absent.

If DCGM does not report a value (a blank field), the corresponding series is simply not updated instead of being set to zero, so missing data is distinguishable from a true zero.

### Examples

The [`examples/`](examples/) directory contains ready-to-adapt configs:

- [`examples/alloy/config.alloy`](examples/alloy/config.alloy) — Grafana Alloy scrape config (5s interval, exactly one scraper).
- [`examples/grafana/`](examples/grafana/) — ready-to-import Grafana dashboard for live GPU monitoring and report KPI CSV exports.
- [`examples/systemd/gpu-exporter.service`](examples/systemd/gpu-exporter.service) — systemd unit for bare-metal installs; requires `libdcgm.so.4` on the host.
- [`examples/docker/Dockerfile`](examples/docker/Dockerfile) — container image with DCGM bundled inside; the host only needs the NVIDIA driver and the NVIDIA Container Toolkit.
- [`examples/docker/compose.yaml`](examples/docker/compose.yaml) — the same image run via Docker Compose, including a prebuilt-image option for air-gapped environments.

### Docker Image for Air-Gapped Installation

The `0.4.0` air-gapped release consists of two archives:

- `dist/gpu-exporter-image-0.4.0-cuda12.tar.gz`
- `dist/gpu-exporter-image-0.4.0-cuda13.tar.gz`

The only intentional difference between them is the DCGM runtime package: `datacenter-gpu-manager-4-cuda12` or `datacenter-gpu-manager-4-cuda13`. Both variants are installed with recommended packages. This matters for DCGM modules that are not part of the open-source DCGM package set; without them DCGM can return errors such as `This request is serviced by a module of DCGM that is not currently loaded`.

On an Internet-connected Docker build machine, create both archives with:

```bash
./scripts/build-offline-images.sh
```

On the air-gapped host, do not download or install anything. Transfer the matching archive and load the image:

```bash
docker load -i gpu-exporter-image-0.4.0-cuda12.tar.gz
docker run -d --name gpu-exporter --restart unless-stopped \
  --gpus all --cap-add SYS_ADMIN \
  -p 127.0.0.1:9990:9990 \
  gpu-exporter:0.4.0-cuda12
```

For a host with driver `570.133.20` and CUDA `12.8`, use the `cuda12` image. For hosts where `nvidia-smi` reports CUDA `13.x`, build and load the `cuda13` image. Host-side DCGM is not required for the Docker path: by default, the exporter uses embedded DCGM inside the container, while NVIDIA Container Toolkit mounts the host driver libraries into the container.

### Development

```bash
go build ./...
go vet ./...
go fmt ./...
```
