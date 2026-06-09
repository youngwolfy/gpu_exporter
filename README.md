# GPU Exporter

Language: [English](#english) | [Русский](#русский)

## English

GPU Exporter is a local exporter for NVIDIA GPU metrics. It is written in Go and uses NVIDIA DCGM through [`github.com/NVIDIA/go-dcgm`](https://github.com/NVIDIA/go-dcgm).

The exporter is designed to run locally and does not require Internet access at runtime. Internet access is only needed when downloading Go modules or installing system packages.

## Features

- NVIDIA GPU metrics exposed through a local `/metrics` endpoint.
- DCGM-based collection via `go-dcgm`.
- Local HTTP endpoint on `127.0.0.1:9990` by default.
- Fast internal sampling with peak aggregation between external scrapes.
- Structured JSON logs with configurable log level.
- Metrics use the `gpu_` prefix.

## Requirements

- Go `1.26.4`
- NVIDIA driver
- NVIDIA DCGM runtime and development libraries
- Linux or WSL with access to the NVIDIA GPU

On Ubuntu/WSL, DCGM packages are provided by NVIDIA repositories. Package names can differ by Ubuntu/CUDA version, for example:

```bash
sudo apt install --install-recommends datacenter-gpu-manager-4-cuda12 datacenter-gpu-manager-4-dev
```

## Build

```bash
go build ./...
```

## Run

```bash
go run .
```

The exporter starts on:

```text
http://127.0.0.1:9990/metrics
```

## Build a Binary

Because this project uses CGO and DCGM, the safest option is to build on the same Linux distribution family as the target server.

For RHEL 7 or RHEL 8 servers, build directly on a compatible RHEL server or build host with NVIDIA driver and DCGM development libraries installed:

```bash
go build -trimpath -ldflags="-s -w" -o gpu-exporter .
```

Check runtime library links:

```bash
ldd ./gpu-exporter
```

The target server must have `libdcgm.so.4` available, commonly under:

```text
/usr/lib64/libdcgm.so.4
```

Avoid building on a much newer Linux system for an older target such as RHEL 7. CGO binaries link against glibc, and a binary built on a newer glibc may not run on an older server.

## Project Structure

| File | Responsibility |
| --- | --- |
| `main.go` | Application entry point. Loads configuration, initializes logging, starts DCGM, creates metrics/exporter/server objects, handles shutdown signals, and stops the HTTP server gracefully. |
| `config.go` | Reads environment variables and applies defaults for listen address, collection interval, request detection thresholds, DCGM mode, and log level. |
| `dcgm_client.go` | Low-level DCGM integration. Initializes DCGM, discovers GPUs, caches static GPU identity, creates persistent DCGM field watchers, reads GPU samples, converts DCGM values, and cleans up DCGM groups on shutdown. |
| `metrics.go` | Defines all exported `gpu_` metrics and their labels, then registers them in a dedicated registry. |
| `exporter.go` | Main collection loop. Periodically reads DCGM samples, updates current metrics, tracks peak values, counts inferred GPU work requests, and flushes peak metrics when `/metrics` is scraped. |
| `tracker.go` | Small thread-safe helpers for peak aggregation and GPU activity/request detection. |
| `server.go` | HTTP layer. Exposes `/metrics` and `/health`, flushes peak values before serving metrics, and supports graceful shutdown. |

## Configuration

Configuration is done through environment variables.

| Variable | Default | Description |
| --- | --- | --- |
| `GPU_EXPORTER_ADDR` | `127.0.0.1:9990` | HTTP listen address. |
| `GPU_EXPORTER_SCRAPE_INTERVAL` | `100ms` | Internal DCGM sampling interval. |
| `GPU_EXPORTER_ACTIVE_THRESHOLD` | `1.0` | GPU utilization threshold used to infer request activity. |
| `GPU_EXPORTER_MIN_REQUEST_TIME` | `50ms` | Minimum active window duration counted as a request. |
| `GPU_EXPORTER_DCGM_MODE` | `embedded` | DCGM mode. |
| `GPU_EXPORTER_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, or `error`. |

Example:

```bash
GPU_EXPORTER_ADDR=0.0.0.0:9990 GPU_EXPORTER_LOG_LEVEL=debug go run .
```

## Metrics

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

Some metrics also have `_max` variants. These represent peak values observed inside the exporter's internal sampling window between external scrapes.

## Development

```bash
go test ./...
go fmt ./...
```

---

## Русский

GPU Exporter - это локальный exporter для метрик NVIDIA GPU. Приложение написано на Go и использует NVIDIA DCGM через [`github.com/NVIDIA/go-dcgm`](https://github.com/NVIDIA/go-dcgm).

Exporter рассчитан на локальную работу и не требует доступа к Интернету во время запуска. Интернет нужен только для скачивания Go-модулей или установки системных пакетов.

## Возможности

- Метрики NVIDIA GPU через локальный endpoint `/metrics`.
- Сбор метрик через DCGM и `go-dcgm`.
- Локальный HTTP endpoint `127.0.0.1:9990` по умолчанию.
- Быстрый внутренний сбор метрик с peak aggregation между external scrapes.
- Структурированные JSON-логи с настраиваемым уровнем логирования.
- Метрики используют префикс `gpu_`.

## Требования

- Go `1.26.4`
- NVIDIA driver
- NVIDIA DCGM runtime и development libraries
- Linux или WSL с доступом к NVIDIA GPU

В Ubuntu/WSL DCGM пакеты устанавливаются из NVIDIA repositories. Названия пакетов могут отличаться в зависимости от версии Ubuntu/CUDA, например:

```bash
sudo apt install --install-recommends datacenter-gpu-manager-4-cuda12 datacenter-gpu-manager-4-dev
```

## Сборка

```bash
go build ./...
```

## Запуск

```bash
go run .
```

Exporter будет доступен по адресу:

```text
http://127.0.0.1:9990/metrics
```

## Сборка binary

Так как проект использует CGO и DCGM, самый надежный вариант - собирать binary на той же Linux distribution family, что и target server.

Для RHEL 7 или RHEL 8 servers лучше собирать прямо на совместимом RHEL server или build host, где установлены NVIDIA driver и DCGM development libraries:

```bash
go build -trimpath -ldflags="-s -w" -o gpu-exporter .
```

Проверить runtime library links:

```bash
ldd ./gpu-exporter
```

На target server должен быть доступен `libdcgm.so.4`, обычно по path:

```text
/usr/lib64/libdcgm.so.4
```

Не стоит собирать binary на сильно более новой Linux system для старого target вроде RHEL 7. CGO binaries линкуются с glibc, и binary, собранный на новой glibc, может не запуститься на старом server.

## Структура проекта

| Файл | Ответственность |
| --- | --- |
| `main.go` | Точка входа приложения. Загружает конфигурацию, настраивает логирование, запускает DCGM, создает metrics/exporter/server objects, обрабатывает shutdown signals и корректно останавливает HTTP server. |
| `config.go` | Читает environment variables и применяет defaults для listen address, collection interval, request detection thresholds, DCGM mode и log level. |
| `dcgm_client.go` | Низкоуровневая интеграция с DCGM. Инициализирует DCGM, находит GPU, кеширует static GPU identity, создает persistent DCGM field watchers, читает GPU samples, конвертирует DCGM values и очищает DCGM groups при shutdown. |
| `metrics.go` | Описывает все exported `gpu_` metrics и их labels, затем регистрирует их в отдельном registry. |
| `exporter.go` | Основной collection loop. Периодически читает DCGM samples, обновляет current metrics, отслеживает peak values, считает inferred GPU work requests и flushes peak metrics при scrape `/metrics`. |
| `tracker.go` | Небольшие thread-safe helpers для peak aggregation и GPU activity/request detection. |
| `server.go` | HTTP layer. Экспортирует `/metrics` и `/health`, flushes peak values перед отдачей metrics и поддерживает graceful shutdown. |

## Настройка

Настройка выполняется через environment variables.

| Переменная | Значение по умолчанию | Описание |
| --- | --- | --- |
| `GPU_EXPORTER_ADDR` | `127.0.0.1:9990` | HTTP listen address. |
| `GPU_EXPORTER_SCRAPE_INTERVAL` | `100ms` | Внутренний интервал сбора метрик DCGM. |
| `GPU_EXPORTER_ACTIVE_THRESHOLD` | `1.0` | Порог GPU utilization для определения активной работы. |
| `GPU_EXPORTER_MIN_REQUEST_TIME` | `50ms` | Минимальная длительность активного окна, которое считается request. |
| `GPU_EXPORTER_DCGM_MODE` | `embedded` | Режим DCGM. |
| `GPU_EXPORTER_LOG_LEVEL` | `info` | Уровень логирования: `debug`, `info`, `warn`, или `error`. |

Пример:

```bash
GPU_EXPORTER_ADDR=0.0.0.0:9990 GPU_EXPORTER_LOG_LEVEL=debug go run .
```

## Метрики

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

Некоторые метрики имеют варианты с суффиксом `_max`. Они показывают пиковые значения, которые exporter увидел во внутреннем sampling window между external scrapes.

## Разработка

```bash
go test ./...
go fmt ./...
```
