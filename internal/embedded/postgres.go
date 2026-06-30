// Package embedded provides an embedded PostgreSQL server for single-binary
// demo deployments. When the IMMICH_EMBEDDED_DB env var is set, the binary
// downloads a pinned Postgres release on first startup (cached on disk),
// creates a fresh cluster under the configured data path, and listens on a
// local TCP port. The returned DSN is then used by the main application as
// if it were a remote database.
//
// Designed for Fly.io's single-machine demo app: pin the BinariesPath and
// DataPath to a persistent volume so the cluster survives restarts and the
// ~30 MB Postgres binary is downloaded only once.
package embedded

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/sirupsen/logrus"
)

// Config controls how the embedded Postgres is provisioned. Zero-value
// fields are filled in from Defaults when Start is called.
type Config struct {
	// DataPath is the directory where the Postgres cluster lives. Must be
	// persistent across restarts.
	DataPath string
	// BinariesPath is the directory where the postgres binaries are
	// cached. Persistent and separate from DataPath so wiping the
	// cluster does not trigger a redownload.
	BinariesPath string
	// Port is the local TCP port the embedded Postgres listens on.
	Port uint32
	// User / Password / Database are created on first start.
	User     string
	Password string
	Database string
	// StartTimeout bounds how long Start may block waiting for the
	// cluster to become ready (download + init + accept connections).
	StartTimeout time.Duration
}

// Runtime holds a running embedded Postgres instance.
type Runtime struct {
	cfg    Config
	server *embeddedpostgres.EmbeddedPostgres
	dsn    string
}

// DSN returns the connection string for the running embedded Postgres.
func (r *Runtime) DSN() string { return r.dsn }

// Stop terminates the embedded Postgres gracefully. Safe to call on a nil
// Runtime.
func (r *Runtime) Stop() error {
	if r == nil || r.server == nil {
		return nil
	}
	return r.server.Stop()
}

// IsEnabled reports whether the embedded Postgres should be started, based
// on the IMMICH_EMBEDDED_DB env var. Accepted truthy values: 1, true, yes
// (case-insensitive).
func IsEnabled() bool {
	switch os.Getenv("IMMICH_EMBEDDED_DB") {
	case "1", "true", "TRUE", "True", "yes", "YES", "Yes":
		return true
	}
	return false
}

// DefaultConfig returns the embedded Postgres configuration used by the
// Fly demo deployment. All paths default to /data so a single persistent
// volume covers the cluster and the cached binary. Override individual
// fields after constructing, or use IMMICH_EMBEDDED_PG_DATA /
// IMMICH_EMBEDDED_PG_BIN env vars.
func DefaultConfig() Config {
	return fillDefaults(Config{})
}

