// +build integration

package trash

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestUser creates a test user and returns the user ID
func createTestUser(t *testing.T, tdb *testdb.TestDB, email string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	userID := uuid.New()
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	_, err := tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    email,
		Name:     "Test User",
		Password: "hashedpassword",
		IsAdmin:  false,
	})
	require.NoError(t, err)

	return userID
}

// createTestAsset creates a test asset and returns the asset ID
func createTestAsset(t *testing.T, tdb *testdb.TestDB, ownerID uuid.UUID, deviceAssetID string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	ownerUUID := pgtype.UUID{Bytes: ownerID, Valid: true}

	asset, err := tdb.Queries.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    deviceAssetID,
		OwnerId:          ownerUUID,
		DeviceId:         "test-device",
		Type:             "IMAGE",
		OriginalPath:     "/test/path/" + deviceAssetID + ".jpg",
		OriginalFileName: deviceAssetID + ".jpg",
		Checksum:         []byte("test-checksum-" + deviceAssetID),
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err)

	return asset.ID.Bytes
}

func TestIntegration_TrashAsset(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and asset
	userID := createTestUser(t, tdb, "trash@test.com")
	assetID := createTestAsset(t, tdb, userID, "trashasset1")

	// Trash the asset
	err := service.TrashAsset(ctx, userID.String(), assetID.String())
	require.NoError(t, err)

	// Verify asset is in trash
	trashedAssets, err := service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, trashedAssets, 1)
	assert.Equal(t, assetID.String(), trashedAssets[0].ID)
}

func TestIntegration_TrashAsset_InvalidAssetID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	userID := createTestUser(t, tdb, "invalidasset@test.com")

	// Try to trash with invalid asset ID
	err := service.TrashAsset(ctx, userID.String(), "not-a-valid-uuid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid asset ID")
}

func TestIntegration_TrashAsset_InvalidUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Try to trash with invalid user ID
	err := service.TrashAsset(ctx, "not-a-valid-uuid", uuid.New().String())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid user ID")
}

func TestIntegration_TrashAsset_NotOwned(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "owner@test.com")
	user2ID := createTestUser(t, tdb, "notowner@test.com")

	// Create asset owned by user1
	assetID := createTestAsset(t, tdb, user1ID, "ownedby1")

	// User2 tries to trash user1's asset
	err := service.TrashAsset(ctx, user2ID.String(), assetID.String())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestIntegration_TrashAssets_Multiple(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and multiple assets
	userID := createTestUser(t, tdb, "multitrash@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "multitrash1")
	asset2ID := createTestAsset(t, tdb, userID, "multitrash2")
	asset3ID := createTestAsset(t, tdb, userID, "multitrash3")

	// Trash multiple assets
	count, err := service.TrashAssets(ctx, userID.String(), []string{
		asset1ID.String(),
		asset2ID.String(),
		asset3ID.String(),
	})
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Verify all are in trash
	trashedAssets, err := service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, trashedAssets, 3)
}

func TestIntegration_RestoreAsset(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and asset
	userID := createTestUser(t, tdb, "restore@test.com")
	assetID := createTestAsset(t, tdb, userID, "restoreasset")

	// Trash the asset
	err := service.TrashAsset(ctx, userID.String(), assetID.String())
	require.NoError(t, err)

	// Verify it's in trash
	trashedAssets, err := service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, trashedAssets, 1)

	// Restore the asset
	err = service.RestoreAsset(ctx, userID.String(), assetID.String())
	require.NoError(t, err)

	// Verify it's no longer in trash
	trashedAssets, err = service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Empty(t, trashedAssets)
}

func TestIntegration_RestoreAsset_NotOwned(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "restoreowner@test.com")
	user2ID := createTestUser(t, tdb, "restorenotowner@test.com")

	// Create and trash asset owned by user1
	assetID := createTestAsset(t, tdb, user1ID, "restoreowned")
	err := service.TrashAsset(ctx, user1ID.String(), assetID.String())
	require.NoError(t, err)

	// User2 tries to restore user1's asset
	err = service.RestoreAsset(ctx, user2ID.String(), assetID.String())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestIntegration_RestoreAssets_Multiple(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and multiple assets
	userID := createTestUser(t, tdb, "multirestore@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "restore1")
	asset2ID := createTestAsset(t, tdb, userID, "restore2")

	// Trash all assets
	_, err := service.TrashAssets(ctx, userID.String(), []string{
		asset1ID.String(),
		asset2ID.String(),
	})
	require.NoError(t, err)

	// Verify in trash
	trashedAssets, err := service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, trashedAssets, 2)

	// Restore multiple assets
	count, err := service.RestoreAssets(ctx, userID.String(), []string{
		asset1ID.String(),
		asset2ID.String(),
	})
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify no longer in trash
	trashedAssets, err = service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Empty(t, trashedAssets)
}

