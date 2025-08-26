# Immich Go Backend - Testing Guide

## Quick Start

### Prerequisites
- PostgreSQL 16+ with extensions: uuid-ossp, vectors, earthdistance
- Redis 7+ (for job queue)
- Go 1.21+ (for building from source)

### Option 1: Using Docker Compose (Recommended)

```bash
# Start PostgreSQL and Redis
docker compose up -d

# Run database migrations
./bin/immich-go-backend migrate

# Start the server
./bin/immich-go-backend serve
```

### Option 2: Manual Setup

1. **Install PostgreSQL and Redis:**
```bash
# Ubuntu/Debian
sudo apt install postgresql-16 redis-server

# macOS
brew install postgresql@16 redis
```

2. **Create database and user:**
```sql
CREATE USER immich WITH PASSWORD 'immich';
CREATE DATABASE immich OWNER immich;
\c immich
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vectors";
CREATE EXTENSION IF NOT EXISTS "earthdistance";
```

3. **Configure connection in config.yaml:**
```yaml
database:
  url: "postgresql://immich:immich@localhost:5432/immich"
redis:
  url: "redis://localhost:6379"
```

4. **Build and run:**
```bash
# Build the binary
go build -o bin/immich-go-backend ./cmd

# Run migrations
./bin/immich-go-backend migrate

# Start server
./bin/immich-go-backend serve
```

## Testing with Immich Apps

### Server Endpoints
- HTTP API: `http://localhost:8080/api`
- gRPC: `localhost:9090`
- WebSocket: `http://localhost:8080/api/socket.io/`

### Mobile App Configuration
1. Open Immich mobile app
2. Go to Settings → Server URL
3. Enter: `http://your-server-ip:8080/api`
4. Use test credentials or create admin account

### Web App Configuration
1. Set environment variable: `IMMICH_SERVER_URL=http://localhost:8080`
2. Or configure in web app settings

### Initial Admin Setup
```bash
# Create admin user via API
curl -X POST http://localhost:8080/api/auth/admin-sign-up \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "adminpassword",
    "name": "Admin User"
  }'
```

## API Testing

### Test Authentication
```bash
# Login
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"adminpassword"}'

# Save the access_token from response
export TOKEN="your-access-token"
```

### Test Asset Upload
```bash
curl -X POST http://localhost:8080/api/assets \
  -H "Authorization: Bearer $TOKEN" \
  -F "assetData=@/path/to/image.jpg" \
  -F "deviceId=test-device" \
  -F "deviceAssetId=unique-id-123" \
  -F "fileCreatedAt=2024-01-01T00:00:00Z" \
  -F "fileModifiedAt=2024-01-01T00:00:00Z"
```

### Test Server Info
```bash
curl http://localhost:8080/api/server/version
```

## Monitoring

### View Logs
```bash
# Server logs (if running in foreground)
./bin/immich-go-backend serve

# Or check system logs
journalctl -u immich-go-backend -f
```

### Check Database
```bash
psql -U immich -d immich -c "SELECT COUNT(*) FROM users;"
psql -U immich -d immich -c "SELECT COUNT(*) FROM assets;"
```

### Redis Monitoring
```bash
redis-cli monitor
```

## Troubleshooting

### Connection Refused
- Ensure PostgreSQL and Redis are running
- Check firewall settings
- Verify connection strings in config.yaml

### Migration Errors
- Ensure database user has CREATE permissions
- Check PostgreSQL extensions are installed
- Review migration logs for specific errors

### Authentication Issues
- Verify JWT secret is configured
- Check token expiration settings
- Ensure cookies are enabled for web access

### Asset Upload Failures
- Check storage path permissions
- Verify available disk space
- Review asset processing logs

## Performance Testing

### Load Testing with Apache Bench
```bash
# Test authentication endpoint
ab -n 1000 -c 10 -T application/json \
  -p login.json http://localhost:8080/api/auth/login

# Test asset listing
ab -n 1000 -c 10 -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/assets
```

### Monitor Resources
```bash
# CPU and memory usage
htop

# Database connections
psql -U immich -c "SELECT count(*) FROM pg_stat_activity;"

# Redis memory
redis-cli info memory
```

## Known Limitations

1. **OAuth Login**: Temporarily disabled, needs proto regeneration
2. **Machine Learning**: Not implemented (face detection, smart search)
3. **Live Photos**: Basic support only
4. **Video Transcoding**: Not implemented
5. **Map Features**: Not implemented

## Reporting Issues

If you encounter issues during testing:
1. Check the TODO.md file for known issues
2. Review server logs for error messages
3. Verify your configuration matches the requirements
4. Test with minimal configuration first

## Next Steps

Once basic testing is successful:
1. Configure storage backends (S3, Rclone)
2. Set up SSL/TLS for production
3. Configure backup strategies
4. Optimize database indexes
5. Set up monitoring and alerting