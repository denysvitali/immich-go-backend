# immich-go-backend

A Go-based alternative backend for [Immich](https://github.com/immich-app/immich), designed with an S3-first architecture and cloud-native patterns.

> [!WARNING]
> **THIS CODE IS AI-GENERATED AND NOT SUITABLE FOR PRODUCTION USE**
>
> This project was developed primarily using AI assistants (Claude Opus 4.5, Claude Sonnet 4) and automated coding tools. While functional, it has **not been thoroughly tested**, **security audited**, or **validated for production workloads**.
>
> **USE AT YOUR OWN RISK.** The authors take no responsibility for data loss, security vulnerabilities, or any other issues that may arise from using this software.
>
> If you choose to use this project:
> - Do NOT use it with important data without backups
> - Do NOT expose it to the public internet without proper security review
> - Expect bugs, incomplete features, and breaking changes

---

## Project Goals

- **API Compatibility**: Achieve full parity with upstream Immich (~230 endpoints)
- **Mobile App Support**: iOS/Android Immich apps should work seamlessly
- **S3-First Architecture**: Native object storage with pre-signed URLs
- **Performance**: Leverage Go's concurrency for better scalability

## Current Status

| Metric | Value |
|--------|-------|
| **API Coverage** | ~60% (estimated) |
| **Total Services** | 28 |
| **SQL Queries** | 200+ |
| **Build Status** | Compiles |

### Implemented Services

| Service | Status | Description |
|---------|--------|-------------|
| AuthService | Working | JWT auth, login, logout, PIN codes, session lock/unlock |
| UsersService | Working | Profile, preferences, onboarding, license |
| AssetService | Working | CRUD, thumbnails, video playback, bulk operations |
| AlbumService | Working | CRUD, sharing, role-based access |
| SyncService | Working | Real-time events, delta/full sync |
| SessionsService | Working | Database-backed session management |
| TimelineService | Working | Chronological asset browsing |
| MemoryService | Working | Memories with asset associations |
| SearchService | Working | Metadata and smart search |
| PeopleService | Working | Face recognition, person management |
| TagsService | Working | Asset tagging |
| SharedLinksService | Working | Shareable links with passwords |
| TrashService | Working | Soft delete with recovery |
| MapService | Working | Geolocation browsing |
| DuplicatesService | Working | Duplicate detection |
| DownloadService | Working | Asset downloads, archives |
| AdminService | Working | User management, notifications |
| MaintenanceService | Working | Maintenance mode control |
| QueueService | Working | Job queue management |
| PluginService | Working | Plugin system (extensibility) |
| WorkflowService | Working | Automation workflows |
| LibrariesService | Working | External library management |
| StacksService | Working | Asset stacking |
| FacesService | Working | Face management |
| NotificationsService | Working | User notifications |
| PartnersService | Working | Partner sharing |
| ActivityService | Working | Activity feed |
| SystemMetadataService | Working | System configuration |

### Known Limitations

- **Token refresh** endpoint not implemented
- **OCR data** endpoint not implemented
- **OAuth mobile redirect** not implemented
- **Email verification/password reset** flows not implemented
- **HLS/adaptive streaming** not implemented
- Some edge cases may not be handled properly
- Error messages may leak internal details
- No comprehensive test coverage

## Technology Stack

- **Language**: Go 1.24+
- **Database**: PostgreSQL with [SQLC](https://sqlc.dev/) for type-safe queries
- **API**: gRPC with [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) for REST
- **Storage**: Local, S3, or Rclone backends
- **Observability**: OpenTelemetry (tracing, metrics)
- **Authentication**: JWT with bcrypt password hashing

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    HTTP/REST Clients                     │
│                  (Immich Mobile/Web)                     │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                   grpc-gateway                           │
│              (REST ↔ gRPC translation)                   │
└─────────────────────────┬───────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                    gRPC Services                         │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐       │
│  │  Auth   │ │  Users  │ │ Assets  │ │ Albums  │  ...  │
│  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘       │
└───────┼──────────┼──────────┼──────────┼───────────────┘
        │          │          │          │
        ▼          ▼          ▼          ▼
┌─────────────────────────────────────────────────────────┐
│                   Service Layer                          │
│            (Business logic, validation)                  │
└─────────────────────────┬───────────────────────────────┘
                          │
        ┌─────────────────┼─────────────────┐
        ▼                 ▼                 ▼
┌───────────────┐ ┌───────────────┐ ┌───────────────┐
│   PostgreSQL  │ │    Storage    │ │   Job Queue   │
│    (SQLC)     │ │ (S3/Local)    │ │   (Redis)     │
└───────────────┘ └───────────────┘ └───────────────┘
```

## Getting Started

### Prerequisites

- [Nix](https://nixos.org/) package manager (recommended)
- PostgreSQL 15+
- Redis (optional, for job queue)

Enable Nix flakes:
```bash
echo "experimental-features = nix-command flakes" >> ~/.config/nix/nix.conf
```

### Setup

1. **Clone and enter dev environment**:
```bash
git clone https://github.com/denysvitali/immich-go-backend.git
cd immich-go-backend
nix develop
```

2. **Configure**:
```bash
cp config.yaml config.yaml.local
# Edit config.yaml.local with your settings
```

3. **Generate code and build**:
```bash
make setup
make build
```

4. **Run**:
```bash
./bin/immich-go-backend serve
# or
go run main.go serve
```

### Development Commands

```bash
make build          # Build binary
make test           # Run tests
make proto-gen      # Generate protobuf code
make sqlc-gen       # Generate SQL code
make lint           # Run linters
make ci-check       # Run all CI checks
```

## Configuration

Configuration is loaded from `config.yaml` with environment variable overrides using the pattern `IMMICH_SECTION_KEY`.

Key configuration sections:
- `server`: HTTP/gRPC ports
- `database`: PostgreSQL connection
- `storage`: Storage backend (local/s3/rclone)
- `auth`: JWT secrets and settings
- `jobs`: Background job processing

## Development History

This project was developed using AI assistance:
- **Claude Opus 4.5** - Architecture, complex implementations
- **Claude Sonnet 4** - Service implementations, bug fixes
- **[OpenHands](https://github.com/All-Hands-AI/OpenHands/)** - Automated development

The AI-assisted development approach allowed rapid prototyping but means the codebase lacks the rigor of traditional software development.

## Contributing

Contributions are welcome, especially:
- Security reviews and fixes
- Test coverage improvements
- Bug reports with reproduction steps
- Documentation improvements

Please open an issue before starting major work.

## License

AGPL-3.0 - Same as the original Immich project. See [LICENSE](./LICENSE).

## Disclaimer

This project is not affiliated with or endorsed by the Immich project. It is an independent reimplementation of the Immich backend API.
