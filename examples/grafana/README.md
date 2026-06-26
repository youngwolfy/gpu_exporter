# Grafana Dashboards

This directory contains importable Grafana dashboards for `gpu_exporter`.

## Dashboards

- `gpu-operations.json` — live technical dashboard for operators.
- `gpu-usage-report.json` — management-oriented report over the selected time range.

Both dashboards use a Prometheus datasource variable named `datasource` and query
GPU labels exported by `gpu_exporter`:

- `hostname`
- `gpu_index`
- `gpu_name`

The `active_threshold` variable defaults to `5` percent and is used to classify
GPU activity windows.

## Import

1. Open Grafana.
2. Go to **Dashboards** → **New** → **Import**.
3. Upload one of the JSON files.
4. Select the Prometheus datasource.

## Report Export

For monthly or weekly management reports, use `gpu-usage-report.json`.
In gpu_exporter `0.4.0` and newer, the report uses integral counter metrics
such as `gpu_active_seconds_total`, `gpu_utilization_weighted_seconds_total`,
`gpu_tensor_active_weighted_seconds_total`, and `gpu_energy_joules_total`.

Set the dashboard time range, for example **Last 30 days**, then export the
`GPU usage KPI table, selected range` panel through Grafana Panel Inspector.
The table intentionally returns one row per `host / GPU / KPI` so the CSV export
does not depend on Grafana table transformations.

The report does not treat calendar-average utilization as the only KPI. For LLM
inference, use it together with:

- active time percent
- active hours
- average utilization while active
- peak and P95 utilization
- inferred GPU activity windows
- tensor active ratio
