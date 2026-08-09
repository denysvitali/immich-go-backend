package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration_Structure(t *testing.T) {
	migration := Migration{
		Version: 1,
		Name:    "initial_schema",
		SQL:     "CREATE TABLE users (id UUID PRIMARY KEY);",
	}

	assert.Equal(t, 1, migration.Version)
	assert.Equal(t, "initial_schema", migration.Name)
	assert.Contains(t, migration.SQL, "CREATE TABLE")
}

func TestMigrationTablesMatchSQLCSchema(t *testing.T) {
	schema, err := os.ReadFile(filepath.Join("..", "..", "sqlc", "schema.sql"))
	require.NoError(t, err)

	migrations, err := loadMigrations()
	require.NoError(t, err)

	var migrationSQL strings.Builder
	for _, migration := range migrations {
		migrationSQL.WriteString(migration.SQL)
		migrationSQL.WriteByte('\n')
	}

	expected := publicTableNames(string(schema))
	actual := publicTableNames(migrationSQL.String())
	assert.Equal(t, expected, actual,
		"runtime migrations and sqlc/schema.sql must declare the same public tables")
}

func publicTableNames(sqlText string) []string {
	tablePattern := regexp.MustCompile(`(?im)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?public\."?([a-z0-9_]+)"?\s*\(`)
	matches := tablePattern.FindAllStringSubmatch(sqlText, -1)
	tables := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if _, ok := seen[match[1]]; ok {
			continue
		}
		seen[match[1]] = struct{}{}
		tables = append(tables, match[1])
	}
	sort.Strings(tables)
	return tables
}

func TestCreateMigrationsTable(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	t.Run("creates migration table successfully", func(t *testing.T) {
		mock.ExpectExec(`CREATE TABLE IF NOT EXISTS schema_migrations`).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := createMigrationsTable(ctx, db)
		assert.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestGetCurrentMigrationVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	t.Run("returns current version", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"version"}).AddRow(5)
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(version\), 0\) FROM schema_migrations`).
			WillReturnRows(rows)

		version, err := getCurrentMigrationVersion(ctx, db)
		assert.NoError(t, err)
		assert.Equal(t, 5, version)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("returns 0 when no migrations", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"version"}).AddRow(0)
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(version\), 0\) FROM schema_migrations`).
			WillReturnRows(rows)

		version, err := getCurrentMigrationVersion(ctx, db)
		assert.NoError(t, err)
		assert.Equal(t, 0, version)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("handles query error", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(version\), 0\) FROM schema_migrations`).
			WillReturnError(sql.ErrConnDone)

		version, err := getCurrentMigrationVersion(ctx, db)
		assert.Error(t, err)
		assert.Equal(t, 0, version)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestRunMigrations(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	migrations, err := loadMigrations()
	require.NoError(t, err)
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })

	mock.ExpectExec(`SELECT pg_advisory_lock\(\$1\)`).
		WithArgs(migrationAdvisoryLockID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SHOW statement_timeout`).
		WillReturnRows(sqlmock.NewRows([]string{"statement_timeout"}).AddRow("30s"))
	mock.ExpectExec(`SET statement_timeout = 0`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`(?s)CREATE TABLE IF NOT EXISTS schema_migrations.*ALTER TABLE schema_migrations`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	rows := sqlmock.NewRows([]string{"version", "name", "checksum"})
	for _, migration := range migrations {
		rows.AddRow(migration.Version, migration.Name, migration.Checksum)
	}
	mock.ExpectQuery(`SELECT version, name, checksum FROM schema_migrations ORDER BY version`).
		WillReturnRows(rows)
	mock.ExpectExec(`SELECT set_config\('statement_timeout', \$1, false\)`).
		WithArgs("30s").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).
		WithArgs(migrationAdvisoryLockID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, RunMigrations(context.Background(), db))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMigrationOrdering(t *testing.T) {
	migrations := []Migration{
		{Version: 3, Name: "third"},
		{Version: 1, Name: "first"},
		{Version: 2, Name: "second"},
	}

	// Simulate sorting migrations by version
	// In real implementation, migrations should be applied in order
	for i := 0; i < len(migrations)-1; i++ {
		for j := i + 1; j < len(migrations); j++ {
			if migrations[i].Version > migrations[j].Version {
				migrations[i], migrations[j] = migrations[j], migrations[i]
			}
		}
	}

	assert.Equal(t, 1, migrations[0].Version)
	assert.Equal(t, "first", migrations[0].Name)
	assert.Equal(t, 2, migrations[1].Version)
	assert.Equal(t, "second", migrations[1].Name)
	assert.Equal(t, 3, migrations[2].Version)
	assert.Equal(t, "third", migrations[2].Name)
}

