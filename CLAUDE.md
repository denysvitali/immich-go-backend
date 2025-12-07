# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## CRITICAL REQUIREMENTS - NO MOCKS OR STUBS

### MANDATORY RULES
1. **NO STUB IMPLEMENTATIONS** - Every method MUST have real functionality
2. **NO MOCK DATA** - All responses MUST come from actual database operations
3. **NO PLACEHOLDER VALUES** - Use real data from PostgreSQL via SQLC
4. **NO "TODO: Implement later" RESPONSES** - Implement it NOW with real database queries
5. **NO HARDCODED TEST DATA** - All data must be read from or written to the database

### When implementing ANY service method:
- Use SQLC queries to interact with the database
- Create new SQLC queries if needed in `sqlc/queries.sql`
- Handle errors properly and return meaningful responses
- Perform actual CRUD operations on the database

## Development Commands

### Environment Setup
```bash
nix develop          # Enter Nix development environment (required)
make setup           # Generate initial files (run after pulling changes)
```

### Build & Test
```bash
make build           # Build to bin/immich-go-backend
make test            # Run tests
make test-verbose    # Run tests with verbose output
make ci-check        # Run all CI checks (proto-gen, lint, test)
```

### Code Generation
```bash
make proto-gen       # Generate protocol buffer Go files
make sqlc-gen        # Generate SQL code (run after modifying sqlc/*.sql)
make proto-check     # Verify protobuf definitions
```

### Code Quality
```bash
make fmt             # Format Go code
make lint            # Run linters
```

### Running a Single Test
```bash
go test -v -run TestFunctionName ./internal/package/...
```

### Starting the Server
```bash
go run main.go serve
```

## Architecture Overview

This is a Go backend for Immich with an S3-first architecture. The project uses:
- **PostgreSQL** with SQLC for type-safe queries
- **gRPC** with grpc-gateway for REST compatibility
- **OpenTelemetry** for observability
- **Nix** for reproducible builds

### Key Directories

```
internal/
├── server/           # gRPC server implementations and HTTP gateway
├── db/sqlc/          # Generated database code (DO NOT EDIT)
├── proto/gen/        # Generated protobuf code (DO NOT EDIT)
├── storage/          # Storage abstraction (local, S3, rclone)
├── auth/             # JWT authentication and middleware
├── users/            # User management service
├── assets/           # Asset management with metadata extraction
├── albums/           # Album management service
└── [other services]/ # Domain-specific services
sqlc/
├── queries.sql       # SQL query definitions (1300+ lines, 116+ queries)
└── schema.sql        # Database schema
```

### Service Pattern

Services follow a consistent pattern:

```go
type Service struct {
    db     *sqlc.Queries
    config *config.Config
    // OpenTelemetry metrics
}

func NewService(queries *sqlc.Queries, cfg *config.Config) (*Service, error) {
    // Initialize metrics
    return &Service{...}, nil
}

func (s *Service) DoSomething(ctx context.Context, req Request) (*Response, error) {
    ctx, span := tracer.Start(ctx, "service.do_something")
    defer span.End()
    // Use s.db.QueryName(ctx, params) for database operations
}
```

### Server Structure

The main server (`internal/server/server.go`) wires together:
- All service implementations
- gRPC server with registered handlers
- HTTP REST gateway via grpc-gateway
- WebSocket hub for real-time updates
- Authentication middleware

### Adding New Features

1. **Add SQL queries** in `sqlc/queries.sql`, then run `make sqlc-gen`
2. **Add protobuf definitions** in `internal/proto/`, then run `make proto-gen`
3. **Create service** in `internal/<feature>/service.go` following the pattern above
4. **Wire into server** in `internal/server/server.go`

### Storage Backends

Three backends via unified `StorageBackend` interface:
- **Local** - filesystem storage
- **S3** - AWS S3 compatible with pre-signed URLs
- **Rclone** - universal backend (40+ cloud providers)

Pre-signed URLs enable direct client uploads/downloads to S3.

### Configuration

YAML config with environment variable overrides:
- `config.yaml` - template
- `config.yaml.local` - local overrides (gitignored)
- Environment: `IMMICH_SECTION_KEY` pattern

## Current Status

**Phase 6/10** - Core Implementation (~70% complete)

Completed:
- Infrastructure, storage layer, configuration, telemetry
- Auth, users, assets, albums with full database operations
- 130+ gRPC endpoints with REST gateway
- Tags, partners, shared links, duplicates, trash, memories, timeline, notifications
- Stacks service (burst photos), faces service (reassignment)
- Job queue system with Redis (asynq) and handlers for all job types

Next: Testing infrastructure, ML integration, documentation

Note: immich-upstream contains the original immich project (original server implementation)