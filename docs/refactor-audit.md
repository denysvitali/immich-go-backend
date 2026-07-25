# Refactor audit — immich-go-backend

Phase A deliverable. **No code changed.** Measured on `master` @ `0755de4`, Go 1.26.0,
golangci-lint v2 config as committed.

Scope of measurement: all Go outside `internal/proto/gen/` and `internal/db/sqlc/`
(both generated, both excluded from every count below).

---

## 1. Metrics baseline

| Metric | Value |
|---|---|
| Go files (non-generated) | 238 |
| LOC (non-generated) | 60,422 |
| Files > 300 lines | 74 |
| Files > 500 lines (must-fix threshold) | **28** |
| Longest file | `internal/assets/service.go` — 1,216 lines |
| Functions > 60 lines (must-fix) | **120** |
| Longest function | `server.NewServer` — 271 lines |
| `gocyclo` > 12 (must-fix) | 59 |
| `gocognit` > 10 | 123 |
| `gocognit` > 15 (must-fix) | 64 |
| Highest cognitive complexity | `applyProtoToSystemConfig` — 99 |
| `dupl -t 100` clone groups | 25 |
| `sync.Mutex` / `sync.RWMutex` declarations | 11 |
| `sync.WaitGroup` | 0 |
| bare `go` statements (non-test) | 10 |
| `context.Background()` below `main` (non-test) | 9 |
| Packages > 20 files | 1 — `internal/server` (**68 files**, 37 non-test) |
| Structs > 12 fields | 28 (most are upstream DTOs; see §2 note) |
| `golangci-lint run` (committed config) | **0 issues** |
| `go vet ./...` | clean |
| `gofumpt -l` | 14 files unformatted (hidden by lint exclusion presets) |
| `go test -short ./...` | **pass** |
| `go test -short -race ./...` | **pass** |
| `go vet -tags=integration ./...` | **FAILS to compile** — `internal/stacks/service_test.go:48` |
| Coverage, `-short` | **5.2%** of statements |
| Coverage, integration | not measurable here — no Docker in this environment |
| Env-var struct tags that are actually honoured | **16 of 72** |

Top 12 cognitive-complexity offenders (non-generated):

| Cognit | Location |
|---|---|
| 99 | `internal/server/systemconfig.go:226` `applyProtoToSystemConfig` |
| 54 | `internal/assets/metadata.go:227` `extractVideoMetadata` |
| 52 | `internal/assets/metadata.go:394` `ExtractVideoMetadataFromFile` |
| 49 | `internal/assets/metadata.go:88` `extractImageMetadata` |
| 43 | `internal/auth/service.go:761` `validatePassword` |
| 39 | `internal/config/config.go:494` `loadFromEnv` |
| 38 | `internal/server/server.go:519` `HTTPHandler` |
| 35 | `internal/assets/service.go:1061` `convertToAssetInfo` |
| 33 | `internal/activity/server.go:35` `GetActivities` |
| 32 | `internal/server/http_frontend.go:30` `handleFrontendShape` |
| 32 | `internal/ffmpeg/ffmpeg.go:87` `ProbeVideo` |
| 30 | `internal/timeline/service.go:98` `GetBucketAssets` |

---

## 2. Findings

Severity: **H** = actively costs correctness/availability or actively misleads;
**M** = real complexity or footgun, contained; **L** = tidy-up.

Note on wide structs: `systemconfig.Dto`, `assets.AssetMetadata`, `timeline.BucketAsset`,
`users.UserPreferences` etc. are flat upstream-API data carriers. Splitting them would
*increase* mental load and risk parity drift. They are recorded here as **justified
exceptions**, not findings.

### 2.1 Concurrency (highest value)

