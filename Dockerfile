# Multi-stage Dockerfile for immich-go-backend
# - builder:  golang:1.24-alpine + buf CLI (GitHub release) to (re)generate protos
# - runtime:  scratch with a static binary and CA certs
#
# The protos under internal/proto/gen/ are committed, so buf generate is a
# no-op when those files exist. We keep the buf install as a safety net so
# the image still builds from a clean checkout without the generated files.

# ---------- Stage 1: build ----------
FROM golang:1.24-alpine AS builder

# Install build deps in one layer. git is needed by `go mod` for some modules;
# ca-certificates is needed for `go install` to talk to proxy.golang.org / github.com.
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

# ---------- Stage 2: minimal user + certs ----------
FROM alpine:3.20 AS runtime-base
RUN adduser -D -s /bin/sh -u 1001 appuser

# ---------- Stage 3: final scratch image ----------
FROM scratch

COPY --from=runtime-base /etc/passwd       /etc/passwd
COPY --from=runtime-base /etc/group        /etc/group
COPY --from=runtime-base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /out/immich-go-backend /immich-go-backend

USER 1001:1001
EXPOSE 8080
ENTRYPOINT ["/immich-go-backend"]
