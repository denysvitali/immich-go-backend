# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Critical: No Mocks or Stubs

Every service method MUST use real SQLC database queries. No stub implementations, mock data, placeholder values, or "TODO: implement later" patterns. If a SQLC query doesn't exist yet, add it to `sqlc/queries.sql` and regenerate.

## Development Commands

**Prerequisites:** Nix package manager. Enter the dev environment first:
```bash
nix develop
```

**Local infrastructure** (PostgreSQL 16, Redis 7):
```bash
docker-compose up -d
```

| Task | Command |
|------|---------|
| Build | `make build` |
| Run server | `go run ./cmd serve` |
| Run all tests | `make test` |
| Run single test | `go test -v -run TestFunctionName ./internal/package/...` |
| Lint | `make lint` |
| Format | `make fmt` |
| Generate protobuf code | `make proto-gen` |
| Generate SQLC code | `make sqlc-gen` |
| Full CI check | `make ci-check` |
| Initial setup after clone | `make setup` |

## Architecture

Go backend for Immich (~28 gRPC services) with S3-first storage. Dual-protocol: gRPC on port 3002, REST via grpc-gateway on port 3001.

### Request flow

HTTP/REST client → grpc-gateway → gRPC service → service layer → SQLC queries → PostgreSQL

Async work goes through asynq (Redis-backed job queue).

### Key directories

- `cmd/` — CLI entry point (Cobra). Subcommands: `serve`, `migrate`, `version`
- `internal/server/server.go` — Wires all services, gRPC server, HTTP gateway, WebSocket hub, auth middleware
- `internal/<service>/service.go` — Domain services (auth, users, assets, albums, etc.). Each holds `*sqlc.Queries` + `*config.Config` + OTel metrics
- `internal/proto/` — `.proto` files defining all gRPC services
- `internal/proto/gen/` — Generated protobuf Go code (**do not edit**)
- `internal/db/sqlc/` — Generated SQLC Go code (**do not edit**)
- `internal/db/testdb/` — Test helpers using testcontainers (real PostgreSQL)
- `internal/storage/` — Storage abstraction: Local, S3 (pre-signed URLs), Rclone
- `internal/jobs/` — Async job queue (asynq/Redis)
- `sqlc/queries.sql` — All SQL query definitions (edit this, then `make sqlc-gen`)
- `sqlc/schema.sql` — Database schema

### Service pattern

```go
type Service struct {
    db     *sqlc.Queries
    config *config.Config
}

func (s *Service) DoSomething(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    ctx, span := tracer.Start(ctx, "service.do_something")
    defer span.End()
    // Use s.db.QueryName(ctx, params)
}
```

### Adding a new feature

1. Add SQL queries in `sqlc/queries.sql` → `make sqlc-gen`
2. Add protobuf definitions in `internal/proto/` → `make proto-gen`
3. Create `internal/<feature>/service.go` following the service pattern
4. Wire into `internal/server/server.go`

### Testing

Integration tests use `testcontainers-go` to spin up real PostgreSQL containers. Use `internal/db/testdb.SetupTestDB()` to get a `*sqlc.Queries` backed by a real database with the schema applied. Tests require Docker.

### Configuration

YAML config with env var overrides (`IMMICH_SECTION_KEY` pattern):
- `config.yaml` — template
- `config.yaml.local` — local overrides (gitignored)

### Linting

Uses `golangci-lint` with: errcheck, staticcheck, gosec, govet, gofmt, gofumpt, goimports, misspell, gocritic, and others. Generated proto files (`internal/proto/gen/`, `*.pb.go`, `*.pb.gw.go`) are excluded.
