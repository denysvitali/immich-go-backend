# Deployment

The binary is a single static Go executable (when built with `CGO_ENABLED=0`). Everything below assumes you have built it: `make build` produces `bin/immich-go-backend`.

## Table of contents

- [Quickstart](#quickstart)
- [Configuration](#configuration)
- [Fly.io single-machine demo](#flyio-single-machine-demo)
- [Systemd](#systemd)
- [Docker Compose](#docker-compose)
- [Reverse proxy](#reverse-proxy)
- [Operations](#operations)

---

## Quickstart

```bash
# 1. Dependencies (PostgreSQL 16 + Redis 7; the compose file ships only pg + redis — the backend is started separately)
docker compose up -d

# 2. Build the binary
make build

# 3. Apply migrations
./bin/immich-go-backend migrate

# 4. Start the server (foreground)
./bin/immich-go-backend serve
```

You'll see:

```
INFO  Starting gRPC server on 0.0.0.0:3002
INFO  Starting HTTP server on 0.0.0.0:3001
```

Verify with:

```bash
curl http://localhost:3001/api/server/ping
curl http://localhost:3001/api/server/version
```

Create the first admin:

```bash
curl -X POST http://localhost:3001/api/auth/admin-sign-up \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"changeme","name":"Admin"}'
```

## Configuration

`config.yaml` is the template. Most fields are overridden by unprefixed environment variables whose name is the upper-snake-case version of the YAML path — `server.address` → `SERVER_ADDRESS`, `database.url` → `DATABASE_URL`, `auth.jwt_secret` → `AUTH_JWT_SECRET`, `jobs.redis_url` → `JOBS_REDIS_URL`, and so on. The exceptions use an `IMMICH_` prefix: `IMMICH_WEBUI_DIR`, `IMMICH_DATABASE_AUTO_MIGRATE`, and `IMMICH_EMBEDDED_DB`. `config.yaml.local` is the standard local override file (gitignored). The authoritative list of fields is the struct tags in `internal/config/config.go`.

### Sections

| Section | Purpose |
|---------|---------|
| `server` | HTTP/gRPC bind, timeouts, CORS, metrics endpoint, request logging |
| `database` | DSN, pool sizing, auto-migrate flag |
| `storage` | Backend (`local` / `s3` / `rclone`); pre-signed URLs (S3 only); upload limits |
| `auth` | JWT secret/expiry, registration toggle, password policy, login rate-limit |
| `jobs` | asynq Redis URL, worker count |
| `telemetry` | OpenTelemetry tracing/metrics toggles, sampling rate |
| `features` | Boolean flags (`feature.machine_learning_enabled`, `feature.face_recognition_enabled`, `feature.clip_search_enabled`, `feature.video_transcoding_enabled`, `feature.thumbnail_generation_enabled`, `feature.exif_extraction_enabled`, `feature.duplicate_detection_enabled`, `feature.backup_sync_enabled`, `feature.sharing_enabled`, `feature.object_detection_enabled`) |
| `logging` | `level`, `format` (`json` / `text`), `output` |
| `webui_dir` | Directory containing a static frontend build (env: `IMMICH_WEBUI_DIR`) |

> The shipped `config.yaml` also lists `redis:`, `mail:`, and `machine_learning:` blocks. Those are not part of the `config.Config` struct and are not read by the binary — they're either aspirational or left over from earlier versions. Set equivalents under `jobs.redis_url` and the `feature.*_enabled` flags instead.

### Most-used environment variables

| Variable | Default | Notes |
|----------|---------|-------|
| `DATABASE_URL` | `postgres://immich:immich@localhost:5432/immich?sslmode=disable` | Full DSN |
| `DATABASE_AUTO_MIGRATE` | `true` | Run migrations during `serve`. Env name is `IMMICH_DATABASE_AUTO_MIGRATE`. Set to `false` for managed-DB prod |
| `AUTH_JWT_SECRET` | _required_ | **Required.** Set a 32+ byte secret in any non-demo deployment |
| `SERVER_ADDRESS` | `0.0.0.0:3001` (Go default `0.0.0.0:8080`) | REST / WebSocket listener |
| `SERVER_GRPC_ADDRESS` | `0.0.0.0:3002` (Go default `0.0.0.0:9090`) | gRPC listener (internal / private) |
| `STORAGE_BACKEND` | `local` | `local`, `s3`, or `rclone` |
| `STORAGE_LOCAL_ROOT` | `./uploads` | Where local backend writes |
| `UPLOAD_TEMP_DIR` | `/tmp/immich-uploads` | Scratch dir for in-flight uploads |
| `S3_BUCKET` / `S3_ENDPOINT` / `S3_REGION` / `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY` | — | S3 / S3-compatible backend |
| `S3_DIRECT_UPLOAD` | `false` | Hand clients pre-signed upload URLs |
| `IMMICH_WEBUI_DIR` | unset | If set, the binary serves this directory as static files at `/` |
| `IMMICH_EMBEDDED_DB` | unset | Set to `1`, `true`, or `yes` to start embedded PostgreSQL inside the binary |
| `JOBS_REDIS_URL` | unset | Set to e.g. `redis://localhost:6379/0` to enable the asynq job queue |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | unset | OTel traces/metrics destination |

The actual defaults come from the struct tags in `internal/config/config.go` — when in doubt, that file is authoritative.

---

## Fly.io single-machine demo

The fastest way to a public preview. A single Fly machine runs the full stack: backend + embedded Postgres + bundled Immich web UI. Persistent state lives on one Fly volume.

### How it fits together

```mermaid
flowchart LR
    subgraph VM["Fly machine: shared-cpu-2x / 2 GB"]
        WEB["Static web bundle<br/>/app/web"]
        BE["immich-go-backend :3001 / :3002"]
        PG["embedded Postgres<br/>127.0.0.1:5433"]
        BE -- DSN --> PG
        BE -- serve / --> WEB
    end

    VOL[("Fly volume: immich_data<br/>mounted at /data")]
    PG -. "writes /data/pg and /data/pg-bin" .-> VOL
    BE -. "writes /data/uploads /data/thumbs /data/profile /data/library /data/video" .-> VOL

    USER((User browser)) -- HTTPS 443 --> FLY["Fly edge TLS"]
    FLY -- ":3001" --> BE
```

### One-time setup

```bash
# 1. Pick a unique app name and update it in fly.toml (default: immich-go-demo)
fly apps create immich-go-demo

# 2. Create the persistent volume (10 GB is enough for a demo library)
fly volumes create immich_data --size 10 --region iad

# 3. JWT secret — never commit it
fly secrets set AUTH_JWT_SECRET="$(openssl rand -hex 32)"

# 4. Deploy
fly deploy
```

(The JWT secret env var is `AUTH_JWT_SECRET`, not `IMMICH_AUTH_JWT_SECRET` — the `IMMICH_` prefix is reserved for `IMMICH_WEBUI_DIR`, `IMMICH_DATABASE_AUTO_MIGRATE`, and `IMMICH_EMBEDDED_DB`.)

The first deploy takes a few minutes:

1. `Dockerfile.fly` stage 0 pulls `ghcr.io/immich-app/immich-server:${IMMICH_VERSION}` and copies `/build/www` → `/app/web`.
2. Stage 1 builds a static Go binary on Go 1.24-alpine.
3. Stage 2 assembles the Fly runtime image (alpine 3.20 + ca-certificates + tini + curl + non-root user).
4. On first `serve`, the binary downloads the ~30 MB embedded-postgres binary, initialises a cluster under `/data/pg`, runs migrations, then starts listening.

> **Which Dockerfile?** `Dockerfile.fly` is for the Fly demo (embedded Postgres, baked-in web bundle, tini). The base `Dockerfile` is leaner and targets external-DB deployments (Docker Compose, Kubernetes, plain Docker) — it doesn't bake in the web bundle and runs the binary directly without tini. Pick via `fly.toml` `[build].dockerfile` or `docker build -f Dockerfile.fly .`.

### Health checks and URLs

- Health: `GET /api/server/ping` (configured in `fly.toml` with a 120 s grace period).
- REST API: `https://<app>.fly.dev/api/...`
- Web UI: `https://<app>.fly.dev/`
- gRPC: `3002` is exposed only on the Fly private network; access it from another Fly app in the same org via 6PN.

### Environment variables in fly.toml

The `[env]` block sets non-secret defaults:

| Variable | Default | Purpose |
|----------|---------|---------|
| `IMMICH_WEBUI_DIR` | `/app/web` | Static frontend bundle |
| `IMMICH_EMBEDDED_DB` | `true` | Start embedded Postgres on `serve` |
| `IMMICH_DATABASE_AUTO_MIGRATE` | `true` | Run migrations during `serve` |
| `STORAGE_BACKEND` | `local` | `local` for demo; use `s3` in prod |
| `STORAGE_LOCAL_ROOT` | `/data/uploads` | Upload root on local backend |
| `UPLOAD_TEMP_DIR` | `/data/tmp` | Scratch for in-flight uploads |
| `SERVER_ADDRESS` | `0.0.0.0:3001` | REST / WebSocket bind |
| `SERVER_GRPC_ADDRESS` | `0.0.0.0:3002` | gRPC bind |

Override with `fly env set KEY=VALUE` or directly in `fly.toml`.

### Customising the frontend version

```bash
fly deploy --build-arg IMMICH_VERSION=v2.5.0
```

The Dockerfile defaults to `v2.4.0`. Pick a release whose proto schema this backend implements.

### Limitations

- **Single machine.** Don't scale horizontally — the embedded Postgres is local to the machine and asynq assumes a single Redis (or no jobs at all). For production point `DATABASE_URL` at a managed Postgres.
- **No ML service.** Face recognition / smart search fall back to their non-ML paths (the official Immich stack ships a separate ML service; this demo doesn't).
- **gRPC is internal-only.** Expose `:3002` via 6PN if you want a mobile app on the same Fly org to connect via gRPC.

### Troubleshooting

**Health check fails for the first 2 minutes.** Expected — embedded PG is downloading its binary and initialising. Tail logs:

```bash
fly logs
```

Look for `embedded postgres ready` followed by `Starting HTTP server on 0.0.0.0:3001`.

**`AUTH_JWT_SECRET is required`.** Set it via `fly secrets set AUTH_JWT_SECRET=...` and redeploy.

**Frontend shows 404 on `/`.** Confirm the bundle exists:

```bash
fly ssh console -C "ls /app/web"
```

If empty, the upstream `immich-server:${IMMICH_VERSION}` image no longer ships `/build/www` — pin to a known version.

---

## Systemd

For a bare-metal or VM deployment without Docker.

### Service file

`/etc/systemd/system/immich-go-backend.service`:

```ini
[Unit]
Description=Immich Go Backend
After=network.target postgresql.service redis.service
Wants=postgresql.service redis.service

[Service]
Type=simple
User=immich
Group=immich
WorkingDirectory=/opt/immich-go-backend
EnvironmentFile=/opt/immich-go-backend/config.env
ExecStartPre=/opt/immich-go-backend/bin/immich-go-backend migrate
ExecStart=/opt/immich-go-backend/bin/immich-go-backend serve
Restart=always
RestartSec=5
TimeoutStopSec=30s

# Hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/opt/immich-go-backend/data

[Install]
WantedBy=multi-user.target
```

`/opt/immich-go-backend/config.env`:

```bash
DATABASE_URL=postgres://immich:CHANGE_ME@localhost:5432/immich?sslmode=disable
AUTH_JWT_SECRET=CHANGE_ME_32_BYTES
SERVER_ADDRESS=0.0.0.0:3001
SERVER_GRPC_ADDRESS=0.0.0.0:3002
IMMICH_WEBUI_DIR=/opt/immich-go-backend/web
STORAGE_LOCAL_ROOT=/opt/immich-go-backend/data/uploads
UPLOAD_TEMP_DIR=/opt/immich-go-backend/data/tmp
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now immich-go-backend
sudo journalctl -u immich-go-backend -f
```

---

## Docker Compose

The included `docker-compose.yml` ships only `postgres` and `redis` — there is no `backend` service in the file. Run the server in a separate container or on the host. Below is an illustrative **sample full-stack compose** you can use as a starting point for your own deployment (not what's shipped in this repo):

```yaml
# Sample full-stack compose — illustrative, not the file shipped in the repo.
# The repo's docker-compose.yml only provides postgres + redis.
services:
  backend:
    image: ghcr.io/your-org/immich-go-backend:latest
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://immich:immich@postgres:5432/immich?sslmode=disable
      AUTH_JWT_SECRET: ${AUTH_JWT_SECRET}
      IMMICH_WEBUI_DIR: /app/web
      STORAGE_BACKEND: local
      STORAGE_LOCAL_ROOT: /data/uploads
    volumes:
      - uploads:/data
    ports:
      - "3001:3001"
      - "3002:3002"
    restart: unless-stopped

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: immich
      POSTGRES_PASSWORD: immich
      POSTGRES_DB: immich
      POSTGRES_EXTENSIONS: "uuid-ossp,vector,earthdistance"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U immich"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redisdata:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  pgdata:
  redisdata:
  uploads:
```

```yaml
services:
  backend:
    image: ghcr.io/your-org/immich-go-backend:latest
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://immich:immich@postgres:5432/immich?sslmode=disable
      AUTH_JWT_SECRET: ${AUTH_JWT_SECRET}
      IMMICH_WEBUI_DIR: /app/web
      STORAGE_LOCAL_ROOT: /data/uploads
    volumes:
      - uploads:/data
    ports:
      - "3001:3001"
      - "3002:3002"
    restart: unless-stopped

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: immich
      POSTGRES_PASSWORD: immich
      POSTGRES_DB: immich
      POSTGRES_EXTENSIONS: "uuid-ossp,vector,vchord,cube,earthdistance,pg_trgm,unaccent"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U immich"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redisdata:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  pgdata:
  redisdata:
  uploads:
```

---

## Reverse proxy

### Nginx

```nginx
server {
    listen 80;
    server_name photos.example.com;
    client_max_body_size 50M;

    location / {
        proxy_pass http://127.0.0.1:3001;
        proxy_http_version 1.1;

        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket (Immich live sync)
        proxy_set_header Upgrade    $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 1h;
    }
}
```

For TLS, use `certbot --nginx -d photos.example.com` or terminate upstream.

### Caddy

```
photos.example.com {
    reverse_proxy 127.0.0.1:3001
    encode gzip zstd
}
```

Caddy terminates TLS automatically via Let's Encrypt.

---

## Operations

### Health

```bash
curl -fsS http://localhost:3001/api/server/ping     # REST liveness
curl -fsS http://localhost:3001/api/server/version  # build metadata
```

### Metrics

`/metrics` is exposed by default (Prometheus exposition — `server.metrics_enabled` defaults to `true`). Disable with `server.metrics_enabled: false`. Scrape with Prometheus or any OTLP-compatible collector.

### Logs

Structured JSON via logrus. Pipe through `jq` for ad-hoc inspection:

```bash
./bin/immich-go-backend serve 2>&1 | jq -c 'select(.level=="error")'
```

### Backups

PostgreSQL:

```bash
pg_dump -U immich -Fc immich > backup.dump
pg_restore -U immich -d immich --clean --if-exists backup.dump
```

Local-storage backend:

```bash
rsync -a /opt/immich-go-backend/data/uploads/ backup/uploads/
```

If you're on S3, use the bucket's native versioning / lifecycle rules.

### Upgrades

1. Pull the new release (image, binary, or commit).
2. Run `./immich-go-backend migrate` (or set `IMMICH_DATABASE_AUTO_MIGRATE=true` and let `serve` do it).
3. Restart the service. Old and new versions can briefly coexist behind a reverse proxy if you need zero-downtime — the wire protocol is the same.

### Troubleshooting

| Symptom | Likely cause |
|---------|--------------|
| Server won't start: `AUTH_JWT_SECRET is required` | Set the env var; the default in `config.yaml` is a placeholder, not a usable secret. |
| `pq: extension "vector" is not available` | Install `postgresql-16-pgvector` (Debian/Ubuntu) or build from source; the official Immich image bundles it. |
| `pq: relation "migrations" does not exist` | First-time boot — `migrate` creates it automatically. |
| WebSocket connections drop every 30 s | Reverse proxy `proxy_read_timeout` too low; raise to 1h. |
| Upload 413 | `client_max_body_size` (nginx) or `--data-proxy-body-size` (Caddy) too small. |
| Embedded-PG demo fails to start | `/data/pg` is unwritable or the volume isn't mounted — check `fly ssh console -C "ls -la /data"`. |
| `asynq: redis is not configured` | Set `jobs.enabled: true` and `jobs.redis_url: redis://...`; otherwise background work degrades silently. |