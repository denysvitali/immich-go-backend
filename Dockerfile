# Base Dockerfile for immich-go-backend.
#
# Target environment: any container runtime that talks to an external
# PostgreSQL and Redis (Docker Compose, Kubernetes, plain Docker, etc.).
# This image is published to ghcr.io as the project's "base" image.
#
# For the single-machine Fly.io demo (embedded Postgres + baked-in web
# bundle + tini for SIGTERM forwarding), use Dockerfile.fly instead.
#
# Notes:
#   * Runtime is alpine (not scratch) so the Go binary can exec helper
#     processes when needed; switch to distroless/static if you want a
#     ~10 MB image and don't run embedded-postgres.
#   * Web frontend is NOT baked in — deploy immich-web separately or
#     mount your own bundle at /app/web and set IMMICH_WEBUI_DIR.
#   * Database / Redis are NOT bundled — point DATABASE_URL and the
#     Redis env vars at external services.

# ---------- Stage 1: build ----------
FROM golang:1.24-alpine AS builder

# Build deps: git for `go mod`, curl + ca-certificates to fetch buf.
RUN apk add --no-cache git ca-certificates curl

# Pin buf to a known-good version. Bump deliberately.
ARG BUF_VERSION=1.70.0
RUN arch="$(apk --print-arch)" && \
    case "$arch" in \
      x86_64)  buf_arch=Linux-x86_64   ;; \
      aarch64) buf_arch=Linux-aarch64  ;; \
      *) echo "unsupported arch: $arch" && exit 1 ;; \
    esac && \
    curl -fsSL "https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-${buf_arch}" \
      -o /usr/local/bin/buf && \
    chmod +x /usr/local/bin/buf && \
    buf --version

# Go protoc plugins buf invokes when committed gen/ is missing or stale.
# Pinned to the same versions used in .github/workflows/go.yaml.
RUN GOBIN=/usr/local/bin go install \
      google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6 && \
    GOBIN=/usr/local/bin go install \
      google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1 && \
    GOBIN=/usr/local/bin go install \
      github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.27.1

WORKDIR /src

# Cache go.mod/go.sum first so dependency downloads are reused across builds.
COPY go.mod go.sum ./
RUN go mod download

# Now copy the rest of the source. Generated protos are committed, so
# `buf generate` is a no-op when internal/proto/gen/ is populated.
COPY . .

# Idempotent proto regen, then a static, stripped binary.
RUN if [ -d "internal/proto/gen" ] && \
       [ "$(find internal/proto/gen -name '*.pb.go' 2>/dev/null | wc -l)" -gt 0 ]; then \
      echo "protos already generated, skipping buf"; \
    else \
      echo "generating protos with buf"; \
      buf generate; \
    fi && \
    CGO_ENABLED=0 GOOS=linux go build \
      -a -installsuffix cgo \
      -ldflags "-extldflags \"-static\" -s -w" \
      -o /out/immich-go-backend \
      ./cmd

# ---------- Stage 2: minimal runtime ----------
FROM alpine:3.20

# ca-certificates for outbound HTTPS. Non-root user keeps the runtime
# unprivileged by default — override the UID/GID at build time if your
# orchestrator requires different ones.
RUN apk add --no-cache ca-certificates \
 && adduser -D -s /bin/sh -u 1001 appuser \
 && mkdir -p /app /data \
 && chown -R appuser:appuser /app /data

# Static binary.
COPY --from=builder --chown=appuser:appuser /out/immich-go-backend /app/immich-go-backend

USER appuser
WORKDIR /app

# REST + gRPC ports. Configure via SERVER_ADDRESS / SERVER_GRPC_ADDRESS.
EXPOSE 3001 3002

# Default to running migrations + serving. Orchestrators (Compose,
# Kubernetes) should signal SIGTERM for graceful shutdown.
ENTRYPOINT ["/app/immich-go-backend"]
CMD ["serve"]