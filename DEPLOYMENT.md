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