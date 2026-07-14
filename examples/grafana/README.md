# Дашборд Grafana

В каталоге лежит единый импортируемый dashboard для `gpu_exporter`:

- `gpu-exporter.json` — оперативный мониторинг GPU и отчётные KPI за выбранный период.

В dashboard также включён раздел **Надёжность экспортера и DCGM** для метрик,
добавленных в `0.5.0`:

- полнота сбора и состояние каждого GPU (`gpu_exporter_*`);
- поддержка, доступность и возраст DCGM-полей (`gpu_dcgm_field_*`);
- PCIe/NVLink current, max, avg и интегральные byte counters;
- profiling ratios для graphics, occupancy, FP/INT и Tensor sub-pipes;
- ECC, retired pages, remapped rows, PCIe replay и clock violations;
- температурный запас до maximum operating temperature;
- доля времени с валидными utilization/DCP observations.

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

При запуске [`../docker/compose.stack.yaml`](../docker/compose.stack.yaml)
ручной импорт не нужен: datasource и dashboard загружаются через файлы из
`provisioning/`. Пошаговая инструкция для Windows находится в
[`../docker/WINDOWS.md`](../docker/WINDOWS.md).

## Выгрузка отчёта

Для общего отчёта за выбранный диапазон времени выставьте нужный диапазон,
например **Last 30 days**, и выгружайте CSV из панели
**Сводная таблица KPI по GPU** через Grafana Panel Inspector.

Для отчёта со средней утилизацией за фиксированные периоды используйте панель
**Средняя утилизация GPU по периодам**. Она показывает календарную среднюю
утилизацию за последние 7, 30 и 90 дней и не зависит от выбранного диапазона
дашборда.

Средняя утилизация GPU, средняя мощность и средняя активность SM/Tensor/DRAM
считаются через интегральные counter-метрики (`increase`/`rate`), а не через
простое среднее от gauge-значений. `avg_over_time` используется только для
визуального усреднения gauge-метрик, для которых нет отдельного counter.

Начиная с `gpu_exporter` `0.5.0`, отчётные KPI используют интегральные counter
метрики:

- `gpu_active_seconds_total`
- `gpu_utilization_weighted_seconds_total`
- `gpu_utilization_observed_seconds_total`
- `gpu_tensor_active_weighted_seconds_total`
- `gpu_tensor_active_observed_seconds_total`
- `gpu_energy_joules_total`

Календарная средняя утилизация не является единственным KPI для LLM-инференса.
Смотрите её вместе с активным временем, активными часами, средней утилизацией
во время активности, peak/P95, оценкой активных окон и активностью Tensor.
