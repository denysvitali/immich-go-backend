# Immich Go Backend - Deployment Guide

## Quick Start

### Prerequisites
- Go 1.21+ (for building)
- PostgreSQL 14+
- Redis 6+ (optional, for job queue)
- Docker & Docker Compose (optional, for dependencies)

### 1. Start Dependencies

Using Docker Compose:
```bash
docker-compose up -d postgres redis
```

Or manually:
- PostgreSQL: `postgres://immich:immich@localhost:5432/immich`
- Redis: `redis://localhost:6379/0`

### 2. Build the Backend

```bash
# Build the binary
go build -o bin/immich-go-backend ./cmd

# Or using Make (requires Nix)
make build
```

### 3. Run Database Migrations

```bash
./bin/immich-go-backend migrate
```

### 4. Start the Server

```bash
./bin/immich-go-backend serve
```

The server will start on:
- HTTP/REST API: `http://localhost:3001`
- gRPC API: `http://localhost:3002`

### 5. Test the Installation

```bash
# Run the test script
./scripts/test-server.sh

# Or manually test endpoints
curl http://localhost:3001/api/server/ping
curl http://localhost:3001/api/server/version
```

## Configuration

Edit `config.yaml` to customize:

```yaml
server:
  address: "0.0.0.0:3001"  # HTTP/REST port
  grpc_address: "0.0.0.0:3002"  # gRPC port

database:
  url: "postgres://immich:immich@localhost:5432/immich?sslmode=disable"

auth:
  jwt_secret: "change-this-in-production"  # ⚠️ IMPORTANT: Change this!

storage:
  upload_location: "./uploads"
  library_location: "./library"
```

### Environment Variables

Override config with environment variables:
```bash
export IMMICH_DATABASE_URL="postgres://user:pass@host/db"
export IMMICH_AUTH_JWT_SECRET="your-secret-key"
export IMMICH_SERVER_ADDRESS="0.0.0.0:8080"
```

## Testing with Immich Clients

### Mobile App
1. Install the Immich mobile app
2. Open settings and set server URL to `http://your-server-ip:3001`
3. Create an admin account on first login
4. Start uploading photos!

### Web App
1. Configure Immich web to use `http://your-server-ip:3001` as the API URL
2. Login with your credentials
3. All features should work (except ML-dependent features)

## Production Deployment

### Using Docker

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o immich-go-backend ./cmd

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/immich-go-backend .
COPY config.yaml .
EXPOSE 3001 3002
CMD ["./immich-go-backend", "serve"]
```

### Using Systemd

Create `/etc/systemd/system/immich-go-backend.service`:

```ini
[Unit]
Description=Immich Go Backend
After=network.target postgresql.service redis.service

[Service]
Type=simple
User=immich
WorkingDirectory=/opt/immich-go-backend
ExecStartPre=/opt/immich-go-backend/bin/immich-go-backend migrate
ExecStart=/opt/immich-go-backend/bin/immich-go-backend serve
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### Reverse Proxy (Nginx)

```nginx
server {
    listen 80;
    server_name photos.example.com;
    client_max_body_size 50M;

    location / {
        proxy_pass http://localhost:3001;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## Feature Status

### ✅ Working Features
- User authentication and management
- Asset upload/download
- Album management
- Thumbnail generation
- Video streaming
- Shared links
- Search (metadata-based)
- Timeline browsing
- Multi-user support
- WebSocket notifications

### ⚠️ Limited Features (No ML Backend)
- Face detection/recognition (returns empty results)
- Smart search (basic text search only)
- Object detection (not available)
- Duplicate detection (basic checksum only)

### 🔧 Configuration Options

To disable ML features completely:
```yaml
machine_learning:
  enabled: false
