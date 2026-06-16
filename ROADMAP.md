# Immich Go Backend Roadmap

## Upstream Compatibility Check (2026-06-16)

**Current stable Immich baseline:** v2.7.5 (released 2026-04-13)
**Latest upstream preview:** v3.0.0-rc.0 (released 2026-06-15)
**Sources:** [Immich releases](https://github.com/immich-app/immich/releases), [Immich OpenAPI spec](https://github.com/immich-app/immich/blob/main/open-api/immich-openapi-specs.json), [OAuth docs](https://docs.immich.app/administration/oauth)

The active roadmap is API parity and behavior hardening against upstream Immich, using v2.7.5 as the stable target and v3.0.0-rc.0 as forward-looking input.

### Latest Immich Changes To Track

- [ ] v2.7.x shared link and auth fixes: review shared-link asset removal permissions and version-check rate limiting/deduplication.
- [ ] v2.7.x media fixes: verify original filename hiding when metadata is disabled and people search behavior for short queries.
- [ ] v3 RC new capabilities: workflows/plugins parity, HLS real-time transcoding, integrity report jobs, recently added assets, OAuth backchannel logout, full-path search, album map markers, and user upload heatmap.
- [ ] v3 RC database/runtime changes: assess pgvecto.rs removal implications and duration-in-milliseconds response changes.

## Active Backlog

- [x] Implement rate limiting for login attempts.
- [x] Implement profile image upload and management.
- [ ] Implement user license management.
- [ ] Add streaming support for large gRPC operations.
- [ ] Implement configurable worker pools for background jobs.
- [ ] Add advanced retry logic for background jobs.
- [ ] Review shared-link asset removal permissions.
- [ ] Add version-check rate limiting and deduplication.
- [ ] Verify original filename hiding when metadata is disabled.
- [ ] Verify people search behavior for short queries.
- [ ] Implement workflows/plugins parity.
- [ ] Implement HLS real-time transcoding.
- [ ] Implement integrity report jobs.
- [ ] Finish recently added assets endpoint behavior.
- [ ] Implement OAuth backchannel logout.
- [ ] Implement full-path search.
- [ ] Implement album map markers.
- [ ] Implement user upload heatmap.
- [ ] Assess pgvecto.rs removal implications.
- [ ] Assess duration-in-milliseconds response changes.

## Future Enhancements

### Performance & Reliability
- [ ] Load testing.
- [ ] Storage performance tests.
- [ ] Database performance tests.
- [ ] Memory usage optimization.
- [ ] Configurable worker pools for background jobs.
- [ ] Advanced retry logic for background jobs.

### Machine Learning Integration (optional)
- [ ] Face recognition.
- [ ] Object detection.
- [ ] CLIP search.
- [ ] ML-based duplicate detection.

### Video Processing
- [ ] Video transcoding (handler ready, needs ffmpeg integration).
- [ ] Video thumbnail generation.
- [ ] Video metadata extraction.

### Monitoring & Operations
- [ ] Grafana dashboards.
- [ ] Alerting rules.
- [ ] Kubernetes deployment with Helm charts.

## Technical Decisions Made

- **Storage**: Rclone-based abstraction for maximum flexibility.
- **Database**: SQLC for type-safe SQL queries.
- **Observability**: OpenTelemetry with autoexport for vendor-neutral monitoring.
- **Configuration**: YAML + environment variables for 12-factor app compliance.
- **Build System**: Nix for reproducible builds.
- **Architecture**: Clean architecture with clear separation of concerns.
- **API**: gRPC with grpc-gateway for REST compatibility.
- **Testing**: Integration tests with Docker-based PostgreSQL.
