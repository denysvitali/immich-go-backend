// Package testdb provides utilities for integration testing with a real PostgreSQL database
// using testcontainers. This replaces mock-based testing with real database operations.
package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestDB holds the test database connection and container
type TestDB struct {
	Container *postgres.PostgresContainer
	Pool      *pgxpool.Pool
	DB        *sql.DB
	Queries   *sqlc.Queries
	ConnStr   string
}

var (
	sharedTestDB *TestDB
	once         sync.Once
	initErr      error
)

// SetupTestDB creates a new PostgreSQL container for testing.
// It applies the schema from sqlc/schema.sql.
// The container is automatically cleaned up when the test finishes.
func SetupTestDB(t *testing.T) *TestDB {
	t.Helper()

	ctx := context.Background()

	// Find the schema file relative to the project root
	schemaPath := findSchemaPath()
	if schemaPath == "" {
		t.Fatal("Could not find sqlc/schema.sql")
	}

	// Read schema
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("Failed to read schema file: %v", err)
	}
	schema := string(schemaBytes)

	// Create PostgreSQL container with required extensions
	pgContainer, err := postgres.Run(ctx,
		"docker.io/tensorchord/pgvecto-rs:pg17-v0.4.0",
		postgres.WithDatabase("immich_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithInitScripts(), // We'll apply schema manually to handle extensions
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("Failed to start postgres container: %v", err)
	}

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		pgContainer.Terminate(ctx)
		t.Fatalf("Failed to get connection string: %v", err)
	}

	// Connect with pgx pool
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		pgContainer.Terminate(ctx)
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// Wait for database to be ready
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Create required extensions
	extensions := []string{
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,
		`CREATE EXTENSION IF NOT EXISTS "vectors"`,
		`CREATE EXTENSION IF NOT EXISTS "cube"`,
		`CREATE EXTENSION IF NOT EXISTS "earthdistance"`,
		`CREATE EXTENSION IF NOT EXISTS "pg_trgm"`,
		`CREATE EXTENSION IF NOT EXISTS "unaccent"`,
	}

	for _, ext := range extensions {
		if _, err := pool.Exec(ctx, ext); err != nil {
			// Log but don't fail - some extensions might not be available
			t.Logf("Warning: failed to create extension: %v", err)
		}
	}

	// Apply schema
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		pgContainer.Terminate(ctx)
		t.Fatalf("Failed to apply schema: %v", err)
	}

	// Create sql.DB for compatibility
	db := stdlib.OpenDBFromPool(pool)

	// Create SQLC queries
	queries := sqlc.New(pool)

	testDB := &TestDB{
		Container: pgContainer,
		Pool:      pool,
		DB:        db,
		Queries:   queries,
		ConnStr:   connStr,
	}

	// Register cleanup
	t.Cleanup(func() {
		testDB.Close(ctx)
	})

	return testDB
}

