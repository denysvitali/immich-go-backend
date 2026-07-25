# Findings — bugs found during the complexity audit, deliberately not fixed

Found while auditing for [docs/refactor-audit.md](docs/refactor-audit.md). Per the refactor
rules, behaviour changes are never bundled with refactoring: each item below needs its own
`fix:` commit. Nothing here has been changed.

Measured on `master` @ `0755de4`.

---

## 1. `panic: close of closed channel` crashes the server on WebSocket disconnect — H

`internal/websocket/websocket.go`

`c.done` is closed in three places:

- `readPump`'s `defer` (line 127)
- `handleMessage`, on an Engine.IO `PacketClose` (line 215)
- `handleSocketIOMessage`, on a Socket.IO `PacketDisconnect` (line 258)

`handleMessage` and `handleSocketIOMessage` both run **on the `readPump` goroutine**, so
after either closes `done`, `readPump` keeps looping until its next read error and then
runs `defer close(c.done)` on an already-closed channel.

An unrecovered panic in that goroutine takes the whole process down — the HTTP server
included.

**Reproduction:** open `/api/socket.io/`, send a Socket.IO DISCONNECT packet (`41`), then
close the TCP connection. `readPump` returns and re-closes `done`.

**Fix shape:** one owner closes `done` exactly once (`sync.OnceFunc`, or a `closeOnce`
method); the packet handlers request close rather than performing it.

---

## 2. 56 documented environment variables are silently ignored — H

`internal/config/config.go`

The config structs carry 72 `env:"…"` tags, but nothing reads them: there is no
reflection, no `envconfig`-style library, only the hand-written `os.Getenv` block at
`config.go:497`, which honours 19 names (16 of which match a tag).

Ignored despite being tagged **and documented in `README.md` / `ROADMAP.md`**:

```
AUTH_EMAIL_VERIFICATION_REQUIRED  AUTH_JWT_EXPIRY  AUTH_JWT_ISSUER
AUTH_JWT_REFRESH_EXPIRY  AUTH_PASSWORD_MIN_LENGTH  AUTH_PASSWORD_REQUIRE_LOWERCASE
AUTH_PASSWORD_REQUIRE_NUMBERS  AUTH_PASSWORD_REQUIRE_SYMBOLS
AUTH_PASSWORD_REQUIRE_UPPERCASE  AUTH_REGISTRATION_ENABLED  AUTH_SESSION_TIMEOUT
DATABASE_CONNECT_TIMEOUT  DATABASE_LOG_QUERIES  DATABASE_MAX_CONN_LIFETIME
DATABASE_MAX_IDLE_CONNS  DATABASE_MAX_OPEN_CONNS  DATABASE_QUERY_TIMEOUT
FEATURE_BACKUP_SYNC_ENABLED  FEATURE_EXIF_EXTRACTION_ENABLED
FEATURE_OBJECT_DETECTION_ENABLED  FEATURE_SHARING_ENABLED
FEATURE_THUMBNAIL_GENERATION_ENABLED  FEATURE_VIDEO_TRANSCODING_ENABLED
JOBS_CLEANUP_ENABLED  JOBS_CLEANUP_INTERVAL  JOBS_ENABLED  JOBS_JOB_TIMEOUT
JOBS_QUEUES  JOBS_REDIS_URL  JOBS_RETENTION_PERIOD  JOBS_WORKERS
LOG_COMPRESS  LOG_FILE_PATH  LOG_FORMAT  LOG_LEVEL  LOG_MAX_AGE  LOG_MAX_BACKUPS
LOG_MAX_SIZE  LOG_OUTPUT  LOG_ROTATION_ENABLED  OAUTH_ENABLED
SERVER_CORS_ALLOWED_ORIGINS  SERVER_CORS_ENABLED  SERVER_HEALTH_CHECK_ENABLED
SERVER_HEALTH_CHECK_PATH  SERVER_IDLE_TIMEOUT  SERVER_METRICS_ENABLED
SERVER_METRICS_PATH  SERVER_READ_TIMEOUT  SERVER_REQUEST_LOGGING
SERVER_SHUTDOWN_TIMEOUT  SERVER_WRITE_TIMEOUT
TLS_AUTO_GENERATE  TLS_CERT_FILE  TLS_ENABLED  TLS_KEY_FILE
```

Operationally this is worst for `JOBS_REDIS_URL`, `JOBS_ENABLED`, `LOG_LEVEL`, the `TLS_*`
set, and every `FEATURE_*` flag: a deployment sets them, the server ignores them, and
nothing warns.

