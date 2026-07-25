# CLAUDE.md

Guidance for Claude Code (claude.ai/code) working in this repository.

Longer prose lives in [ARCHITECTURE.md](ARCHITECTURE.md), [TESTING.md](TESTING.md),
[CONTRIBUTING.md](CONTRIBUTING.md), [DEPLOYMENT.md](DEPLOYMENT.md), [ROADMAP.md](ROADMAP.md).
This file is the short list of things that are easy to get wrong.

## What this project is

A Go reimplementation of the Immich backend. The goal is **API parity with upstream Immich**
so the *official, unmodified* Immich web app and mobile apps work against it. That constraint
outranks internal elegance: if upstream sends a weird payload or expects a non-obvious response
shape, match upstream.

CI enforces it. `.github/workflows/go.yaml` pins:

- `OPENAPI_UPSTREAM_REF: v3.0.1` — the spec parity is measured against
- `IMMICH_UPSTREAM_REF: v2.7.5` — the web app the browser E2E suite runs
- `OPENAPI_COVERAGE_FAIL_UNDER: 100` — **every** route in the upstream spec must be served

So: adding a route is cheap, *missing* one is a red build.

```bash
# Reproduce the parity gate locally
git clone --depth 1 -b v3.0.1 --filter=blob:none --sparse \
  https://github.com/immich-app/immich immich-upstream
git -C immich-upstream sparse-checkout set open-api
go run ./cmd openapi-coverage --md coverage.md   # defaults find the spec + gen dirs
```

## Rule 1: no mocks, no stubs, no placeholders

Every service method uses real SQLC queries against PostgreSQL. No stub implementations, fake
data, hardcoded return values, or `// TODO: implement later`. If the query you need doesn't
exist, add it to `sqlc/queries.sql` and run `sqlc generate`.

## Rule 2: a fresh clone does not compile

`internal/proto/gen/` is **gitignored** (see `.gitignore:1`). Nothing builds until you generate it:

```bash
buf generate          # writes internal/proto/gen/ (~104 files); needed once per clone
go build ./cmd
```

Asymmetry worth remembering: **proto output is generated, SQLC output is committed.**
`internal/db/sqlc/` is in git; `internal/proto/gen/` is not. Never hand-edit either.

## Rule 3: the DB schema has two sources — update both

| File | Used by |
|------|---------|
| `sqlc/schema.sql` | `sqlc generate` (type generation) **and** `internal/db/testdb` for integration tests |
| `internal/db/migrations/NNN_*.sql` | the running server (`migrate` cmd, embedded via `go:embed`) |

Adding a column to only one of them produces tests that pass and a server that fails, or the
reverse. Add a new numbered migration **and** patch `sqlc/schema.sql`.

## Commands

Prerequisites: Go 1.24 + `buf` + `sqlc` + `golangci-lint`. `nix develop` (or `shell.nix`) pins
those versions but is optional — plain toolchain installs work fine, and CI doesn't use Nix.

| Task | Command | Note |
|------|---------|------|
| Generate protos | `buf generate` | `make proto-gen` also works |
| Generate SQLC | `sqlc generate` | after editing `sqlc/queries.sql` |
| Build | `go build -o bin/immich-go-backend ./cmd` | `make build` re-runs proto-gen first (slow) |
| Unit tests | `go test -short -count=1 ./...` | what CI runs; no Docker needed |
| One test | `go test -run TestName ./internal/pkg/...` | |
| Integration tests | `go test -tags=integration -race ./internal/...` | **needs Docker**; 30 files carry the tag |
| Lint | `golangci-lint run` | config is golangci-lint **v2** schema |
| Format | `gofmt -s -w .` | CI checks `gofmt -s -l`; `make fmt` runs plain `go fmt` and will *not* satisfy it |
| Parity check | `go run ./cmd openapi-coverage` | see above |
| Run server | `go run ./cmd serve` / `... migrate` / `... version` | `--config path.yaml` |
| Browser E2E | `pnpm --dir e2e test` | needs backend on :3001 + Immich web on :3000 |

Local infra: `docker compose up -d` (PostgreSQL + Redis). Note the E2E workflow uses
`tensorchord/vchord-suite:pg17` and creates `vector`, `vchord`, `cube`, `earthdistance`,
`pg_trgm`, `unaccent` extensions — pgvector-flavoured Postgres, not vanilla.