| Location | Issue | Cat | Sev | Action | Risk | Size |
|---|---|---|---|---|---|---|
| `internal/websocket/websocket.go:127,215,258` | `close(c.done)` in three places on the same channel; `readPump`'s `defer` runs after a `PacketClose`/`PacketDisconnect` already closed it → **`panic: close of closed channel`, crashes the process** | correctness | **H** | Single owner closes `done` exactly once (`sync.OnceFunc` or a `closeOnce` method); other sites request close | med — needs a characterization test first | M |
| `internal/websocket/websocket.go:221` | `c.send <- pongPacket` with no `select`/`default` — blocks forever once the 256 buffer fills and `writePump` has exited → goroutine + conn leak | concurrency | **H** | Non-blocking send with the same drop policy the other two send sites already use | low | S |
| `internal/server/http_hls.go:44,233` | package-global `hlsGenerationMu` held across `downloadAssetVideoToTemp` + `ffmpeg` transcode. **Every HLS request server-wide serialises behind one lock**, and waiters ignore `ctx` cancellation | concurrency | **H** | Per-session keyed in-flight map (`map[string]chan struct{}` owned by one goroutine, or `golang.org/x/sync/singleflight`) + ctx-aware wait | med | M | 
| `internal/websocket/websocket.go:24-67` | `Hub.mu` guards `clients`, but `clients` is only ever touched inside `Run` → the lock is dead weight. `clients` itself is **write-only**: nothing broadcasts, so the whole map is unreachable state | concurrency / dead code | **M** | Delete `mu` (state is already goroutine-owned); document the contract. Keep `clients` only if a broadcast path is coming — otherwise delete and say so | low | S |
| `internal/server/server.go:171` | `go wsHub.Run()` in the constructor, no shutdown handle, `Run` has no exit path. `Stop()` does not wait for it | concurrency | **M** | `New(ctx)`-style construction returning a `Close()`; `Run` selects on `ctx.Done()` | low | S |
| `internal/assets/service.go:239,254,891` + `internal/libraries/service.go:286` | fire-and-forget `go s.processAsset(...)`: unbounded goroutine count on upload bursts, no owner, no wait at shutdown, `:254` uses `context.Background()` | concurrency | **M** | Bounded worker pool owned by `Service` with a `Close()`; or enqueue on asynq (already present) instead of a raw goroutine | med | M |
| `internal/sync/service.go:19-30` | three `RWMutex`-guarded maps (`syncAcks`, `lastSync`, `eventSubscribers`) keyed by userID, **never evicted** → unbounded memory; comment admits it should be Redis/DB | concurrency / footgun | **M** | Single owner goroutine for all three (they're one logical state), plus a TTL eviction policy stated in code | med | M |
| `internal/server/maintenance.go:139` | mutable package-global `versionCheckCache` with an **embedded** `sync.Mutex`, plus overridable global `versionCheckHTTPClient` | footgun | **M** | Move both onto `Server` as an unexported field; inject the client through the constructor | low | S |
| `internal/auth/rate_limiter.go:16` | `attempts` map only evicted on access → one entry per distinct login email, forever | footgun | **M** | Periodic sweep or a bounded LRU; document the bound | low | S |
| `internal/queue/service.go:103`, `internal/maintenance/service.go:41`, `internal/plugin/service.go:119` | three more `RWMutex`-guarded in-memory registries whose comments say *"in production, this would be backed by Redis/BullMQ / the database"*. `queue` shadows the real asynq queue in `internal/jobs/` | structure / Rule 1 | **M** | Decide per package: persist via SQLC (the project rule) or delete the shadow. Needs a product call on `queue` vs `jobs` | high | L |
| all 11 locks | none documents which fields it guards or the lock ordering | docs | **M** | Add the §7.2 contract comment to each survivor | low | S |

### 2.2 Complexity

| Location | Issue | Cat | Sev | Action | Risk | Size |
|---|---|---|---|---|---|---|
| `internal/server/server.go:166` | `NewServer` — 271 lines, 23 cognit, 45 constructor calls, mixes wiring + Redis URL parsing + job-worker startup + a **DB write** (`recordVersionHistory`) | complexity | **H** | Split into `newServices()` / `newGRPCServer()` / `registerGRPC()`; move `recordVersionHistory` to an explicit `Start(ctx)` — I/O does not belong in a constructor | med | L |
| `internal/server/server.go:75-164` | `Server` — **51 fields** + 35 embedded `Unimplemented*`. God object: one type implements ~15 gRPC services and holds 25 services it delegates to | structure | **H** | Group fields into a `services` struct; longer term move the per-domain RPC methods next to their domain servers (most domains already have one) | high | XL |
| `internal/server/server.go:537-645` | 34× copy-pasted `if err := Register…HandlerServer(...); err != nil { log }`. **Registration failure is logged, never fatal** — a missing route silently breaks parity that CI is supposed to guarantee | duplication / footgun | **H** | Table of `{name, register func}` + `errors.Join`; return the error from `HTTPHandler` so startup fails loudly | low | M |
| `internal/server/systemconfig.go:226` | `applyProtoToSystemConfig` — 188 lines, cognit 99, the worst function in the repo | complexity | **H** | One `applyX(cfg, dto)` per config section, driven by a table; each section independently testable | med | L |
| `internal/config/config.go:367,494` | `setDefaults` (108 lines) and `loadFromEnv` (cognit 39) are two hand-maintained parallel lists that must agree with each other **and** with 72 struct tags | duplication / footgun | **H** | See §2.4 — collapse to one source | med | M |
| `internal/assets/metadata.go:88,227,394` | three extractors, 49/54/52 cognit, 113/139/113 lines; `:288` and `:435` are a `dupl` clone pair | complexity / duplication | **M** | Separate decide-from-effect: pure `mapExifToMetadata(tags)` + thin I/O wrapper; table-driven tests replace the branches | med | L |
| `cmd/main.go:91` | `runServer` — 129 lines, cognit 30, five lifecycle concerns interleaved | complexity | **M** | `startEmbeddedPG` / `openDB` / `startGRPC` / `startHTTP` / `waitForShutdown`; keeps `main.go` under the 100-line guideline | low | M |
| `internal/server/http_frontend.go:30` | `handleFrontendShape` — 147 lines, cyclo 41 | complexity | **M** | Split per HTTP method into `getFrontendShape` / `postFrontendShape` / … **keeping every route string a literal** (see §5) | low | S |
| `internal/assets/service.go` (1,216), `internal/jobs/handlers.go` (1,094), `internal/server/asset.go` (1,019), + 25 more | over the 500-line must-fix line | structure | **M** | Split by capability (`service_create.go`, `service_query.go`, …); `git mv`-only commits | low | L |
| 120 functions > 60 lines | — | complexity | **M** | Address the top 20 in this pass; enable `funlen` afterwards so the rest can't grow | — | L |

### 2.3 Dead code

| Location | Issue | Cat | Sev | Action | Risk | Size |
|---|---|---|---|---|---|---|
| `internal/auth/middleware.go:53-309` | `AuthMiddleware`, `AdminMiddleware`, `OptionalAuthMiddleware`, `GetUserFromContext`, `GetClaimsFromContext` — **zero references anywhere**. ~250 lines of gin handlers in a project whose HTTP layer is grpc-gateway | dead code | **H** | Delete. `github.com/gin-gonic/gin` then leaves `go.mod` as a direct dependency | low | S |
| `internal/websocket/websocket.go:25` | `Hub.clients` — written and deleted, never read | dead code | **M** | Delete with the lock (see §2.1) | low | S |
| `internal/server/server.go:907` | `func (s *Server) Shutdown(ctx) error { return nil }` — never called, returns nil unconditionally. A reader reasonably assumes shutdown is wired | footgun | **M** | Delete, or implement and call it from `cmd/main.go` | low | S |
| `internal/server/server.go` fields | 11 fields assigned in `NewServer` and never read: `adminService`, `albumService`, `apiKeyService`, `duplicatesService`, `facesService`, `libraryService`, `memoriesService`, `notificationsService`, `searchService`, `stacksService`, `syncService`, `viewService` | dead code | **M** | Delete the fields (keep the `*Server` variants, which *are* used) | low | S |
| `internal/server/errors.go` | entire file is two `var X = grpcutil.X` aliases carrying a **verbatim copy** of `grpcutil`'s 20-line doc comment | duplication / docs | **L** | Delete the file; call `grpcutil.SanitizedInternal` directly (mechanical rename across `internal/server`) | low | M |

### 2.4 Footguns

| Location | Issue | Cat | Sev | Action | Risk | Size |
|---|---|---|---|---|---|---|
| `internal/config/config.go` | **56 of 72 `env:"…"` struct tags are never read** — no reflection, no envconfig library, only a hand-written `os.Getenv` block. `JOBS_WORKERS`, every `FEATURE_*`, all `LOG_*`, all `TLS_*`, all `SERVER_*` timeouts look configurable and are silently ignored. README and ROADMAP both document them as working | footgun | **H** | Behaviour-preserving now: delete the unhonoured tags and point the struct doc at `loadFromEnv`. **Wiring them up is a behaviour change** → separate `fix:` commit, listed in `FINDINGS.md` | low (delete) / med (wire) | M |
| `cmd/main.go:190-195` | `http.Server` hardcodes `ReadTimeout`/`WriteTimeout: 30s` and ignores `cfg.Server.ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `ShutdownTimeout` — config knobs that exist, are documented, and do nothing | footgun | **H** | Read them from `cfg`. Behaviour change (defaults differ) → `fix:` commit | low | S |
| `internal/config/config.go` | `default:"…"` tags duplicate `setDefaults()`; the tags are likewise unread | duplication | **M** | Same treatment as `env:` | low | S |
| `cmd/main.go:199` | `err != http.ErrServerClosed` — direct comparison instead of `errors.Is` | footgun | **L** | `errors.Is`; enable `errorlint` | low | S |
| `internal/websocket/websocket.go:34` | `CheckOrigin: return true` — accepts every WebSocket origin, with a comment deferring the fix | security footgun | **M** | Origin allow-list from `cfg.Server.CORS`. Behaviour change → `fix:` commit + `FINDINGS.md` | med | S |
| `internal/assets/service.go:254` | `context.Background()` in `TriggerProcessing` — no cancellation, no trace linkage | footgun | **M** | Take `ctx` and use `context.WithoutCancel` as `:239` already does | low | S |
| 14 files | not `gofumpt`-clean, but `gofumpt` is in the `formatters` block and the exclusion presets swallow it | ci | **L** | Format, then verify the CI gate actually fails on drift | low | S |

### 2.5 Duplication (classified per §6)

| Location | Kind | Action |
|---|---|---|
| `duplicates`, `faces`, `stacks`, `users`, `workflow` `NewService` — 5 clones × ~35 lines of `meter.Int64Counter`/`Float64Histogram`/`Int64UpDownCounter` + `if err != nil` | **structural boilerplate** | One `telemetry.NewServiceMetrics(prefix)` helper returning a small struct. Removes ~150 lines and lets several constructors drop their `error` return (**BREAKING** — flag) |
| `internal/server/server.go:537-645` — 34 registration blocks | **structural boilerplate** | Table + `errors.Join` (§2.2) |
| `internal/trash/service.go:57,122,237` (3 clones), `internal/albums/service.go:253,302`, `internal/memories/service.go:299,339`, `internal/tags/server.go:328,372`, `internal/trash/server.go:31,63`, `internal/sessions/server.go:72,158`, `internal/stacks/service.go:462,521`, `internal/libraries/service.go:125,230`, `internal/storage/rclone.go:288,309`, `internal/admin/service.go:484,652`, `internal/server/sharedlinks.go:252,281`, `internal/server/auth.go:195,231`, `internal/memories/server.go:306,333`, `internal/server/album.go:188,223`, `internal/server/plugin.go:16`/`workflow.go:28` | mixed — needs a read each | Triage individually. Expectation: the *same-file* pairs (trash, albums, memories, tags, sessions) are true duplication of one rule; the *cross-domain* pairs (`album.go`/`asset.go`, `plugin.go`/`workflow.go`) are coincidental and get a one-line "independent" comment |
| `internal/server/errors.go` doc comment vs `internal/grpcutil/errors.go` | **true duplication** (docs) | Delete the shim (§2.3) |
| `internal/trash/service_test.go:159,324`, `internal/notifications/service_test.go:262,348`, `internal/server/workflow_test.go:32,60`, `internal/jobs/handlers_integration_test.go:288,313`, `internal/people/server_test.go` vs `internal/tags/server_test.go` | test duplication | Collapse into table-driven subtests |

### 2.6 Structure

| Issue | Sev | Action |
|---|---|---|
| `internal/server/` holds **68 files** (37 non-test): the gRPC god object, 20 hand-written REST handlers, socket.io codec, conversion helpers, error shims. It is the one package a newcomer cannot navigate | **H** | Split — but see the hard constraint in §5 before moving any `http_*.go` file |
| `internal/server/socketio/` (+ `socketio/engine/`) is imported by `internal/websocket`, *not* by `internal/server`; meanwhile `internal/server` imports `internal/websocket`. A pure protocol codec is nested under an unrelated transport package | **M** | `git mv internal/server/socketio internal/socketio` |
| `internal/util/` (`Ptr`, `Offset`, `OptionalBool/Text/Int8/UUID`) — a forbidden dumping-ground name; the `Optional*` helpers are pgtype adapters that belong beside `internal/db/pgutil/` | **M** | `Optional*` → `internal/db/pgutil`; `Offset` → the package that paginates; `Ptr` → `internal/ptr` or drop for a local generic |
| `internal/server/utils.go` — forbidden name; holds 4 auth-context accessors | **L** | `git mv` → `internal/server/authcontext.go` |
| `internal/db/pgutil/` — forbidden `util` suffix, but the contents are cohesive (pgtype ↔ Go conversions) | **L** | Rename to `internal/db/pgtypeconv` (or accept and document; low value, some churn) |
| No `doc.go` anywhere; 20+ packages exceed 3 files | **M** | Add package docs stating what belongs and the concurrency model |
| No `depguard`/architecture rule in CI. Today only `internal/auth` violates domain↔transport separation (via the dead gin file) — so the boundary is nearly free to lock in | **M** | Add `depguard` once §2.3 lands |

### 2.7 Bugs found — recorded, not fixed

Written up with reproductions in [`FINDINGS.md`](../FINDINGS.md):

1. **H** double-`close(c.done)` panic in the WebSocket client (`internal/websocket/websocket.go`).
2. **H** 56 documented env vars silently ignored (`internal/config/config.go`).
3. **H** `cfg.Server.*Timeout` ignored; hardcoded 30s (`cmd/main.go:190`).
4. **H** `internal/stacks/service_test.go:48` does not compile under `-tags=integration` → the CI `integration-tests` job cannot be green on `master`.
5. **M** gateway handler registration failures are logged, not fatal (`internal/server/server.go:537`).
6. **M** unbounded blocking send on `c.send` (`internal/websocket/websocket.go:221`).
7. **M** `CheckOrigin` accepts all origins (`internal/websocket/websocket.go:34`).

---

## 3. Proposed target tree (path diff)

Deliberately conservative: this is not a layered-architecture rewrite. Every move below is
a `git mv` with no content edit, except where noted.

```
  cmd/
-   main.go                                 # 320 lines, runServer 129 of them
+   main.go                                 # flags + wiring only, <100 lines
+   serve.go                                # runServer, split into lifecycle stages
+   migrate.go                              # already separate? verify during Phase B

  internal/
-   util/optional.go                        → internal/db/pgutil/optional.go
-   util/pagination.go                      → internal/pagination/offset.go
-   util/ptr.go                             → internal/ptr/ptr.go
-   util/                                   (package removed)

-   server/socketio/                        → internal/socketio/
-   server/socketio/engine/                 → internal/socketio/engine/

-   server/utils.go                         → internal/server/authcontext.go
-   server/errors.go                        (deleted — alias shim)

-   auth/middleware.go                      (deleted — dead gin code)
+   auth/context.go                         # the 6 live context helpers kept from it

    server/
-     server.go                             # 956 lines
+     server.go                             # Server type + Start/Stop only
+     services.go                           # newServices() — the 45 constructor calls
+     grpc_register.go                       # the registration table
+     gateway.go                            # mux + marshaler + response modifier
+     logging.go                            # httpLoggingHandler + responseStatusRecorder
+     doc.go
+   <one doc.go per package over 3 files>

  docs/
+   refactor-audit.md                       # this file
+   refactor-report.md                      # Phase C
+   adr/0001-owner-goroutine-for-websocket-hub.md
+   adr/0002-hls-generation-keyed-singleflight.md
+   adr/0003-config-env-single-source.md
  FINDINGS.md                               # bugs found, not fixed
```

Explicitly **not** proposed: moving `internal/server/http_*.go` into subpackages, and
splitting the `Server` god object across packages. Both are blocked on §5.

---

## 4. Concurrency inventory — every lock, every goroutine

| # | Site | Disposition |
|---|---|---|
| 1 | `websocket.Hub.mu` (RWMutex) | **Delete** — state is already owned by `Run` |
| 2 | `sync.Service.syncAckMutex` | **Convert** to owner goroutine (with 3 and 4 — one logical state) |
| 3 | `sync.Service.lastSyncMutex` | **Convert** (see 2) |
| 4 | `sync.Service.eventMutex` | **Convert** (see 2) |
| 5 | `server.hlsGenerationMu` (global) | **Replace** with keyed in-flight map / singleflight; the current lock spans minutes of I/O |
| 6 | `server.versionCheckCache` (embedded Mutex, global) | **Keep the lock, move the state** onto `Server`; critical section is already short and I/O-free — document fields guarded |
| 7 | `auth.loginRateLimiter.mu` | **Keep** — short, no I/O, per-process, hot-ish. Add the guarded-fields comment + an eviction policy |
| 8 | `maintenance.Service.mu` | **Keep** short-term, document. Real fix is persistence (product call) |
| 9 | `queue.Service.mu` | **Blocked** — depends on the `queue` vs `jobs` decision |
| 10 | `plugin.Service.mu` | **Blocked** — same class as 9 |
| 11 | `oauth` `jwksCacheMu` | **Keep** if the fetch happens outside the lock — verify during Phase B; document either way |

| # | `go` statement | Owner today | Disposition |
|---|---|---|---|
| 1 | `cmd/main.go:156` gRPC serve | none | Wrap in `errgroup`, wait at shutdown |
| 2 | `cmd/main.go:197` HTTP serve | none | Same |
| 3 | `server.go:171` `go wsHub.Run()` | none, no exit path | `Hub` gets `Close()`; `Run` selects on `ctx.Done()` |
| 4 | `websocket.go:115` `writePump` | `HandleWebSocket` | Keep; document the exit path |
| 5 | `websocket.go:116` `readPump` | `HandleWebSocket` | Keep; fix the double-close |
| 6 | `websocket.go:117` `pingPump` | `HandleWebSocket` | Keep; document |
| 7 | `assets/service.go:239` `processAsset` | none | Bounded pool owned by `Service`, or asynq |
| 8 | `assets/service.go:254` `processAsset` | none, `context.Background()` | Same, plus take a `ctx` |
| 9 | `assets/service.go:891` | none | Same |
| 10 | `libraries/service.go:286` | none | Same |

No `goleak` anywhere today; adding it to `TestMain` is what makes items 3, 7–10 stay fixed.

---

## 5. Hard constraints discovered (read before moving any file)

1. **`internal/openapicoverage.ParseManualRouteDir` globs exactly one directory**
   (`filepath.Glob(dir + "/*.go")`, `manual.go:27`) and finds routes by AST-matching
   **literal** `r.URL.Path == "/api/…"` comparisons and `http.Method*` constants.
   Consequences: moving `http_*.go` out of `internal/server/`, or replacing any route
   literal with a variable/table/helper, **fails the `OPENAPI_COVERAGE_FAIL_UNDER: 100`
   gate even though the endpoint still works**. Any such move must ship together with a
   coverage-tool change (recursive walk, or an explicit multi-dir flag) in the same commit.
2. **A fresh clone does not compile.** `internal/proto/gen/` is gitignored (`.gitignore:2`,
   0 files tracked); `buf generate` is required first. Every metric above was taken after
   generating.
3. **No Docker in this environment** → the 30 `//go:build integration` files can only be
   compile-checked (`go vet -tags=integration`). Characterization tests for DB-touching
   code must be written so they'd run in CI, and I cannot prove them green locally.
   And right now that compile check **fails** (see §2.7 #4) — that break has to be fixed
   before Phase B can claim "every commit green" for the integration tier.
4. **Parity outranks elegance.** Weird DTO shapes, wide flat structs, and per-endpoint
   special cases are the product. Nothing in §2 proposes changing a wire shape.

---

## 6. Doc drift — every statement that is false or unverifiable

| Doc | Claim | Reality |
|---|---|---|
| `README.md` | "Generated proto code under `internal/proto/gen/` is committed, so a fresh checkout builds without running `buf generate`." | **False.** `.gitignore:2` ignores it; `git ls-files` returns 0. `CLAUDE.md` says the opposite. |
| `README.md` (layout) | "`proto/` .proto sources + generated Go (gen/ is committed)" | **False**, same reason. |
| `README.md` | "34 gRPC services (~222 RPCs)" | 34 services ✓, but **274** RPCs. |
| `README.md` | "`schema.sql` Database schema (43 tables)" | **50** `CREATE TABLE`. |
| `README.md` | "`queries.sql` 240 SQLC query definitions" | **314** `-- name:` entries. |
| `README.md` | "Most fields are overridden by unprefixed environment variables… The authoritative list lives in the struct tags of `internal/config/config.go`." | **False and actively harmful.** The tags are never read; 56 of 72 do nothing. The authoritative list is the `os.Getenv` block at `config.go:497`. |
| `README.md` (features table) | `FEATURE_MACHINE_LEARNING_ENABLED`, `FEATURE_VIDEO_TRANSCODING_ENABLED`, `FEATURE_EXIF_EXTRACTION_ENABLED`, … documented as env vars | **None are read.** |
| `README.md` | "The exceptions use an `IMMICH_` prefix: `IMMICH_WEBUI_DIR`, `IMMICH_DATABASE_AUTO_MIGRATE`, `IMMICH_EMBEDDED_DB`" | Incomplete — `IMMICH_JOBS_RETRY_MAX_RETRIES` also exists. |
| `README.md` | "`make test` — unit + integration tests (needs Docker)" | **False.** `make test` is `go test ./...`: no `-short`, no `-tags=integration`, so it runs unit tests only. |
| `README.md` | "The linter is golangci-lint with errcheck, ineffassign, staticcheck, unused, gosec, govet, gofmt, gofumpt, goimports, misspell, dogsled, nakedret, gocritic, **gosimple**, unconvert, copyloopvar" | `gosimple` no longer exists in golangci-lint v2 (folded into `staticcheck`). Rest ✓ via defaults + the `enable` block. |
| `README.md` | "CI runs on Go 1.24 + buf 1.49.0" | Go 1.24 ✓ (`GO_VERSION: '1.24'`); buf version is not pinned in the workflow — `bufbuild/buf-action@v1` chooses it. |
| `README.md` | "PostgreSQL 16" via compose | ✓ `postgres:16-alpine` — but the E2E workflow uses `tensorchord/vchord-suite:pg17`. Worth stating both. |
| `README.md` | "Per-package shard files live under `.github/test-shards/` and are not yet consumed by the workflow." | ✓ files exist; the "not consumed" half needs re-verifying in Phase B. |
| `README.md` / `ROADMAP.md` | "tracks upstream Immich `v3.0.2` stable" | CI pins `OPENAPI_UPSTREAM_REF: v3.0.1` and `IMMICH_UPSTREAM_REF: v2.7.5`. Doc and gate disagree. |
| `README.md` | "graceful shutdown" as a feature | Partly true: HTTP has `Shutdown`, gRPC has `GracefulStop`. But `Server.Shutdown` is a `return nil` stub, the WS hub goroutine is never awaited, and in-flight `processAsset` goroutines are abandoned. |
| `README.md` | Live demo at `https://immich-go-backend.fly.dev` with published credentials | **Unverifiable offline.** Per §5 rules it must be re-verified or deleted, not hedged. |
| `ROADMAP.md` | "Configurable worker pools for background jobs (`JOBS_WORKERS`, default 4)" marked `[x]` **twice** | `JOBS_WORKERS` is one of the 56 ignored tags. The default is real; the env var is not. |
| `ARCHITECTURE.md`, `DEPLOYMENT.md`, `TESTING.md`, `CONTRIBUTING.md` | not yet swept | 1,255 lines still to check symbol-by-symbol in Phase B step 7 |
| every package | no `doc.go`, no concurrency contract on any of the 11 locked types | — |

---

## 7. Ordered plan

Highest value / lowest risk first. Each numbered item is one commit; each is independently
green on `build` + `go vet` + `go test -short` + `-race` + `golangci-lint`.

**Batch 0 — unblock (must land first)**

1. `fix(stacks): repair integration test call to CreateStack` — one-line signature fix. Without it the integration tier cannot be verified at all. *(behaviour-neutral, test-only)*

**Batch 1 — characterization tests (§2 rule 3)**

2. `test(websocket): pin client lifecycle — close, disconnect, buffer-full` — reproduces the double-close panic before anything is touched.
3. `test(config): pin loadFromEnv + setDefaults for every honoured variable` — the safety net for Batch 4.
4. `test(server): pin systemconfig proto↔dto round-trip` — the safety net for the cognit-99 split.
5. `test(hls): pin ensureHLS reuse + concurrent-request behaviour` — safety net for the lock replacement.

**Batch 2 — deletions (cheapest complexity reduction available)**

6. `chore(auth): delete unused gin middleware` — ~250 lines, drops `gin-gonic` as a direct dependency.
7. `chore(server): delete 11 write-only service fields and the no-op Shutdown stub`.
8. `chore(server): delete the errors.go alias shim, call grpcutil directly`.
9. `chore(websocket): delete the write-only clients map and its lock`.
10. `chore(config): delete the 56 env tags and default tags nothing reads` — with a struct doc comment pointing at `loadFromEnv`/`setDefaults`. *(Wiring them up is a behaviour change and is deliberately NOT in this commit — see `FINDINGS.md`.)*

**Batch 3 — mechanical moves (`git mv` only, zero content edits)**

11. `refactor: move internal/server/socketio → internal/socketio`.
12. `refactor: retire internal/util into pgutil / pagination / ptr`.
13. `refactor: rename server/utils.go → server/authcontext.go`.
14. `refactor(server): split server.go into server/services/grpc_register/gateway/logging`.
15. `refactor(cmd): split runServer into lifecycle stages; main.go under 100 lines`.

**Batch 4 — complexity, one theme per commit**

16. `refactor(server): drive gateway registration from a table; join errors` — and make failure fatal (this one **is** a behaviour change: startup now fails loudly; call it out).
17. `refactor(server): split applyProtoToSystemConfig into per-section appliers`.
18. `refactor(assets): separate EXIF mapping from EXIF I/O in metadata.go`.
19. `refactor(server): split handleFrontendShape per HTTP method, literals preserved`.
20. `refactor: split the remaining 500+ line files` (one commit per package: assets, jobs, server/asset, admin, auth).

**Batch 5 — duplication**

21. `refactor(telemetry): one NewServiceMetrics helper for the 5 cloned constructors` — **BREAKING:** `duplicates/faces/stacks/users/workflow.NewService` lose their `error` return.
22. `refactor: collapse the same-file service clones (trash, albums, memories, tags, sessions)`; annotate the cross-domain lookalikes as independent.
23. `test: collapse duplicated tests into table-driven subtests`.

**Batch 6 — concurrency ownership, one type per commit, `-race` each**

24. `refactor(websocket): Hub owns its state; New(ctx) returns a Close handle`.
25. `refactor(websocket): single-owner close of Client.done; non-blocking pong send`.
26. `refactor(server): replace the global HLS mutex with a ctx-aware keyed in-flight map` + benchmark, since this is a media hot path.
27. `refactor(sync): one owner goroutine for the three sync maps, with stated eviction`.
28. `refactor(server): move versionCheckCache and its HTTP client onto Server`.
29. `refactor(assets): bounded, owned worker pool for processAsset`.
30. `refactor(auth): bound the login-attempt map`.
31. `docs: concurrency contract comment on every surviving lock` + `docs/adr/0001–0003`.

**Batch 7 — docs**

32. `docs: correct every false statement listed in §6` (README, ROADMAP first).
33. `docs: sweep ARCHITECTURE / DEPLOYMENT / TESTING / CONTRIBUTING symbol-by-symbol`.
34. `docs: package doc.go for every package over 3 files`.
35. `docs: doc comments on exported identifiers, stating guarantees`.
36. `docs: convert illustrative snippets into Example* tests so they can rot loudly`.

**Batch 8 — lock it in**

37. `ci: add goleak to TestMain in the concurrency-owning packages`.
38. `ci: enable gocognit, funlen, nestif, errorlint, nilerr, contextcheck, containedctx, noctx, bodyclose, thelper, tparallel, dupl` — with a baseline nolint only where §2 justified an exception in writing.
39. `ci: depguard boundary rules + forbidigo on sync.Mutex/RWMutex`.
40. `ci: make gofumpt drift actually fail`.
41. `docs: refactor-report.md with before/after metrics`.

**Deferred, needs your decision (not in the plan above)**

- `internal/queue` vs `internal/jobs`: two job systems, one in-memory and one real asynq. Deleting or persisting the in-memory one is a product call.
- `plugin` and `maintenance` in-memory registries: same class — persist via SQLC per project Rule 1, or accept as intentionally ephemeral and document it.
- Breaking up the `Server` god object across packages: blocked on the §5 coverage-tool constraint.
- Fixing the 56 ignored env vars, the ignored HTTP timeouts, and `CheckOrigin` — all real behaviour changes, all in `FINDINGS.md` awaiting a `fix:` go-ahead.

### `BREAKING:`

- **Commit 21** — `duplicates.NewService`, `faces.NewService`, `stacks.NewService`, `users.NewService`, `workflow.NewService` drop their `error` return once metric construction is centralised. All callers are in-repo (`internal/server/server.go`).
- **Commit 16** — gateway handler registration failure changes from "log and continue" to "fail startup". Intended, and the whole point, but it is a behaviour change.
