package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migration represents a database migration
type Migration struct {
	Version  int
	Name     string
	SQL      string
	Checksum string
}

type appliedMigration struct {
	Version  int
	Name     string
	Checksum sql.NullString
}

const migrationAdvisoryLockID int64 = 0x494d4d494348474f

// RunMigrations runs all pending database migrations
func RunMigrations(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to reserve migration connection: %w", err)
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(unlockCtx, "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockID)
		_ = conn.Close()
	}()

	// A session-scoped lock serializes migrations across application replicas.
	// Closing the reserved connection releases it even on cancellation/failure.
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationAdvisoryLockID); err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	var previousStatementTimeout string
	if err := conn.QueryRowContext(ctx, "SHOW statement_timeout").Scan(&previousStatementTimeout); err != nil {
		return fmt.Errorf("failed to read migration statement timeout: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "SET statement_timeout = 0"); err != nil {
		return fmt.Errorf("failed to disable statement timeout for migrations: %w", err)
	}
	defer func() {
		restoreCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(restoreCtx,
			"SELECT set_config('statement_timeout', $1, false)", previousStatementTimeout)
	}()

	// Create migrations table if it doesn't exist
	if err := createMigrationsTable(ctx, conn); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Load all migrations
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("failed to load migrations: %w", err)
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	if err := validateMigrationSequence(migrations); err != nil {
		return err
	}

	applied, err := getAppliedMigrations(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}
	if err := validateAppliedMigrations(migrations, applied); err != nil {
		return err
	}

	// Older installations predate checksum tracking. Once their ordered names
	// are verified, establish a checksum baseline for subsequent drift checks.
	for i, migration := range migrations[:len(applied)] {
		if applied[i].Checksum.Valid && applied[i].Checksum.String != "" {
			continue
		}
		if _, err := conn.ExecContext(ctx,
			"UPDATE schema_migrations SET checksum = $1 WHERE version = $2",
			migration.Checksum, migration.Version,
		); err != nil {
			return fmt.Errorf("failed to record checksum for migration %03d: %w", migration.Version, err)
		}
	}

	// Apply pending migrations
	for _, m := range migrations[len(applied):] {

		logrus.Infof("Applying migration %03d: %s", m.Version, m.Name)

		// Start transaction
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Execute migration
		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to execute migration %03d: %w", m.Version, err)
		}

		// Record migration
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)",
			m.Version, m.Name, m.Checksum,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration %03d: %w", m.Version, err)
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %03d: %w", m.Version, err)
		}

		logrus.Infof("Successfully applied migration %03d", m.Version)
	}

	return nil
}

type migrationDB interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func createMigrationsTable(ctx context.Context, db migrationDB) error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		checksum TEXT,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum TEXT;`

	_, err := db.ExecContext(ctx, query)
	return err
}

func getAppliedMigrations(ctx context.Context, db migrationDB) ([]appliedMigration, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT version, name, checksum FROM schema_migrations ORDER BY version",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var applied []appliedMigration
	for rows.Next() {
		var migration appliedMigration
		if err := rows.Scan(&migration.Version, &migration.Name, &migration.Checksum); err != nil {
			return nil, err
		}
		applied = append(applied, migration)
	}
	return applied, rows.Err()
}

func getCurrentMigrationVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	err := db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(version), 0) FROM schema_migrations",
	).Scan(&version)

	if err == sql.ErrNoRows {
		return 0, nil
	}
	return version, err
}

func loadMigrations() ([]Migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	var migrations []Migration
	filenamePattern := regexp.MustCompile(`^([0-9]{3})_([a-z0-9][a-z0-9_]*)\.sql$`)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		matches := filenamePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			return nil, fmt.Errorf("invalid migration filename %q; expected NNN_name.sql", entry.Name())
		}
		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("invalid migration version in %q: %w", entry.Name(), err)
		}

		// Read SQL content
		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, err
		}

		digest := sha256.Sum256(content)
		migrations = append(migrations, Migration{
			Version:  version,
			Name:     matches[2],
			SQL:      string(content),
			Checksum: hex.EncodeToString(digest[:]),
		})
	}

	return migrations, nil
}

func validateMigrationSequence(migrations []Migration) error {
	names := make(map[string]struct{}, len(migrations))
	for i, migration := range migrations {
		expected := i + 1
		if migration.Version != expected {
			return fmt.Errorf("migration sequence is invalid: expected version %03d, found %03d", expected, migration.Version)
		}
		if _, exists := names[migration.Name]; exists {
			return fmt.Errorf("duplicate migration name %q", migration.Name)
		}
		names[migration.Name] = struct{}{}
	}
	return nil
}

func validateAppliedMigrations(available []Migration, applied []appliedMigration) error {
	if len(applied) > len(available) {
		return fmt.Errorf("database has %d migrations but binary only contains %d", len(applied), len(available))
	}
	for i, actual := range applied {
		expected := available[i]
		if actual.Version != expected.Version {
			return fmt.Errorf("applied migration gap: expected version %03d, found %03d", expected.Version, actual.Version)
		}
		if actual.Name != expected.Name {
			return fmt.Errorf("migration %03d name drift: database has %q, binary has %q", actual.Version, actual.Name, expected.Name)
		}
		if actual.Checksum.Valid && actual.Checksum.String != "" && actual.Checksum.String != expected.Checksum {
			return fmt.Errorf("migration %03d checksum drift", actual.Version)
		}
	}
	return nil
}
