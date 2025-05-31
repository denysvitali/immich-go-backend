# Docker Setup for Immich Go Backend

This document describes the Docker setup for building and running the Immich Go Backend application.

## Overview

The Docker setup uses a multi-stage build process:

1. **Builder Stage**: Uses `nixos/nix:latest` with the project's Nix flake to create a reproducible build environment
2. **Runtime Stage**: Uses `scratch` for a minimal, secure runtime image

## Features

- ✅ **Multi-architecture support**: Builds for both `linux/amd64` and `linux/arm64`
- ✅ **Statically linked binary**: No runtime dependencies required
- ✅ **Non-root execution**: Runs as user ID 1001 for security
- ✅ **Minimal attack surface**: Uses `scratch` base image
- ✅ **Reproducible builds**: Uses Nix for consistent build environment
- ✅ **Protocol buffer generation**: Automatically generates protobuf files during build

## Building Locally

### Prerequisites

- Docker with BuildKit support
- Multi-architecture support (for cross-platform builds)

### Build Commands

```bash
# Build for current architecture
docker build -t immich-go-backend .

# Build for multiple architectures (requires buildx)
docker buildx build --platform linux/amd64,linux/arm64 -t immich-go-backend .

# Test the build
./test-docker-build.sh
```

## GitHub Actions CI/CD

The project includes a GitHub Action (`.github/workflows/docker.yaml`) that:

- Builds multi-architecture Docker images
- Pushes to GitHub Container Registry (`ghcr.io`)
- Creates attestations for supply chain security
- Uses the same image tag for both architectures (manifest list)

### Image Tags

The CI system creates the following tags:

- `latest` - Latest build from the default branch
- `main` - Latest build from main branch
- `v1.2.3` - Semantic version tags
- `main-abc1234` - Branch name with commit SHA

### Registry

Images are published to: `ghcr.io/denysvitali/immich-go-backend`

## Running the Container

```bash
# Run with default settings
docker run --rm -p 8080:8080 ghcr.io/denysvitali/immich-go-backend:latest

# Run with custom configuration
docker run --rm -p 8080:8080 \
  -v /path/to/config.yaml:/config.yaml \
  ghcr.io/denysvitali/immich-go-backend:latest \
  --config /config.yaml

# Run with environment variables
docker run --rm -p 8080:8080 \
  -e DATABASE_URL=postgres://user:pass@host:5432/db \
  ghcr.io/denysvitali/immich-go-backend:latest
```

## Security Considerations

- The container runs as a non-root user (UID 1001)
- Uses a minimal `scratch` base image with no shell or package manager
- Binary is statically linked with no runtime dependencies
- SSL certificates are included for HTTPS requests

## Troubleshooting

### Build Issues

1. **Nix flake evaluation fails**: Ensure `flake.nix` and `flake.lock` are valid
2. **Protocol buffer generation fails**: Check that all required `.proto` files are present
3. **Go build fails**: Verify `go.mod` and `go.sum` are up to date

### Runtime Issues

1. **Permission denied**: Ensure the container has access to required files/directories
2. **Port binding fails**: Check that the port is not already in use
3. **Configuration not found**: Verify volume mounts and file paths

## Development

For local development, use the Nix development environment:

```bash
# Enter Nix shell
nix develop

# Or using legacy nix-shell
nix-shell

# Generate protobuf files
make proto-gen

# Build locally
make build
```

## File Structure

```
.
├── Dockerfile              # Multi-stage Docker build
├── .dockerignore           # Files to exclude from Docker context
├── docker-compose.yml      # Docker Compose setup (if present)
├── test-docker-build.sh    # Local build testing script
└── .github/workflows/
    └── docker.yaml         # GitHub Actions workflow
```