# immich-go-backend Helm Chart

Production-oriented Helm chart for the [immich-go-backend](https://github.com/denysvitali/immich-go-backend) Go server.

| Port | Protocol | Purpose |
|------|----------|---------|
| 3001 | HTTP / WebSocket | REST API (grpc-gateway), metrics, optional static UI |
| 3002 | gRPC | Internal gRPC services |

PostgreSQL and Redis are **external** — this chart only deploys the backend.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.8+
- External **PostgreSQL 16** (with `uuid-ossp`, `vector`, `earthdistance` extensions as required by the schema)
- External **Redis 7** (asynq job queue)

## Install

```bash
# Minimal values (secrets via --set or a values file; never commit secrets)
helm install immich ./deploy/helm/immich-go-backend \
  --namespace immich --create-namespace \
  --set database.url='postgres://immich:SECRET@postgres:5432/immich?sslmode=require' \
  --set auth.jwtSecret="$(openssl rand -hex 32)" \
  --set redis.url='redis://redis:6379/0'
```

Using an existing Secret:

```bash
kubectl -n immich create secret generic immich-secrets \
  --from-literal=DATABASE_URL='postgres://...' \
  --from-literal=AUTH_JWT_SECRET="$(openssl rand -hex 32)" \
  --from-literal=JOBS_REDIS_URL='redis://redis:6379/0'

helm install immich ./deploy/helm/immich-go-backend \
  --namespace immich \
  --set existingSecret=immich-secrets \
  --set database.url='' \
  --set auth.jwtSecret=''
```

## Upgrade

```bash
helm upgrade immich ./deploy/helm/immich-go-backend -n immich -f my-values.yaml
```

Migrations run on `serve` when `IMMICH_DATABASE_AUTO_MIGRATE=true` (default via `database.autoMigrate`).

## Configuration

See [`values.yaml`](values.yaml) for the full list. Important keys:

| Key | Description | Default |
|-----|-------------|---------|
| `replicaCount` | Pod replicas (use 1 with local PVC RWO) | `1` |
| `image.repository` / `image.tag` | Container image | `ghcr.io/denysvitali/immich-go-backend` / chart `appVersion` |
| `service.http.port` / `service.grpc.port` | Service ports | `3001` / `3002` |
| `database.url` | PostgreSQL DSN → `DATABASE_URL` | `""` |
| `database.autoMigrate` | Run migrations on start | `"true"` |
| `auth.jwtSecret` | JWT signing secret → `AUTH_JWT_SECRET` | `""` |
| `redis.url` | Redis URL → `JOBS_REDIS_URL` | `redis://redis:6379/0` |
| `storage.backend` | `local`, `s3`, or `rclone` | `local` |
| `storage.localRoot` | Upload root on volume | `/data/uploads` |
| `persistence.enabled` / `persistence.size` | PVC for local media | `true` / `50Gi` |
| `ingress.enabled` | Expose REST via Ingress | `false` |
| `autoscaling.enabled` | HPA | `false` |
| `existingSecret` | Use external Secret for credentials | `""` |

Non-secret settings go into a ConfigMap; credentials into a chart-managed Secret (or `existingSecret`).

Environment variable names match the binary (`internal/config/config.go`): unprefixed path-style names such as `DATABASE_URL`, `SERVER_ADDRESS`, plus a few `IMMICH_*` keys (`IMMICH_DATABASE_AUTO_MIGRATE`, `IMMICH_WEBUI_DIR`).

## Probes

Probes hit the real Immich ping endpoint used in production (`fly.toml`, `DEPLOYMENT.md`):

```
GET /api/server/ping
```

Startup / liveness / readiness probes are enabled by default on the `http` port.

## Storage

### Local (`storage.backend=local`)

- PVC mounted at `persistence.mountPath` (default `/data`)
- `STORAGE_LOCAL_ROOT=/data/uploads`, `UPLOAD_TEMP_DIR=/data/tmp`
- Deployment strategy is `Recreate` to avoid multi-attach on RWO volumes
- For multi-replica, use `ReadWriteMany` storage or switch to S3

### S3

```yaml
storage:
  backend: s3
  s3:
    enabled: true
    bucket: my-immich-bucket
    region: us-east-1
    accessKeyId: "..."
    secretAccessKey: "..."
persistence:
  enabled: false
```

Or put S3 credentials in `existingSecret`.

## Ingress

```yaml
ingress:
  enabled: true
  className: nginx
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "0"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
  hosts:
    - host: photos.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: immich-tls
      hosts:
        - photos.example.com
```

gRPC stays on the ClusterIP Service (`:3002`); Ingress only fronts HTTP.

## Security

- Runs as non-root **uid/gid 1001** (matches Dockerfile `appuser`)
- Drops all capabilities; `seccompProfile: RuntimeDefault`
- Prefer `existingSecret` over putting `auth.jwtSecret` / `database.url` in values files checked into git

## Uninstall

```bash
helm uninstall immich -n immich
# PVCs are retained by default — delete manually if desired:
kubectl -n immich delete pvc -l app.kubernetes.io/instance=immich
```

## Template check

```bash
helm template test ./deploy/helm/immich-go-backend \
  --set database.url='postgres://immich:immich@postgres:5432/immich?sslmode=disable' \
  --set auth.jwtSecret='test-secret-at-least-32-bytes-long!!'
```