// Start provisions (if first run) and starts the embedded Postgres. The
// returned DSN is suitable for passing to db.New(). Stop must be called by
// the caller on shutdown.
func Start(cfg Config) (*Runtime, error) {
	if cfg = fillDefaults(cfg); cfg.DataPath == "" || cfg.BinariesPath == "" {
		return nil, errors.New("embedded postgres: DataPath and BinariesPath are required")
	}
	if err := os.MkdirAll(cfg.DataPath, 0o755); err != nil {
		return nil, fmt.Errorf("create data path %s: %w", cfg.DataPath, err)
	}
	if err := os.MkdirAll(cfg.BinariesPath, 0o755); err != nil {
		return nil, fmt.Errorf("create binaries path %s: %w", cfg.BinariesPath, err)
	}

	// Resolve absolute paths — embedded-postgres does not always handle
	// relative paths consistently across versions.
	absData, err := filepath.Abs(cfg.DataPath)
	if err != nil {
		return nil, fmt.Errorf("resolve data path: %w", err)
	}
	absBin, err := filepath.Abs(cfg.BinariesPath)
	if err != nil {
		return nil, fmt.Errorf("resolve binaries path: %w", err)
	}

	// Preload vchord at server start. vchord (from tensorchord/vchord)
	// is a custom index access method that hooks into the executor at
	// process startup; `CREATE EXTENSION vchord` requires the .so to
	// already be loaded via `shared_preload_libraries` or it errors
	// with "vchord must be loaded via shared_preload_libraries." and
	// the migration aborts on its 3rd statement.
	//
	// Also silence server-side log noise: stock Postgres is quiet, but
	// embedding the fergusstrange lib inside the Fly 100-line log
	// buffer means any verbose `log_min_messages` setting (or pgx
	// tracelog) would drown out the real ERROR line. Keep ERROR +
	// WARNING only so the migration's failure reason surfaces.
	ep := embeddedpostgres.NewDatabase(
		embeddedpostgres.DefaultConfig().
			Port(cfg.Port).
			DataPath(absData).
			BinariesPath(absBin).
			Username(cfg.User).
			Password(cfg.Password).
			Database(cfg.Database).
			StartTimeout(cfg.StartTimeout).
			StartParameters(map[string]string{
				"shared_preload_libraries": "vchord",
				"log_min_messages":         "warning",
				"log_statement":            "none",
				// Postgres' defaults assume an order of magnitude
				// more RAM than the Fly free tier (shared-cpu-1x /
				// 256 MB) provides. shared_buffers defaults to 1/4
				// of total RAM but STORES as much as 128 MB on first
				// start (postgresql.conf's `min(128MB, 1/4 RAM)`);
				// on a 256 MB VM that's half the box, leaving no
				// headroom for the Go backend, the immich backend's
				// gRPC/REST handlers, or page-cache. Cap at 64 MB
				// so the Go process can actually run alongside it.
				// maintenance_work_mem bumps index/extension builds
				// (vchord creates two vchordrq indexes during
				// migration; default 64 MB OOMs the migration on
				// free-tier). effective_cache_size is informational
				// for the planner (not allocated).
				"shared_buffers":       "64mb",
				"maintenance_work_mem": "32mb",
				"effective_cache_size": "192mb",
				"work_mem":             "4mb",
				"max_connections":      "20",
			}),
	)

	logrus.WithFields(logrus.Fields{
		"data_path":     absData,
		"binaries_path": absBin,
		"port":          cfg.Port,
		"database":      cfg.Database,
	}).Info("starting embedded postgres")

	if err := ep.Start(); err != nil {
		return nil, fmt.Errorf("start embedded postgres: %w", err)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@127.0.0.1:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Port, cfg.Database)
	logrus.WithField("dsn", redactDSN(dsn)).Info("embedded postgres ready")
	return &Runtime{cfg: cfg, server: ep, dsn: dsn}, nil
}

func fillDefaults(c Config) Config {
	if c.DataPath == "" {
		c.DataPath = envOr("IMMICH_EMBEDDED_PG_DATA", "/data/pg")
	}
	if c.BinariesPath == "" {
		// Bundled binary baked into the image by Dockerfile.fly (see
		// stage 0b/2: copied from tensorchord/vchord-suite). The
		// fergusstrange library checks for <BinariesPath>/bin/pg_ctl
		// and skips its Maven download when present. Defaults to the
		// image path; the env override is kept for anyone running a
		// vanilla binary off a volume.
		c.BinariesPath = envOr("IMMICH_EMBEDDED_PG_BIN", "/usr/lib/postgres")
	}
	if c.Port == 0 {
		c.Port = 5433
	}
	if c.User == "" {
		c.User = "immich"
	}
	if c.Password == "" {
		c.Password = "immich"
	}
	if c.Database == "" {
		c.Database = "immich"
	}
	if c.StartTimeout == 0 {
		c.StartTimeout = 120 * time.Second
	}
	return c
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// redactDSN replaces the password component with *** for log output.
func redactDSN(dsn string) string {
	// Cheap parser: find ://, then between the next : and the following @.
	at := -1
	scheme := -1
	for i := 0; i+2 < len(dsn); i++ {
		if scheme < 0 && dsn[i] == ':' && dsn[i+1] == '/' && dsn[i+2] == '/' {
			scheme = i + 3
			i += 2
			continue
		}
		if scheme >= 0 && dsn[i] == '@' {
			at = i
			break
		}
	}
	if scheme < 0 || at < 0 || at <= scheme {
		return dsn
	}
	// user:password@host — find the ':' between scheme and at.
	colon := -1
	for i := scheme; i < at; i++ {
		if dsn[i] == ':' {
			colon = i
			break
		}
	}
	if colon < 0 {
		return dsn
	}
	return dsn[:colon+1] + "***" + dsn[at:]
}