```

## Troubleshooting

### Database Connection Failed
- Check PostgreSQL is running: `pg_isready -h localhost -p 5432`
- Verify credentials in config.yaml
- Check database exists: `psql -U immich -c "SELECT 1"`

### Server Won't Start
- Check port 3001 is free: `lsof -i :3001`
- Verify config.yaml syntax
- Check logs: `./bin/immich-go-backend serve --log-level=debug`

### Upload Failures
- Check storage permissions: `ls -la ./uploads`
- Verify disk space: `df -h`
- Check client_max_body_size in reverse proxy

### Authentication Issues
- Ensure JWT secret is set and consistent
- Check token expiry settings
- Verify CORS settings for web clients

## Performance Tuning

### Database
```yaml
database:
  max_open_conns: 100  # Increase for high load
  max_idle_conns: 25
```

### Job Queue
```yaml
jobs:
  workers: 8  # Number of concurrent workers
  concurrency: 20  # Jobs processed simultaneously
```

### Storage
- Use S3-compatible storage for scalability
- Enable CDN for thumbnail delivery
- Configure pre-signed URLs for direct uploads

## Monitoring

### Health Check
```bash
curl http://localhost:3001/api/server/ping
```

### Metrics Endpoint
```bash
curl http://localhost:3001/metrics
```

### Logs
- Application logs: Follow stdout/stderr
- Access logs: Configure in reverse proxy
- Error logs: Check systemd journal

## Backup & Recovery

### Database Backup
```bash
pg_dump -U immich immich > backup.sql
```

### File Backup
```bash
tar -czf immich-files.tar.gz uploads/ library/ thumbs/
```

### Restore
```bash
psql -U immich immich < backup.sql
tar -xzf immich-files.tar.gz
```

## Support

- GitHub Issues: [Report bugs or request features]
- Documentation: Check TODO.md for implementation status
- Compatibility: ~96% Immich API compatible

## License

See LICENSE file for details.

## Demo Deployment on Fly.io

A single-machine demo Fly app runs the full stack — backend, frontend,
and an embedded PostgreSQL — on one machine. Use it to host a public
preview without provisioning a managed Postgres or a separate web app.

### What you get

- `immich-go-backend` binary serving REST on `:3001` and gRPC on `:3002`
- An embedded Postgres cluster (`github.com/fergusstrange/embedded-postgres`)
  started by the binary on `127.0.0.1:5433`, with the cluster and cached
  binaries persisted on a Fly volume
- The official Immich web bundle (from `ghcr.io/immich-app/immich-server`)
  baked into the Docker image and served as static files from `/`

### Prerequisites

- [`flyctl`](https://fly.io/docs/hands-on/install-flyctl/) ≥ the latest
  stable release
- A Fly account (`fly auth signup`)
- A unique Fly app name (used as the URL — e.g. `immich-go-demo.fly.dev`)

### One-time setup

```bash
# 1. Pick a unique app name and update it in fly.toml.
#    The included fly.toml uses `immich-go-demo`.
fly apps create immich-go-demo

# 2. Create the persistent volume that holds the embedded Postgres
#    cluster, cached Postgres binaries, and uploaded media.
fly volumes create immich_data --size 10 --region iad

# 3. Set the JWT secret as a Fly secret (never commit it).
fly secrets set AUTH_JWT_SECRET="$(openssl rand -hex 32)"

