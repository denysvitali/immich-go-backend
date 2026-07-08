# Monitoring — immich-go-backend

Grafana dashboard, Prometheus scrape examples, and Alertmanager-compatible rules grounded on **metrics the binary defines in code** (OpenTelemetry instruments under `internal/*/service.go` + `internal/telemetry`).

## Layout

```
deploy/monitoring/
├── README.md
├── grafana/dashboards/immich-go-backend.json
└── prometheus/
    ├── alerts.yml
    ├── scrape-config.example.yml
    └── servicemonitor.yaml
```

## How metrics leave the process

| Path | When | Scrape target |
|------|------|---------------|
| App config `server.metrics_path` (default `/metrics`) | Documented in `DEPLOYMENT.md` / `server.metrics_enabled` | REST `:3001/metrics` |
| OTel autoexport Prometheus exporter | `telemetry.NewProvider` + `OTEL_METRICS_EXPORTER=prometheus` | `OTEL_EXPORTER_PROMETHEUS_HOST:PORT` (default `localhost:9464`) `/metrics` |
| OTLP | `OTEL_METRICS_EXPORTER=otlp` (or unset default for autoexport) + `OTEL_EXPORTER_OTLP_ENDPOINT` | Scrape collector / backend, not the app |

Instruments are created via `telemetry.GetMeter()` (`Meter("immich-go-backend")`) in domain services. Prometheus export uses `go.opentelemetry.io/otel/exporters/prometheus` naming:

- Monotonic counters → `*_total` (names that already end in `_total` stay stable)
- Histograms → `*_bucket` / `*_sum` / `*_count`
- UpDownCounters → gauges
- Attributes (e.g. `operation`) → labels
- Optional `target_info`, `otel_scope_name`, `otel_scope_version`

> **Note:** `telemetry.NewMetrics` in `internal/telemetry/telemetry.go` defines extra names (`http_requests_total`, `db_queries_total`, …) but **is not called** by the binary today. Those names are **not** included in the dashboard or alerts. There are also **no** `go_*` / `process_*` collectors registered in-tree — do not expect them unless you bridge a Prometheus registry with runtime collectors.

## Metric names used

### Counters (rate / increase)

| Metric | Source |
|--------|--------|
| `assets_uploads_total` | `internal/assets` |
| `assets_downloads_total` | `internal/assets` |
| `user_operations_total` | `internal/users` (`operation` label) |
| `admin_operations_total` | `internal/admin` |
| `face_operations_total` | `internal/faces` |
| `stack_operations_total` | `internal/stacks` |
| `duplicates_operations_total` | `internal/duplicates` |
| `plugin_operations_total` | `internal/plugin` |
| `view_operations_total` | `internal/view` |
| `systemmetadata_operations_total` | `internal/systemmetadata` |
| `workflow_executions_total` | `internal/workflow` |

### Histograms (latency)

| Metric | Source |
|--------|--------|
| `assets_processing_duration_seconds` | `internal/assets` |
| `user_operation_duration_seconds` | `internal/users` |
| `admin_operation_duration_seconds` | `internal/admin` |
| `face_operation_duration_seconds` | `internal/faces` |
| `stack_operation_duration_seconds` | `internal/stacks` |
| `duplicates_operation_duration_seconds` | `internal/duplicates` |
| `plugin_operation_duration_seconds` | `internal/plugin` |
| `view_operation_duration_seconds` | `internal/view` |
| `systemmetadata_operation_duration_seconds` | `internal/systemmetadata` |
| `workflow_operation_duration_seconds` | `internal/workflow` |

### Gauges (UpDownCounter)

| Metric | Source |
|--------|--------|
| `assets_storage_bytes` | `internal/assets` |
| `users_total` | `internal/users` |
| `faces_total` | `internal/faces` |
| `stacks_total` | `internal/stacks` |
| `duplicates_found_total` | `internal/duplicates` |
| `workflows_total` | `internal/workflow` |
| `plugins_total` | `internal/plugin` |

### Prometheus built-in (scrape)

| Metric | Purpose |
|--------|---------|
| `up` | Target reachability (alerts + dashboard status) |
| `target_info` | OTel resource attributes when exporter emits them |

## Grafana

1. Add a Prometheus datasource in Grafana.
2. **Import** `grafana/dashboards/immich-go-backend.json` (Dashboards → Import → Upload JSON).
3. Select the Prometheus datasource. Variables: `job`, `instance`.

Optional provisioning:

```yaml
# grafana provisioning example
apiVersion: 1
providers:
  - name: immich-go-backend
    folder: Immich
    type: file
    options:
      path: /var/lib/grafana/dashboards/immich-go-backend
```

Copy the JSON into that path (or mount `deploy/monitoring/grafana/dashboards/`).

## Prometheus scrape

See [`prometheus/scrape-config.example.yml`](prometheus/scrape-config.example.yml).

Minimal static scrape:

```yaml
scrape_configs:
  - job_name: immich-go-backend
    metrics_path: /metrics
    static_configs:
      - targets: ["127.0.0.1:3001"]
```

Kubernetes: apply [`prometheus/servicemonitor.yaml`](prometheus/servicemonitor.yaml) (Prometheus Operator). Align Service labels/ports with your Deployment.

## Alerts

Load [`prometheus/alerts.yml`](prometheus/alerts.yml):

```yaml
rule_files:
  - /etc/prometheus/rules/immich-go-backend-alerts.yml
```

| Alert | Severity | Signal |
|-------|----------|--------|
| `ImmichGoBackendTargetDown` | critical | `up == 0` for 2m |
| `ImmichGoBackendAbsentMetrics` | warning | no matching scrape job |
| `ImmichGoBackendAssetProcessingLatencyHigh` | warning | processing p95 > 30s |
| `ImmichGoBackendAssetUploadRateAnomaly` | warning | uploads stalled after traffic |
| `ImmichGoBackendUserOperationLatencyHigh` | warning | user op p95 > 5s |
| `ImmichGoBackendAdminOperationLatencyHigh` | warning | admin op p95 > 60s |
| `ImmichGoBackendWorkflowExecutionSpike` | info | high execution rate |
| `ImmichGoBackendAssetStorageGrowingFast` | info | storage derivative |

Route via Alertmanager as usual (`severity` / `service` labels are set).

## Quick verify

```bash
# App metrics path (when wired / enabled)
curl -sS "http://127.0.0.1:3001/metrics" | head

# OTel autoexport Prometheus port (when exporter enabled)
curl -sS "http://127.0.0.1:9464/metrics" | head

# Expect names like:
#   assets_uploads_total
#   user_operations_total{operation="get_user"}
#   assets_processing_duration_seconds_bucket
```

## Related code

- `internal/telemetry/telemetry.go` — OTel provider + `NewMetrics` (unused catalog)
- Domain services listed above — live instruments
- `internal/config/config.go` — `server.metrics_enabled`, `server.metrics_path`
- `DEPLOYMENT.md` — operations / `/metrics` notes
