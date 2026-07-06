//go:build integration
// +build integration

package sync

import (
	"context"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationGetDeltaSyncReportsDeletedAssets(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	ctx := context.Background()
	tdb := testdb.SetupTestDB(t)

	ownerID := tdb.CreateTestUser(t, "sync-owner@example.com")
	otherID := tdb.CreateTestUser(t, "sync-other@example.com")

	cursor := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	beforeCursor := cursor.Add(-time.Minute)
	afterCursor := cursor.Add(time.Minute)

	activeRecent := tdb.CreateTestAsset(t, ownerID, "active-recent")
	trashedRecent := tdb.CreateTestAsset(t, ownerID, "trashed-recent")
	deletedRecent := tdb.CreateTestAsset(t, ownerID, "deleted-recent")
	hardDeletedRecent := tdb.CreateTestAsset(t, ownerID, "hard-deleted-recent")
	trashedBefore := tdb.CreateTestAsset(t, ownerID, "trashed-before")
	otherTrashed := tdb.CreateTestAsset(t, otherID, "other-trashed")

	setSyncAssetState(t, ctx, tdb, activeRecent, sqlc.AssetsStatusEnumActive, afterCursor, nil)
	setSyncAssetState(t, ctx, tdb, trashedRecent, sqlc.AssetsStatusEnumTrashed, afterCursor, nil)
	setSyncAssetState(t, ctx, tdb, deletedRecent, sqlc.AssetsStatusEnumDeleted, afterCursor, nil)
	setSyncAssetState(t, ctx, tdb, hardDeletedRecent, sqlc.AssetsStatusEnumActive, afterCursor, &afterCursor)
	setSyncAssetState(t, ctx, tdb, trashedBefore, sqlc.AssetsStatusEnumTrashed, beforeCursor, nil)
	setSyncAssetState(t, ctx, tdb, otherTrashed, sqlc.AssetsStatusEnumTrashed, afterCursor, nil)

	service := NewService(tdb.Queries, nil)
	service.lastSync[ownerID.String()] = cursor

	result, err := service.GetDeltaSync(ctx, ownerID.String(), cursor)
	require.NoError(t, err)

	assert.False(t, result.NeedsFullSync)
	assert.ElementsMatch(t, []string{activeRecent.String()}, result.UpsertedAssets)
	assert.ElementsMatch(t, []string{
		trashedRecent.String(),
		deletedRecent.String(),
		hardDeletedRecent.String(),
	}, result.DeletedAssets)
}

func setSyncAssetState(
	t *testing.T,
	ctx context.Context,
	tdb *testdb.TestDB,
	assetID uuid.UUID,
	status sqlc.AssetsStatusEnum,
	updatedAt time.Time,
	deletedAt *time.Time,
) {
	t.Helper()

	deletedAtValue := pgtype.Timestamptz{}
	if deletedAt != nil {
		deletedAtValue = pgtype.Timestamptz{Time: *deletedAt, Valid: true}
	}

	_, err := tdb.Pool.Exec(ctx, `
		UPDATE assets
		SET status = $2::assets_status_enum,
		    "updatedAt" = $3,
		    "deletedAt" = $4
		WHERE id = $1
	`, pgtype.UUID{Bytes: assetID, Valid: true}, string(status), updatedAt, deletedAtValue)
	require.NoError(t, err)
}
