# Запуск полного стека в Windows через Docker Desktop

Эта конфигурация запускает четыре контейнера:

- `gpu-exporter` с доступом к NVIDIA GPU;
- Prometheus для хранения метрик;
- Grafana с автоматически добавленными datasource и dashboard;
- официальный Grafana MCP server в read-only режиме.

## 1. Подготовка Docker Desktop

1. Установите актуальный NVIDIA-драйвер с поддержкой WSL2.
2. В Docker Desktop используйте Linux containers и WSL2 backend.
3. Откройте PowerShell в корне репозитория и проверьте GPU:

```powershell
nvidia-smi
docker run --rm --gpus all nvidia/cuda:12.8.1-base-ubuntu22.04 nvidia-smi
```

Обе команды должны показать NVIDIA GPU. Отдельно устанавливать DCGM в Windows не нужно: он уже находится внутри образа экспортера.

## 2. Настройка переменных

Создайте локальный `.env` из примера:

```powershell
Copy-Item .\examples\docker\.env.example .\examples\docker\.env
```

По умолчанию используются CUDA 12 и локальные учётные данные Grafana `admin` / `admin`. Измените пароль в `.env`. Для CUDA 13 задайте:

```dotenv
DCGM_CUDA_MAJOR=13
```

## 3. Загрузка готового образа

Docker умеет загружать gzip-архив напрямую:

```powershell
docker load --input .\distr\gpu-exporter-image-0.5.0-cuda12.tar.gz
```

Для CUDA 13 замените имя файла на `gpu-exporter-image-0.5.0-cuda13.tar.gz` и значение `DCGM_CUDA_MAJOR` в `.env` на `13`.

Контрольную сумму можно проверить так:

```powershell
Get-FileHash .\distr\gpu-exporter-image-0.5.0-cuda12.tar.gz -Algorithm SHA256
Get-Content .\distr\SHA256SUMS
```

## 4. Запуск стека

С уже загруженным образом:

```powershell
docker compose --env-file .\examples\docker\.env -f .\examples\docker\compose.stack.yaml up -d --no-build
```

Либо соберите exporter из текущего исходного кода и сразу запустите стек:

```powershell
docker compose --env-file .\examples\docker\.env -f .\examples\docker\compose.stack.yaml up -d --build
```

Проверка состояния:

```powershell
docker compose --env-file .\examples\docker\.env -f .\examples\docker\compose.stack.yaml ps
Invoke-WebRequest http://localhost:9990/ready -UseBasicParsing
```

После запуска доступны:

| Компонент | Адрес |
|---|---|
| GPU Exporter | `http://localhost:9990/metrics` |
| Prometheus | `http://localhost:9090` |
| Prometheus targets | `http://localhost:9090/targets` |
| Grafana | `http://localhost:3000` |
| Grafana MCP | `http://localhost:8000/mcp` |

В Grafana datasource `Prometheus` и dashboard **GPU Exporter / Мониторинг и отчёт** создаются автоматически. Первый вход выполняется учётными данными из `.env`.

## 5. Подключение Grafana MCP

В MCP-клиенте создайте HTTP server со следующими параметрами:

- transport: `Streamable HTTP`;
- URL: `http://localhost:8000/mcp`.

Открывать этот адрес в браузере не требуется: это MCP endpoint, а не веб-интерфейс. Состояние сервера смотрите через журнал:

```powershell
docker compose --env-file .\examples\docker\.env -f .\examples\docker\compose.stack.yaml logs --tail 100 grafana-mcp
```

MCP запущен с `--disable-write`: он может искать dashboards/datasources и выполнять PromQL, но не изменяет Grafana. Для не локального окружения вместо admin/password используйте Grafana service account token в переменной `GRAFANA_SERVICE_ACCOUNT_TOKEN` и минимально необходимые права.

## 6. Остановка

Остановить контейнеры, сохранив данные Prometheus и Grafana:

```powershell
docker compose --env-file .\examples\docker\.env -f .\examples\docker\compose.stack.yaml down
```

Удалить также Docker volumes со всей накопленной историей:

```powershell
docker compose --env-file .\examples\docker\.env -f .\examples\docker\compose.stack.yaml down -v
```

Последняя команда необратимо удаляет локальные данные Prometheus и Grafana.