## Architecture

34 gRPC service definitions in `internal/proto/*.proto`. Dual-protocol: gRPC on **:3002**,
REST on **:3001** via grpc-gateway. REST paths come from `option (google.api.http)` annotations
in the `.proto` files — that's where an endpoint's URL is decided.

```
HTTP client → webui.Handler (static SPA passthrough)
            → httpLoggingHandler
            → handleWs  ← hand-written interceptors + socket.io live here
            → grpc-gateway mux → gRPC service impl → SQLC → PostgreSQL
```

Async work (thumbnails, metadata extraction, ML) goes through asynq on Redis (`internal/jobs/`).

### Key locations

- `cmd/` — Cobra CLI: `serve`, `migrate`, `version`, `openapi-coverage`
- `internal/server/server.go` — all wiring: services, gRPC server, gateway mux, WS hub, auth middleware. `HTTPHandler()` (~line 518) is the route table.
- `internal/server/http_*.go` — hand-written REST handlers (see below)
- `internal/<domain>/service.go` — domain logic, one package per gRPC service
- `internal/openapicoverage/` — the parity tool (parses `.pb.gw.go` + hand-written handler ASTs)
- `internal/storage/` — Local / S3 (pre-signed URLs) / Rclone backends
- `internal/webui/` — serves an optional static Immich web build; empty `WebUIDir` is a transparent passthrough
- `internal/db/testdb/` — testcontainers-backed real PostgreSQL for tests

### Service pattern

```go
type Service struct {
    db     *sqlc.Queries
    config *config.Config
}

func (s *Service) DoSomething(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    ctx, span := tracer.Start(ctx, "service.do_something")
    defer span.End()
    // s.db.QueryName(ctx, params)
}
```

## The hand-written HTTP escape hatch

grpc-gateway JSON-encodes everything. That's wrong for some upstream endpoints — raw image/video
bytes, multipart upload, socket.io, response shapes protojson can't express. Those are
intercepted **before** the gateway mux, in `handleWs` (`internal/server/server.go:653`), and
implemented in `internal/server/http_*.go`.

Two consequences when you add one:

1. Register the interception inside `handleWs` — order matters, first match wins, `mux.ServeHTTP`
   is the fallthrough at the end.
2. `internal/openapicoverage/manual.go` finds these routes by **parsing the Go AST** for literal
   `r.URL.Path == "/api/..."` comparisons and `http.MethodGet`-style method constants. A route
   built from a variable or a helper will not be detected, and the 100% parity gate will fail
   even though the endpoint works. Write the literal.

Reach for the gateway first. Use this only when the wire format genuinely can't be expressed as
protojson.

## Configuration

YAML (`config.yaml`, plus gitignored `config.yaml.local`) with env overrides.

Env var names are **not** a uniform scheme — they're a hand-maintained block of `os.Getenv`
checks starting around `internal/config/config.go:499`. Some are `IMMICH_`-prefixed
(`IMMICH_DATABASE_AUTO_MIGRATE`, `IMMICH_WEBUI_DIR`, `IMMICH_JOBS_RETRY_MAX_RETRIES`), others are
bare (`DATABASE_URL`, `SERVER_ADDRESS`, `AUTH_JWT_SECRET`, `STORAGE_BACKEND`). To add an override
you must edit that function — struct tags alone won't wire it up.

## Testing tiers

1. **Unit** — `go test -short ./...`, no Docker, must stay fast.
2. **Integration** — `//go:build integration`, real PostgreSQL via testcontainers. Use
   `testdb.SetupTestDB(t)`. Mock at the SQLC boundary only; there are no in-memory SQLC fakes.
3. **Browser E2E** — `e2e/` Playwright, drives the real upstream Immich web app against this
   backend. This is what catches parity regressions the spec check can't see (wrong field
   casing, missing DTO fields, unusable media content types).

Details in [TESTING.md](TESTING.md).

## Conventions

- Conventional commits, enforced on PR titles: `fix(assets): accept AVIF uploads`.
- One logical change per PR.
- New behaviour needs tests: unit for logic, integration for anything touching DB or storage.
- gosec and golangci-lint exclude `internal/proto/gen/` and `internal/db/sqlc/`.