func TestIntegration_RestoreAllAssets(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and multiple assets
	userID := createTestUser(t, tdb, "restoreall@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "restoreall1")
	asset2ID := createTestAsset(t, tdb, userID, "restoreall2")
	asset3ID := createTestAsset(t, tdb, userID, "restoreall3")

	// Trash all assets
	_, err := service.TrashAssets(ctx, userID.String(), []string{
		asset1ID.String(),
		asset2ID.String(),
		asset3ID.String(),
	})
	require.NoError(t, err)

	// Verify in trash
	trashedAssets, err := service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, trashedAssets, 3)

	// Restore all
	restored, err := service.RestoreAllAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Equal(t, 3, restored)

	// Verify no longer in trash
	trashedAssets, err = service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Empty(t, trashedAssets)
}

func TestIntegration_EmptyTrash(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and multiple assets
	userID := createTestUser(t, tdb, "emptytrash@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "empty1")
	asset2ID := createTestAsset(t, tdb, userID, "empty2")

	// Trash all assets
	_, err := service.TrashAssets(ctx, userID.String(), []string{
		asset1ID.String(),
		asset2ID.String(),
	})
	require.NoError(t, err)

	// Verify in trash
	trashedAssets, err := service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, trashedAssets, 2)

	// Empty trash
	deleted, err := service.EmptyTrash(ctx, userID.String())
	require.NoError(t, err)
	assert.Equal(t, 2, deleted)

	// Verify trash is empty
	trashedAssets, err = service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Empty(t, trashedAssets)

	// Verify assets are permanently deleted (can't be found)
	assetUUID := pgtype.UUID{Bytes: asset1ID, Valid: true}
	_, err = tdb.Queries.GetAsset(ctx, assetUUID)
	assert.Error(t, err) // Asset should not be found
}

func TestIntegration_PermanentlyDeleteAsset(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and asset
	userID := createTestUser(t, tdb, "permdelete@test.com")
	assetID := createTestAsset(t, tdb, userID, "permdeleteasset")

	// Trash first
	err := service.TrashAsset(ctx, userID.String(), assetID.String())
	require.NoError(t, err)

	// Permanently delete
	err = service.PermanentlyDeleteAsset(ctx, userID.String(), assetID.String())
	require.NoError(t, err)

	// Verify asset is gone
	assetUUID := pgtype.UUID{Bytes: assetID, Valid: true}
	_, err = tdb.Queries.GetAsset(ctx, assetUUID)
	assert.Error(t, err)
}

func TestIntegration_PermanentlyDeleteAsset_NotOwned(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "permowner@test.com")
	user2ID := createTestUser(t, tdb, "permnotowner@test.com")

	// Create and trash asset owned by user1
	assetID := createTestAsset(t, tdb, user1ID, "permowned")
	err := service.TrashAsset(ctx, user1ID.String(), assetID.String())
	require.NoError(t, err)

	// User2 tries to permanently delete user1's asset
	err = service.PermanentlyDeleteAsset(ctx, user2ID.String(), assetID.String())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestIntegration_GetTrashedAssets_UserIsolation(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "trashuser1@test.com")
	user2ID := createTestUser(t, tdb, "trashuser2@test.com")

	// Create and trash assets for user1
	asset1ID := createTestAsset(t, tdb, user1ID, "user1asset1")
	asset2ID := createTestAsset(t, tdb, user1ID, "user1asset2")
	_, err := service.TrashAssets(ctx, user1ID.String(), []string{
		asset1ID.String(),
		asset2ID.String(),
	})
	require.NoError(t, err)

	// Create and trash asset for user2
	asset3ID := createTestAsset(t, tdb, user2ID, "user2asset1")
	err = service.TrashAsset(ctx, user2ID.String(), asset3ID.String())
	require.NoError(t, err)

	// User1 should only see their own trashed assets
	user1Trash, err := service.GetTrashedAssets(ctx, user1ID.String())
	require.NoError(t, err)
	assert.Len(t, user1Trash, 2)

	// User2 should only see their own trashed assets
	user2Trash, err := service.GetTrashedAssets(ctx, user2ID.String())
	require.NoError(t, err)
	assert.Len(t, user2Trash, 1)
}

func TestIntegration_EmptyTrash_NoTrashedAssets(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user with no trashed assets
	userID := createTestUser(t, tdb, "emptyuser@test.com")
	createTestAsset(t, tdb, userID, "nottrashed")

	// Empty trash should succeed with 0 deleted
	deleted, err := service.EmptyTrash(ctx, userID.String())
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

func TestIntegration_TrashAndRestoreCycle(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and asset
	userID := createTestUser(t, tdb, "cycle@test.com")
	assetID := createTestAsset(t, tdb, userID, "cycleasset")

	// Initial state: not in trash
	trashedAssets, err := service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Empty(t, trashedAssets)

	// Trash the asset
	err = service.TrashAsset(ctx, userID.String(), assetID.String())
	require.NoError(t, err)

	// Verify in trash
	trashedAssets, err = service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, trashedAssets, 1)

	// Restore the asset
	err = service.RestoreAsset(ctx, userID.String(), assetID.String())
	require.NoError(t, err)

	// Verify no longer in trash
	trashedAssets, err = service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Empty(t, trashedAssets)

	// Trash again
	err = service.TrashAsset(ctx, userID.String(), assetID.String())
	require.NoError(t, err)

	// Verify back in trash
	trashedAssets, err = service.GetTrashedAssets(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, trashedAssets, 1)
}
