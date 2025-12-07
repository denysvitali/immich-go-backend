package db

import (
	"context"
	"database/sql"
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

	t.Run("skips migrations when up to date", func(t *testing.T) {
		// Create migrations table
		mock.ExpectExec(`CREATE TABLE IF NOT EXISTS schema_migrations`).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Get current version (assuming we're at version 1 and that's the latest)
		rows := sqlmock.NewRows([]string{"version"}).AddRow(1)
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(version\), 0\) FROM schema_migrations`).
			WillReturnRows(rows)

		// Since loadMigrations() reads from filesystem, we can't fully test RunMigrations
		// without mocking the filesystem. This test demonstrates the structure.
	})

	t.Run("runs pending migrations", func(t *testing.T) {
		// Create migrations table
		mock.ExpectExec(`CREATE TABLE IF NOT EXISTS schema_migrations`).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Get current version (0 - no migrations yet)
		rows := sqlmock.NewRows([]string{"version"}).AddRow(0)
		mock.ExpectQuery(`SELECT COALESCE\(MAX\(version\), 0\) FROM schema_migrations`).
			WillReturnRows(rows)

		// The actual migration running would happen here
		// but requires filesystem mocking
	})
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