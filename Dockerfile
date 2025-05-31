# Multi-stage Dockerfile for immich-go-backend
# Uses Nix for build environment and creates a statically linked binary

# Stage 1: Build environment using Nix
FROM nixos/nix:latest AS builder

# Enable flakes and install git (needed for flakes)
RUN echo "experimental-features = nix-command flakes" >> /etc/nix/nix.conf

# Install git for flake support
RUN nix-env -iA nixpkgs.git

# Set working directory
WORKDIR /app

# Copy Nix configuration files first for better caching
COPY flake.nix flake.lock shell.nix ./

# Copy the entire project (Nix needs access to all files for the flake)
COPY . .

# Build the application using the Nix development environment
RUN nix develop --command bash -c "
    echo '🔍 Verifying tools are available...' && \
    which protoc protoc-gen-go protoc-gen-go-grpc buf && \
    echo '🔨 Generating protocol buffers...' && \
    ./scripts/generate-protos.sh && \
    echo '📦 Building application with static linking...' && \
    CGO_ENABLED=0 GOOS=linux go build \
        -a -installsuffix cgo \
        -ldflags '-extldflags \"-static\" -s -w' \
        -o immich-go-backend \
        .
"

# Stage 2: Create minimal runtime image
# Create a non-root user directly in the builder stage
RUN adduser -D -s /bin/sh -u 1001 appuser

# Final stage
FROM scratch

# Copy user information
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy SSL certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /app/immich-go-backend /immich-go-backend

# Switch to non-root user
USER 1001:1001

# Expose the default port (adjust if needed)
EXPOSE 8080

# Set the entrypoint
ENTRYPOINT ["/immich-go-backend"]