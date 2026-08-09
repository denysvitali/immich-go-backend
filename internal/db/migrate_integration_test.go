//go:build integration

package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeMigrationSchemaParity(t *testing.T) {
	testdb.SkipIfNoDocker(t)
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	t.Run("forward migrations create asset views and backfill bootstrap state", func(t *testing.T) {
		_, err := tdb.DB.ExecContext(ctx, `
			DROP TABLE public.asset_views;
			CREATE TABLE schema_migrations (
				version INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
		`)
		require.NoError(t, err)
		_, err = tdb.DB.ExecContext(ctx, `
			INSERT INTO public.users (email, password, name, "isAdmin")
			VALUES ('existing-admin@test.com', 'not-used', 'Existing Admin', true)
		`)
		require.NoError(t, err)

		migrations, err := loadMigrations()
		require.NoError(t, err)
		for _, migration := range migrations {
			if migration.Version >= 5 {
				continue
			}
			_, err = tdb.DB.ExecContext(ctx,
				`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
				migration.Version, migration.Name)
			require.NoError(t, err)
		}

		require.NoError(t, RunMigrations(ctx, tdb.DB))
		assertAssetViewsSchema(t, ctx, tdb)

		var bootstrapMarker bool
		require.NoError(t, tdb.DB.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM public.system_metadata
				WHERE key = 'initial_admin_bootstrap'
			)
		`).Scan(&bootstrapMarker))
		assert.True(t, bootstrapMarker,
			"upgrading an initialized installation must permanently close bootstrap")
	})

	t.Run("clean migrations create every SQLC table", func(t *testing.T) {
		_, err := tdb.DB.ExecContext(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
		require.NoError(t, err)
		require.NoError(t, RunMigrations(ctx, tdb.DB))

		schema, err := os.ReadFile(filepath.Join("..", "..", "sqlc", "schema.sql"))
		require.NoError(t, err)
		expected := publicTableNames(string(schema))

		rows, err := tdb.DB.QueryContext(ctx, `
			SELECT c.relname
			FROM pg_catalog.pg_class AS c
			JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
			WHERE n.nspname = 'public'
			  AND c.relkind IN ('r', 'p')
			  AND c.relname <> 'schema_migrations'
			  AND NOT EXISTS (
				SELECT 1
				FROM pg_catalog.pg_depend AS d
				WHERE d.classid = 'pg_catalog.pg_class'::regclass
				  AND d.objid = c.oid
				  AND d.deptype = 'e'
			  )
			ORDER BY c.relname
		`)
		require.NoError(t, err)
		defer rows.Close()

		actual := make([]string, 0, len(expected))
		for rows.Next() {
			var table string
			require.NoError(t, rows.Scan(&table))
			actual = append(actual, table)
		}
		require.NoError(t, rows.Err())
		assert.Equal(t, expected, actual)

		migrations, err := loadMigrations()
		require.NoError(t, err)
		var applied int
		require.NoError(t, tdb.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM schema_migrations`).Scan(&applied))
		assert.Equal(t, len(migrations), applied)
	})
}

func assertAssetViewsSchema(t *testing.T, ctx context.Context, tdb *testdb.TestDB) {
	t.Helper()

	type column struct {
		name       string
		dataType   string
		nullable   string
		defaultSQL string
	}
	rows, err := tdb.DB.QueryContext(ctx, `
		SELECT column_name, data_type, is_nullable, COALESCE(column_default, '')
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'asset_views'
		ORDER BY ordinal_position
	`)
	require.NoError(t, err)
	defer rows.Close()

	var columns []column
	for rows.Next() {
		var current column
		require.NoError(t, rows.Scan(
			&current.name, &current.dataType, &current.nullable, &current.defaultSQL))
		columns = append(columns, current)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []column{
		{name: "asset_id", dataType: "uuid", nullable: "NO"},
		{name: "user_id", dataType: "uuid", nullable: "NO"},
		{name: "viewed_at", dataType: "timestamp with time zone", nullable: "NO", defaultSQL: "now()"},
	}, columns)

	constraintRows, err := tdb.DB.QueryContext(ctx, `
		SELECT conname
		FROM pg_catalog.pg_constraint
		WHERE conrelid = 'public.asset_views'::regclass
		ORDER BY conname
	`)
	require.NoError(t, err)
	defer constraintRows.Close()
	var constraints []string
	for constraintRows.Next() {
		var name string
		require.NoError(t, constraintRows.Scan(&name))
		constraints = append(constraints, name)
	}
	require.NoError(t, constraintRows.Err())
	assert.Equal(t, []string{
		"asset_views_asset_id_fkey",
		"asset_views_pkey",
		"asset_views_user_id_fkey",
	}, constraints)

	indexRows, err := tdb.DB.QueryContext(ctx, `
		SELECT indexname
		FROM pg_catalog.pg_indexes
		WHERE schemaname = 'public' AND tablename = 'asset_views'
		ORDER BY indexname
	`)
	require.NoError(t, err)
	defer indexRows.Close()
	var indexes []string
	for indexRows.Next() {
		var name string
		require.NoError(t, indexRows.Scan(&name))
		indexes = append(indexes, name)
	}
	require.NoError(t, indexRows.Err())
	assert.Equal(t, []string{
		"asset_views_pkey",
		"idx_asset_views_asset_id",
		"idx_asset_views_user_id",
		"idx_asset_views_viewed_at",
	}, indexes)
}