# 4. Deploy.
fly deploy
```

The first deploy takes a few minutes: the Dockerfile pulls the official
Immich server image to extract its web bundle, downloads the ~30 MB
embedded Postgres binary on startup, initialises a fresh cluster, runs
migrations, and then starts serving.

### How it works

- `Dockerfile` stage 0 is `FROM ghcr.io/immich-app/immich-server:<tag>`
  — the official Immich server image bundles the SvelteKit web build at
  `/build/www` (see `immich-app/immich` `server/Dockerfile`). The final
  stage copies that bundle to `/app/web`.
- The `Dockerfile` final stage is `alpine:3.20` (not `scratch`) so the
  embedded Postgres binary — which the library downloads and `exec`s at
  startup — has the libc it expects.
- On `serve`, when `IMMICH_EMBEDDED_DB=true` is set, the binary
  (`internal/embedded/postgres.go`) starts an embedded Postgres that
  binds to `127.0.0.1:5433` and writes its cluster under
  `/data/pg`. The cached Postgres binaries live under `/data/pg-bin` so
  subsequent restarts skip the download.
- The HTTP handler is wrapped by `internal/webui.Handler` which serves
  files from `IMMICH_WEBUI_DIR=/app/web`. API and websocket routes are
  matched first; extension-less paths fall back to `index.html` for SPA
  routing.
- The `[[mounts]]` block in `fly.toml` mounts a single named volume
  (`immich_data`) at `/data`, covering both Postgres data and
  user-uploaded media.

### Environment variables

The `fly.toml` `[env]` block sets the non-secret defaults. Override
with `flyctl env set` or in `fly.toml`:

| Variable                    | Default                                  | Purpose                                                |
|-----------------------------|------------------------------------------|--------------------------------------------------------|
| `IMMICH_WEBUI_DIR`          | `/app/web`                               | Directory containing the static frontend bundle        |
| `IMMICH_EMBEDDED_DB`        | `true`                                   | Start the embedded Postgres on `serve`                 |
| `IMMICH_DATABASE_AUTO_MIGRATE` | `true`                                | Run schema migrations during `serve` (default on)      |
| `STORAGE_BACKEND`           | `local`                                  | Storage backend (`local` for demo, `s3` for production)|
| `STORAGE_LOCAL_ROOT`        | `/data/uploads`                          | Where uploads are stored on the local backend          |
| `UPLOAD_TEMP_DIR`           | `/data/tmp`                              | Scratch directory for in-flight uploads                |
| `SERVER_ADDRESS`            | `0.0.0.0:3001`                           | REST/gRPC-gateway bind address                         |
| `SERVER_GRPC_ADDRESS`       | `0.0.0.0:3002`                           | gRPC bind address                                      |
| `AUTH_JWT_SECRET`           | —                                        | **Required.** Set as a Fly secret.                     |

### Health checks and URLs

- Health: `GET /api/server/ping` (configured in `fly.toml`'s
  `[[services.http_checks]]`).
- REST API: `https://<app-name>.fly.dev/api/...`
- Web UI: `https://<app-name>.fly.dev/` — open in a browser to log in
  and use the demo.

### Customising the frontend version

To pin a different Immich web version, pass the `IMMICH_VERSION` build
arg on the command line:

```bash
fly deploy --build-arg IMMICH_VERSION=v2.5.0
```

The Dockerfile defaults to `v2.4.0`. Note that the backend's gRPC API
surface is what the frontend talks to, so the frontend version should
match a release whose proto schema this backend implements.

### Limitations

This is a **demo** configuration:

- **Single machine only.** Don't scale to multiple machines — the
  embedded Postgres is local to the machine and the asynq job queue
  expects a single-process Redis (or no jobs at all). For production,
  point `DATABASE_URL` at a managed Postgres and run the embedded
  mode only when you need a one-shot preview.
- **No ML/face-recognition.** The ML backend is a separate service in
  the official Immich stack; this demo doesn't ship one. Search and
  duplicates fall back to their non-ML paths.
- **gRPC port 3002 is internal-only.** Expose it via Fly's private
  network if you want a mobile app to connect via gRPC; the public
  HTTPS endpoint is the REST gateway at 3001.

### Troubleshooting

**Machine boots but health check fails for 2+ minutes.** Expected on
first deploy — the embedded Postgres is downloading its binary and
initialising the cluster. Tail the logs:

```bash
fly logs
```

You should see `embedded postgres ready` followed by `Starting HTTP
server on 0.0.0.0:3001`. After that, the health check passes.

**`AUTH_JWT_SECRET is required`.** Set it via `fly secrets set
AUTH_JWT_SECRET=...` and redeploy.

**Frontend shows 404 on `/`.** Confirm `IMMICH_WEBUI_DIR=/app/web`
points at a directory containing `index.html`. Inspect the image:

```bash
fly ssh console -C "ls /app/web"
```