// SetupSharedTestDB creates a shared PostgreSQL container for multiple tests.
// This is more efficient when running multiple integration tests.
// Call CleanupSharedTestDB in TestMain to clean up.
func SetupSharedTestDB() (*TestDB, error) {
	once.Do(func() {
		ctx := context.Background()

		schemaPath := findSchemaPath()
		if schemaPath == "" {
			initErr = fmt.Errorf("could not find sqlc/schema.sql")
			return
		}

		schemaBytes, err := os.ReadFile(schemaPath)
		if err != nil {
			initErr = fmt.Errorf("failed to read schema file: %w", err)
			return
		}
		schema := string(schemaBytes)

		pgContainer, err := postgres.Run(ctx,
			"docker.io/tensorchord/pgvecto-rs:pg17-v0.4.0",
			postgres.WithDatabase("immich_test"),
			postgres.WithUsername("test"),
			postgres.WithPassword("test"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(60*time.Second),
			),
		)
		if err != nil {
			initErr = fmt.Errorf("failed to start postgres container: %w", err)
			return
		}

		connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			pgContainer.Terminate(ctx)
			initErr = fmt.Errorf("failed to get connection string: %w", err)
			return
		}

		pool, err := pgxpool.New(ctx, connStr)
		if err != nil {
			pgContainer.Terminate(ctx)
			initErr = fmt.Errorf("failed to connect to database: %w", err)
			return
		}

		// Wait for database to be ready
		for i := 0; i < 30; i++ {
			if err := pool.Ping(ctx); err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Create required extensions
		extensions := []string{
			`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,
			`CREATE EXTENSION IF NOT EXISTS "vectors"`,
			`CREATE EXTENSION IF NOT EXISTS "cube"`,
			`CREATE EXTENSION IF NOT EXISTS "earthdistance"`,
			`CREATE EXTENSION IF NOT EXISTS "pg_trgm"`,
			`CREATE EXTENSION IF NOT EXISTS "unaccent"`,
		}

		for _, ext := range extensions {
			if _, err := pool.Exec(ctx, ext); err != nil {
				// Log but continue - some extensions might not be available
				fmt.Printf("Warning: failed to create extension: %v\n", err)
			}
		}

		// Apply schema
		if _, err := pool.Exec(ctx, schema); err != nil {
			pool.Close()
			pgContainer.Terminate(ctx)
			initErr = fmt.Errorf("failed to apply schema: %w", err)
			return
		}

		db := stdlib.OpenDBFromPool(pool)
		queries := sqlc.New(pool)

		sharedTestDB = &TestDB{
			Container: pgContainer,
			Pool:      pool,
			DB:        db,
			Queries:   queries,
			ConnStr:   connStr,
		}
	})

	return sharedTestDB, initErr
}

// CleanupSharedTestDB should be called in TestMain to clean up the shared container
func CleanupSharedTestDB() {
	if sharedTestDB != nil {
		sharedTestDB.Close(context.Background())
	}
}

// Close terminates the container and closes connections
func (tdb *TestDB) Close(ctx context.Context) {
	if tdb.Pool != nil {
		tdb.Pool.Close()
	}
	if tdb.DB != nil {
		tdb.DB.Close()
	}
	if tdb.Container != nil {
		tdb.Container.Terminate(ctx)
	}
}

// TruncateTables removes all data from the specified tables
func (tdb *TestDB) TruncateTables(ctx context.Context, tables ...string) error {
	for _, table := range tables {
		if _, err := tdb.Pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)); err != nil {
			return fmt.Errorf("failed to truncate table %s: %w", table, err)
		}
	}
	return nil
}

// TruncateAllTables removes all data from all application tables
func (tdb *TestDB) TruncateAllTables(ctx context.Context) error {
	tables := []string{
		"sessions",
		"api_keys",
		"assets",
		"albums",
		"users",
		"libraries",
		"partners",
		"notifications",
		"memories",
		"tags",
		"person",
	}
	return tdb.TruncateTables(ctx, tables...)
}

// findSchemaPath looks for the schema.sql file
func findSchemaPath() string {
	// Try different paths relative to test location
	paths := []string{
		"../../../sqlc/schema.sql",
		"../../sqlc/schema.sql",
		"../sqlc/schema.sql",
		"sqlc/schema.sql",
		"./sqlc/schema.sql",
	}

	// Also try from working directory
	wd, _ := os.Getwd()
	for i := 0; i < 5; i++ {
		testPath := filepath.Join(wd, "sqlc/schema.sql")
		if _, err := os.Stat(testPath); err == nil {
			return testPath
		}
		wd = filepath.Dir(wd)
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			absPath, _ := filepath.Abs(p)
			return absPath
		}
	}

	return ""
}

// SkipIfNoDocker skips the test if Docker is not available
func SkipIfNoDocker(t *testing.T) {
	t.Helper()
	if os.Getenv("SKIP_INTEGRATION_TESTS") != "" {
		t.Skip("Skipping integration test (SKIP_INTEGRATION_TESTS is set)")
	}
}