**Reproduction:** `LOG_LEVEL=debug ./bin/immich-go-backend serve` → level stays at the
YAML/default value.

**Fix shape:** honour the tags with one small reflective loader (single source of truth,
also retires the parallel `default:` tags and `setDefaults`), or extend `loadFromEnv`
explicitly. Either way it is a behaviour change — deployments that currently set these
variables will start taking effect.

The audit's Phase B instead *deletes* the unhonoured tags, which is behaviour-preserving
and stops the code from lying. That is a stopgap, not this fix.

---

## 3. `cfg.Server.*Timeout` is ignored; HTTP timeouts are hardcoded — H

`cmd/main.go:190-195`

```go
httpServer := &http.Server{
    Addr:         httpAddr,
    Handler:      srv.HTTPHandler(),
    ReadTimeout:  30 * time.Second,
    WriteTimeout: 30 * time.Second,
}
```

`cfg.Server.ReadTimeout`, `WriteTimeout`, `IdleTimeout`, and `ShutdownTimeout` all exist,
are documented, and are never read. `IdleTimeout` is left at the Go default.

A 30s `WriteTimeout` also caps large asset downloads and HLS segment delivery on slow
links regardless of configuration.

**Reproduction:** set `server.read_timeout: 5m` in `config.yaml`, upload a large file over
a slow connection, observe the 30s cut-off.

**Fix shape:** read the four values from `cfg`; keep the current numbers as defaults so
nothing changes for a config that doesn't set them.

---

## 4. `internal/stacks/service_test.go` does not compile under `-tags=integration` — H

```
$ go vet -tags=integration ./...
vet: internal/stacks/service_test.go:48:3: not enough arguments in call to service.CreateStack
	have (context.Context, CreateStackRequest)
	want (context.Context, string, CreateStackRequest)
```

`CreateStack` gained a `userID string` parameter (`internal/stacks/service.go:112`) and the
integration test was not updated. Because the file is behind `//go:build integration`,
`go build ./...` and the `-short` unit job never see it — so this has been broken on
`master` without turning the `build` job red.

The CI `integration-tests` job (`go test -tags=integration -race ./internal/...`) cannot be
green while this stands.

**Fix shape:** pass the owner UUID the test already creates. Test-only, one line.

---

## 5. Gateway handler registration failures are logged, not fatal — M

`internal/server/server.go:537-645`

All 34 `RegisterXServiceHandlerServer` calls follow:

```go
if err := immichv1.RegisterAuthServiceHandlerServer(ctx, mux, s); err != nil {
    logrus.WithError(err).Error("Failed to register AuthService handler")
}
```

A failure means an entire service's REST surface is absent, which is exactly the class of
regression `OPENAPI_COVERAGE_FAIL_UNDER: 100` exists to prevent — but the coverage tool
reads the AST, not the running mux, so it cannot see it. The server starts happily and
returns 404 for every route in that service.

**Fix shape:** collect with `errors.Join`, return the error from `HTTPHandler`, fail
startup. Behaviour change: a configuration that currently limps along would now refuse to
boot. That is the intent.

---

## 6. Unbounded blocking send on `Client.send` — M

`internal/websocket/websocket.go:221`

```go
case engine.PacketPing:
    pongPacket := engine.EncodePacket(engine.PacketPong, nil)
    c.send <- pongPacket        // no select, no default
```

The three other sends on this channel (`pingPump:180`, `handleSocketIOConnect:290`,
`handleSocketIOEvent:328`) all use `select` with a `default` and drop on a full buffer.
This one blocks forever once the 256-slot buffer fills and `writePump` has exited, leaking
the `readPump` goroutine and its connection for the process lifetime.

**Reproduction:** a client that sends Engine.IO pings while never reading, until the buffer
fills.

**Fix shape:** same non-blocking send + drop policy as the neighbouring sites.

---

## 7. WebSocket `CheckOrigin` accepts every origin — M

`internal/websocket/websocket.go:34`

```go
CheckOrigin: func(r *http.Request) bool {
    return true // Consider implementing proper origin checking in production
},
```

Any web page can open an authenticated socket.io connection against a logged-in user's
browser session (the handshake accepts the `immich_access_token` cookie via the standard
request path). `cfg.Server.CORS` exists but is unused here — and, per finding 2, is not
configurable by env var either.

**Fix shape:** validate `Origin` against the configured CORS allow-list, with same-origin
always permitted. Behaviour change for any deployment currently relying on cross-origin
sockets, so it needs a config escape hatch.
