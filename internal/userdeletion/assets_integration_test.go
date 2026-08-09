//go:build integration

package userdeletion

import (
	"context"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingAssetDeleter struct {
	ids []uuid.UUID
}

func (d *recordingAssetDeleter) HardDeleteAsset(_ context.Context, assetID, _ uuid.UUID) error {
	d.ids = append(d.ids, assetID)
	return nil
}

func TestIntegrationPermanentUserDeletionFencesUploadsAndIncludesTombstones(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()
	ownerID := tdb.CreateTestUser(t, "delete-fence@example.com")
	activeID := tdb.CreateTestAsset(t, ownerID, "active")
	tombstoneID := tdb.CreateTestAsset(t, ownerID, "legacy-tombstone")
	ownerUUID := pgutil.UUIDToPgtype(ownerID)

	require.NoError(t, tdb.Queries.PermanentlyDeleteAssets(ctx, sqlc.PermanentlyDeleteAssetsParams{
		Column1: []pgtype.UUID{pgutil.UUIDToPgtype(tombstoneID)},
		OwnerId: ownerUUID,
	}))
	// Hold an upload transaction open after it has verified the active owner.
	// The deletion fence must wait for this asset to commit before enumerating.
	uploadTx, err := tdb.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = uploadTx.Rollback(ctx) }()
	inFlightAsset, err := sqlc.New(uploadTx).CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    "in-flight",
		OwnerId:          ownerUUID,
		DeviceId:         "test-device",
		Type:             "IMAGE",
		OriginalPath:     "/test/path/in-flight.jpg",
		OriginalFileName: "in-flight.jpg",
		Checksum:         []byte("in-flight-checksum"),
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err)

	type markResult struct {
		rows int64
		err  error
	}
	markDone := make(chan markResult, 1)
	go func() {
		rows, markErr := tdb.Queries.MarkUserForPermanentDeletion(ctx, ownerUUID)
		markDone <- markResult{rows: rows, err: markErr}
	}()

	select {
	case result := <-markDone:
		t.Fatalf("deletion fence did not wait for in-flight upload: rows=%d err=%v", result.rows, result.err)
	case <-time.After(200 * time.Millisecond):
	}
	require.NoError(t, uploadTx.Commit(ctx))

	select {
	case result := <-markDone:
		require.NoError(t, result.err)
		require.EqualValues(t, 1, result.rows)
	case <-time.After(5 * time.Second):
		t.Fatal("deletion fence remained blocked after upload commit")
	}

	_, err = tdb.Queries.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    "late-upload",
		OwnerId:          ownerUUID,
		DeviceId:         "test-device",
		Type:             "IMAGE",
		OriginalPath:     "/test/path/late-upload.jpg",
		OriginalFileName: "late-upload.jpg",
		Checksum:         []byte("late-upload-checksum"),
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.ErrorIs(t, err, pgx.ErrNoRows)

	deleter := &recordingAssetDeleter{}
	require.NoError(t, DeleteOwnedAssets(ctx, tdb.Queries, deleter, ownerID))
	assert.ElementsMatch(t, []uuid.UUID{activeID, tombstoneID, uuid.UUID(inFlightAsset.ID.Bytes)}, deleter.ids)
}
