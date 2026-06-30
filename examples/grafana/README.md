# Дашборд Grafana

В каталоге лежит единый импортируемый dashboard для `gpu_exporter`:

- `gpu-exporter.json` — оперативный мониторинг GPU и отчётные KPI за выбранный период.

Дашборд использует переменную Prometheus datasource `datasource` и метки,
которые экспортирует `gpu_exporter`:

- `hostname`
- `gpu_index`
- `gpu_name`

Переменная `active_threshold` по умолчанию равна `5` процентам и используется
для определения активных окон GPU.

## Импорт

1. Откройте Grafana.
2. Перейдите в **Dashboards** -> **New** -> **Import**.
3. Загрузите `gpu-exporter.json`.
4. Выберите Prometheus datasource.

## Выгрузка отчёта

Для недельных и месячных отчётов выставьте нужный диапазон времени, например
**Last 30 days**, и выгружайте CSV из панели **Сводная таблица KPI по GPU**
через Grafana Panel Inspector.

Начиная с `gpu_exporter` `0.4.0`, отчётные KPI используют интегральные counter
метрики:

- `gpu_active_seconds_total`
- `gpu_utilization_weighted_seconds_total`
- `gpu_tensor_active_weighted_seconds_total`
- `gpu_energy_joules_total`

Календарная средняя утилизация не является единственным KPI для LLM-инференса.
Смотрите её вместе с активным временем, активными часами, средней утилизацией
во время активности, peak/P95, оценкой активных окон и активностью Tensor.