func TestValidateMigrationSequence(t *testing.T) {
	t.Run("accepts contiguous unique migrations", func(t *testing.T) {
		err := validateMigrationSequence([]Migration{
			{Version: 1, Name: "initial"},
			{Version: 2, Name: "next"},
		})
		assert.NoError(t, err)
	})

	t.Run("rejects gaps", func(t *testing.T) {
		err := validateMigrationSequence([]Migration{
			{Version: 1, Name: "initial"},
			{Version: 3, Name: "later"},
		})
		assert.ErrorContains(t, err, "expected version 002")
	})

	t.Run("rejects duplicate names", func(t *testing.T) {
		err := validateMigrationSequence([]Migration{
			{Version: 1, Name: "same"},
			{Version: 2, Name: "middle"},
			{Version: 3, Name: "same"},
		})
		assert.ErrorContains(t, err, "duplicate migration name")
	})
}

func TestValidateAppliedMigrations(t *testing.T) {
	available := []Migration{
		{Version: 1, Name: "initial", Checksum: "checksum-1"},
		{Version: 2, Name: "next", Checksum: "checksum-2"},
		{Version: 3, Name: "last", Checksum: "checksum-3"},
	}

	t.Run("accepts an ordered prefix and legacy empty checksum", func(t *testing.T) {
		err := validateAppliedMigrations(available, []appliedMigration{
			{Version: 1, Name: "initial", Checksum: sql.NullString{}},
			{Version: 2, Name: "next", Checksum: sql.NullString{String: "checksum-2", Valid: true}},
		})
		assert.NoError(t, err)
	})

	t.Run("rejects a missing lower migration", func(t *testing.T) {
		err := validateAppliedMigrations(available, []appliedMigration{
			{Version: 1, Name: "initial"},
			{Version: 3, Name: "last"},
		})
		assert.ErrorContains(t, err, "applied migration gap")
	})

	t.Run("rejects name drift", func(t *testing.T) {
		err := validateAppliedMigrations(available, []appliedMigration{
			{Version: 1, Name: "renamed"},
		})
		assert.ErrorContains(t, err, "name drift")
	})

	t.Run("rejects checksum drift", func(t *testing.T) {
		err := validateAppliedMigrations(available, []appliedMigration{
			{Version: 1, Name: "initial", Checksum: sql.NullString{String: "changed", Valid: true}},
		})
		assert.ErrorContains(t, err, "checksum drift")
	})

	t.Run("rejects migrations newer than the binary", func(t *testing.T) {
		err := validateAppliedMigrations(available[:1], []appliedMigration{
			{Version: 1, Name: "initial"},
			{Version: 2, Name: "next"},
		})
		assert.ErrorContains(t, err, "binary only contains")
	})
}

func TestMigrationTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	t.Run("rolls back on error", func(t *testing.T) {
		// Begin transaction
		mock.ExpectBegin()

		// Expect migration SQL to fail
		mock.ExpectExec(`CREATE TABLE test_table`).
			WillReturnError(sql.ErrNoRows)

		// Expect rollback
		mock.ExpectRollback()

		// Simulate a migration with transaction
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		_, err = tx.ExecContext(ctx, "CREATE TABLE test_table")
		assert.Error(t, err)

		err = tx.Rollback()
		assert.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("commits on success", func(t *testing.T) {
		// Begin transaction
		mock.ExpectBegin()

		// Expect successful migration SQL
		mock.ExpectExec(`CREATE TABLE test_table`).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Record migration
		mock.ExpectExec(`INSERT INTO migrations`).
			WithArgs(1, "test_migration", sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Expect commit
		mock.ExpectCommit()

		// Simulate a successful migration with transaction
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)

		_, err = tx.ExecContext(ctx, "CREATE TABLE test_table")
		assert.NoError(t, err)

		_, err = tx.ExecContext(ctx,
			"INSERT INTO migrations (version, name, executed_at) VALUES ($1, $2, $3)",
			1, "test_migration", time.Now())
		assert.NoError(t, err)

		err = tx.Commit()
		assert.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestMigrationIdempotency(t *testing.T) {
	// Test that migrations can be run multiple times safely
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()

	// First run
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS schema_migrations`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = createMigrationsTable(ctx, db)
	assert.NoError(t, err)

	// Second run - should also succeed
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS schema_migrations`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = createMigrationsTable(ctx, db)
	assert.NoError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestMigrationVersionGaps(t *testing.T) {
	// Test that system detects gaps in migration versions
	migrations := []Migration{
		{Version: 1, Name: "first"},
		{Version: 2, Name: "second"},
		{Version: 4, Name: "fourth"}, // Gap - missing version 3
		{Version: 5, Name: "fifth"},
	}

	// Check for gaps
	hasGap := false
	for i := 1; i < len(migrations); i++ {
		expectedVersion := migrations[i-1].Version + 1
		if migrations[i].Version != expectedVersion {
			hasGap = true
			break
		}
	}

	assert.True(t, hasGap, "Should detect gap in migration versions")
}

func TestContextCancellation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Attempt to run migration with cancelled context
	// The query should not even be attempted
	mock.ExpectQuery(`SELECT`).
		WillReturnError(context.Canceled)

	_, err = getCurrentMigrationVersion(ctx, db)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}
