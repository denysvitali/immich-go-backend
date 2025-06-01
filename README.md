# immich-go-backend

A high-performance alternative backend for [Immich](https://github.com/immich-app/immich) written in **Go**, designed for enhanced performance and S3-first architecture.

## 🚀 Why This Project?

This project aims to provide a more performant alternative to the original Immich backend with specific focus on:

- **Performance**: Go's superior concurrency model and performance characteristics
- **S3-First Architecture**: Native support for object storage with pre-signed URLs for direct client uploads/downloads
- **Cloud-Native Design**: Built with modern cloud storage patterns in mind
- **Scalability**: Designed to handle larger datasets and concurrent operations more efficiently

## 🛠️ Technology Stack

This backend is built with modern Go technologies and cloud-native patterns:

- **Language**: Go 1.24+
- **Database**: PostgreSQL with [SQLC](https://sqlc.dev/) for type-safe SQL queries
- **API**: Protocol Buffers (protobuf) with [gRPC](https://grpc.io/) and [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) for REST compatibility
- **Storage**: Multi-backend support (Local, S3, Rclone) with pre-signed URL capabilities
- **Observability**: OpenTelemetry for comprehensive tracing and metrics
- **Configuration**: Viper for flexible configuration management
- **Authentication**: JWT-based authentication with bcrypt password hashing

## 🏗️ Architecture

The project follows a clean architecture pattern with:

- **Storage Abstraction Layer**: Universal storage interface supporting local filesystem, S3, and rclone backends
- **Service Layer**: Business logic separated into domain-specific services (auth, users, assets, albums)
- **Protocol Buffers**: Type-safe API definitions with automatic REST gateway generation
- **Database Layer**: SQLC-generated type-safe database operations
- **Telemetry**: Comprehensive observability with OpenTelemetry

## ⚠️ Project Status

**This project is currently a Work In Progress (WIP) and is NOT ready for production use.**

See the [ROADMAP.md](./ROADMAP.md) for detailed progress tracking. Currently implemented:
- ✅ Database schema and SQLC queries
- ✅ Protocol buffer definitions and code generation
- ✅ Storage abstraction layer with S3 support
- ✅ Configuration and telemetry systems
- ✅ Authentication service
- ✅ User management service
- 🔄 Asset management service (in progress)
- 🔄 Album management service (pending)
- 🔄 HTTP/gRPC controllers (pending)

## 🤖 Development

This project was developed mostly with:
- **Claude Sonnet 4**
- **[OpenHands (All-Hands-AI)](https://github.com/All-Hands-AI/OpenHands/)**

## 📋 Prerequisites

- [Nix](https://nixos.org/) package manager (recommended)
- PostgreSQL 15+ (can be managed through Nix or installed separately)

**Note**: This project uses Nix flakes to manage the development environment. You need to enable experimental features:
```bash
# Enable flakes and nix-command experimental features
echo "experimental-features = nix-command flakes" >> ~/.config/nix/nix.conf
# Or temporarily enable them:
# nix --experimental-features "nix-command flakes" develop
```

The Nix environment automatically provides:
- Go 1.24+
- Protocol Buffers compiler (protoc)
- Buf CLI tool (for protobuf management)
- SQLC for type-safe SQL code generation
- Other development tools and dependencies

## 🚀 Getting Started

This project uses [Nix](https://nixos.org/) to manage the development environment, ensuring all developers have the same tools and dependencies.

1. **Clone the repository**:
```bash
git clone https://github.com/denysvitali/immich-go-backend.git
cd immich-go-backend
```

2. **Enter the Nix development environment**:
```bash
nix develop
# or alternatively:
make dev-shell
```
This will automatically install and make available all required tools including Go, protoc, buf, sqlc, and other dependencies.

3. **Set up your configuration**:
Copy and modify the configuration file according to your environment:
```bash
cp config.yaml config.yaml.local
# Edit config.yaml.local with your database credentials, S3 settings, etc.
```

4. **Initialize and generate code**:
```bash
make setup
```
This command will:
- Download Go module dependencies
- Generate protocol buffer code
- Generate type-safe SQL code with SQLC

5. **Start the server**:
```bash
go run main.go serve
```

**Available Make targets**: Run `make help` to see all available development commands.

## 🤝 Contributing

Contributions are welcome! Please read the [ROADMAP.md](./ROADMAP.md) to understand the current development priorities.

## 📄 License

This project is licensed under the same terms as the original Immich project: AGPL-3.0. See the [LICENSE](./LICENSE) file for details.
