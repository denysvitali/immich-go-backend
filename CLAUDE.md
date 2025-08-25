# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Environment Setup
- `nix develop` or `make dev-shell` - Enter Nix development environment (required for all development)
- `make setup` - Set up development environment and generate initial files

### Build & Test
- `make build` - Build the application (outputs to `bin/immich-go-backend`)
- `make test` - Run tests
- `make test-verbose` - Run tests with verbose output
- `make ci-check` - Run all CI checks (protobuf generation, linting, and tests)
- `make all` - Clean, setup, build, and test everything

### Code Quality
- `make fmt` - Format Go code
- `make lint` - Run linters (golangci-lint if available, otherwise go vet)
- `make mod-tidy` - Tidy Go modules

### Code Generation
- `make proto-gen` - Generate protocol buffer Go files using buf
- `make sqlc-gen` - Generate SQL code using sqlc
- `make proto-check` - Verify protocol buffer definitions and check for breaking changes

### Development Workflow
Always run these commands in the Nix development environment. The typical workflow is:
1. `make dev-shell` (if not already in Nix environment)
2. `make setup` (on first setup or after pulling changes)
3. Make code changes
4. `make ci-check` (before committing)

## Architecture Overview

### Project Structure
```
immich-go-backend/
├── cmd/                    # CLI commands and entry point (Cobra)
├── internal/              # Core application code
│   ├── albums/           # Album management service
│   ├── assets/           # Asset management with metadata extraction
│   ├── auth/             # JWT authentication and middleware
│   ├── config/           # Configuration management (Viper)
│   ├── db/sqlc/          # Generated database code
│   ├── proto/            # Protocol buffer definitions and generated code
│   ├── server/           # gRPC server and HTTP handlers
│   ├── storage/          # Storage abstraction layer
│   ├── telemetry/        # OpenTelemetry setup
│   ├── users/            # User management service
│   └── websocket/        # WebSocket support
├── sqlc/                 # SQL schema and queries
│   ├── queries.sql       # All database queries (116+)
│   └── schema.sql        # Database schema definition
└── scripts/              # Build and development scripts

### Core Components
- **Storage Abstraction Layer** (`internal/storage/`) - Universal storage interface supporting local filesystem, S3, and rclone backends with pre-signed URL support
- **Service Layer** (`internal/*/service.go`) - Domain-specific business logic (auth, users, assets, albums)
- **Database Layer** (`internal/db/`) - SQLC-generated type-safe PostgreSQL operations with 116+ queries
- **Protocol Buffers** (`internal/proto/`) - gRPC service definitions with automatic REST gateway generation
- **Configuration** (`internal/config/`) - YAML and environment variable configuration with Viper
- **Telemetry** (`internal/telemetry/`) - OpenTelemetry observability with autoexport

### Key Technologies
- **Go 1.24+** with Go modules
- **PostgreSQL** database with SQLC for type-safe queries
- **Protocol Buffers** with gRPC and grpc-gateway for REST compatibility
- **OpenTelemetry** for comprehensive observability
- **Nix** for reproducible development environment
- **AWS SDK v2** for S3 backend support

### Database Schema
The project uses SQLC to generate type-safe Go code from SQL queries. Key locations:
- `sqlc/queries.sql` - All SQL query definitions (116+ queries)
- `sqlc/schema.sql` - Database schema with custom UUID v7 function
- `internal/db/sqlc/` - Generated Go files for type-safe database operations
- `sqlc.yaml` - SQLC configuration

Run `make sqlc-gen` after modifying SQL files to regenerate Go code.

### Storage Backends
Three storage backends are supported through a unified interface:
- **Local** (`internal/storage/local.go`) - Local filesystem storage
- **S3** (`internal/storage/s3.go`) - AWS S3 compatible storage with pre-signed URLs
- **Rclone** (`internal/storage/rclone.go`) - Universal backend supporting 40+ cloud providers

### Service Architecture
Services follow clean architecture principles:
- **Auth Service** (`internal/auth/`) - JWT authentication, user registration/login, session management
- **User Service** (`internal/users/`) - User CRUD, profile management, preferences, admin functions
- **Asset Service** (`internal/assets/`) - Asset upload/download, metadata extraction, thumbnails
- **Album Service** (`internal/albums/`) - Album management and sharing

### Server Implementation
- **gRPC Server** (`internal/server/`) - Protocol buffer service implementations
- **HTTP REST Gateway** - Automatic REST API generation from protobuf definitions
- **WebSocket Support** (`internal/websocket/`) - Real-time communication
- **Middleware** - Authentication, CORS, logging, metrics collection

## Current Development Status

**Phase:** 4/10 (Core Services) - ~40% complete

**Completed:**
- ✅ Infrastructure setup (database, protobuf, dependencies)
- ✅ Storage abstraction layer with multi-backend support
- ✅ Configuration and telemetry systems
- ✅ Authentication service with JWT tokens
- ✅ User management service with full CRUD operations
- ✅ Asset management service with comprehensive search, deletion, and download features

**In Progress:**
- 🔄 Album management service completion
- 🔄 HTTP/gRPC controllers

**Next Priorities:**
1. Finish album management service
2. Add job queue system for background processing
3. Complete HTTP REST API endpoints
4. Add comprehensive testing infrastructure
5. Implement advanced features (face recognition, search, etc.)

## Configuration

The application uses YAML configuration files with environment variable overrides:
- `config.yaml` - Main configuration template
- `config.yaml.local` - Local development configuration (gitignored)
- Environment variables follow the pattern `IMMICH_SECTION_KEY`

Key configuration sections:
- Database connection settings
- Storage backend configuration (local/S3/rclone)
- Authentication settings (JWT secrets)
- Telemetry and observability settings
- Feature flags for optional functionality

## Development Environment

This project requires the Nix package manager for reproducible development environments. The Nix environment automatically provides:
- Go 1.24+
- Protocol Buffers compiler (protoc)
- Buf CLI tool for protobuf management
- SQLC for SQL code generation
- Development tools and dependencies

## Testing

Run `make test` for unit tests and `make test-verbose` for detailed output. The project aims for comprehensive test coverage across:
- Storage layer functionality
- Service layer business logic
- Database operations
- Authentication and authorization
- API endpoints

## Build System

The project uses a Makefile with Nix integration. Key build artifacts:
- `bin/immich-go-backend` - Main application binary
- Generated protobuf Go files in `internal/proto/`
- Generated SQLC database code in `internal/db/sqlc/`

Always ensure you're in the Nix development environment before running build commands.

## Important Files

- `buf.yaml`, `buf.gen.yaml` - Protocol buffer configuration
- `sqlc.yaml` - SQLC code generation configuration
- `flake.nix` - Nix flake for development environment
- `docker/Dockerfile` - Multi-stage Docker build
- `ROADMAP.md` - Detailed implementation phases and progress