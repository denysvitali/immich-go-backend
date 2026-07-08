# Performance / load scaffolding

Lightweight tools for smoke load and micro-benchmarks. **Not** wired into default CI.

| Tool | What it measures | Needs |
|------|------------------|--------|
| `load-smoke.sh` | HTTP latency / success vs a running server | `curl`, running backend |
| `load-k6.js` | Optional k6 scenario (same read-only paths) | [k6](https://k6.io) installed |
| Go benches (`-tags bench`) | Local storage I/O + SQLC DB ops | Go; Docker for DB only |

## Makefile targets

```bash
make perf-load      # bash+curl smoke (IMMICH_URL)
make perf-storage   # local filesystem storage benchmarks
make perf-db        # PostgreSQL/SQLC benchmarks (skips without Docker)
```

## Load smoke (`make perf-load`)

Hits **read-only** endpoints only:

- `/health`
- `/api/server/ping`
- `/api/server/version`
- `/api/server/features`
- `/api/server/config`
- `/api/server/media-types`

```bash
# defaults: IMMICH_URL=http://localhost:3001, REQUESTS=100, CONCURRENCY=10
make perf-load

IMMICH_URL=http://127.0.0.1:3001 REQUESTS=500 CONCURRENCY=25 ./scripts/perf/load-smoke.sh
```

| Env | Default | Meaning |
|-----|---------|---------|
| `IMMICH_URL` | `http://localhost:3001` | Base URL (no trailing slash required) |
| `REQUESTS` | `100` | Total HTTP requests |
| `CONCURRENCY` | `10` | Parallel workers |
| `TIMEOUT_SEC` | `5` | Per-request curl timeout |
| `FAIL_ON_ERROR` | `1` | Exit non-zero if any non-2xx |

Start the API first, e.g. `docker-compose up -d` then `go run ./cmd serve`.

### Optional k6

```bash
k6 run -e IMMICH_URL=http://localhost:3001 scripts/perf/load-k6.js
k6 run -e IMMICH_URL=http://localhost:3001 --vus 20 --duration 30s scripts/perf/load-k6.js
```

k6 is **not** required for Makefile targets or CI.

## Storage benchmarks (`make perf-storage`)

Go benchmarks against `storage.LocalBackend` (temp dir). No Docker.

```bash
make perf-storage
# equivalent:
go test -tags bench -bench=BenchmarkStorage -benchmem -count=1 -run='^$' ./scripts/perf/
```

Covers upload / download (1 KiB & 1 MiB), exists, and upload→download→delete round-trip.

## DB benchmarks (`make perf-db`)

SQLC queries via `internal/db/testdb` (testcontainers Postgres). **Skips** when:

- `docker` is not on `PATH`
- `SKIP_INTEGRATION_TESTS` is set
- container / schema setup fails

```bash
make perf-db
# equivalent:
go test -tags bench -bench=BenchmarkDB -benchmem -count=1 -run='^$' ./scripts/perf/
```

Ops: `GetUserByID`, `GetUserByEmail`, `GetAssetByID`, `CreateAsset`, `CountAssets`.

First run may pull the Postgres image (same image as integration tests).

## Build tag `bench`

All `*_bench_test.go` files use `//go:build bench`, so they are **excluded** from plain `go test ./...` and default CI.

```bash
go test -tags bench -bench=. -benchmem ./scripts/perf/ -run='^$'
```

## Notes

- Smoke load is intentionally gentle; raise `REQUESTS` / k6 VUs for harder pressure.
- Do not point load scripts at production without care; only use environments you own.
- Storage benches measure local backend only (not S3/rclone).
- DB benches use a real Postgres container; results vary by host/Docker disk.
