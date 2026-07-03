//go:build integration
// +build integration

package duplicates

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestUser(t *testing.T, tdb *testdb.TestDB, email string) uuid.UUID {
	return tdb.CreateTestUser(t, email)
}

// createTestAssetWithChecksum creates a test asset with a specific checksum
func createTestAssetWithChecksum(t *testing.T, tdb *testdb.TestDB, ownerID uuid.UUID, deviceAssetID string, checksum []byte) uuid.UUID {
	return tdb.CreateTestAssetWithChecksum(t, ownerID, deviceAssetID, checksum)
}

// createTestAssetWithExif creates a test asset with EXIF data including file size
func createTestAssetWithExif(t *testing.T, tdb *testdb.TestDB, ownerID uuid.UUID, deviceAssetID string, checksum []byte, fileSize int64) uuid.UUID {
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
		Checksum:         checksum,
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err)

	// Create EXIF data with file size
	_, err = tdb.Queries.CreateExif(ctx, sqlc.CreateExifParams{
		AssetId:        asset.ID,
		FileSizeInByte: pgtype.Int8{Int64: fileSize, Valid: true},
	})
	require.NoError(t, err)

	return asset.ID.Bytes
}

func TestIntegration_GetAssetDuplicates_NoDuplicates(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create user with unique assets (different checksums)
	userID := createTestUser(t, tdb, "noduplicates@test.com")
	createTestAssetWithChecksum(t, tdb, userID, "unique1", []byte("checksum1"))
	createTestAssetWithChecksum(t, tdb, userID, "unique2", []byte("checksum2"))
	createTestAssetWithChecksum(t, tdb, userID, "unique3", []byte("checksum3"))

	// Get duplicates - should be empty
	response, err := service.GetAssetDuplicates(ctx, userID.String())
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Empty(t, response.Duplicates)
}

func TestIntegration_GetAssetDuplicates_WithDuplicates(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create user with duplicate assets (same checksum)
	userID := createTestUser(t, tdb, "duplicates@test.com")
	sharedChecksum := []byte("duplicate-checksum-123")

	createTestAssetWithChecksum(t, tdb, userID, "dup1", sharedChecksum)
	createTestAssetWithChecksum(t, tdb, userID, "dup2", sharedChecksum)
	createTestAssetWithChecksum(t, tdb, userID, "unique", []byte("unique-checksum"))

	// Get duplicates
	response, err := service.GetAssetDuplicates(ctx, userID.String())
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Duplicates, 1)
	assert.Len(t, response.Duplicates[0].Assets, 2)
	assert.Equal(t, hex.EncodeToString(sharedChecksum), response.Duplicates[0].DuplicateID)
}

func TestIntegration_GetAssetDuplicates_MultipleDuplicateGroups(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create user with multiple duplicate groups
	userID := createTestUser(t, tdb, "multidup@test.com")

	// First duplicate group
	checksum1 := []byte("group1-checksum")
	createTestAssetWithChecksum(t, tdb, userID, "group1a", checksum1)
	createTestAssetWithChecksum(t, tdb, userID, "group1b", checksum1)
	createTestAssetWithChecksum(t, tdb, userID, "group1c", checksum1)

	// Second duplicate group
	checksum2 := []byte("group2-checksum")
	createTestAssetWithChecksum(t, tdb, userID, "group2a", checksum2)
	createTestAssetWithChecksum(t, tdb, userID, "group2b", checksum2)

	// Get duplicates
	response, err := service.GetAssetDuplicates(ctx, userID.String())
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Duplicates, 2)

	// Check group sizes
	groupSizes := make(map[int]int)
	for _, group := range response.Duplicates {
		groupSizes[len(group.Assets)]++
	}
	assert.Equal(t, 1, groupSizes[3]) // One group with 3 assets
	assert.Equal(t, 1, groupSizes[2]) // One group with 2 assets
}

func TestIntegration_GetAssetDuplicates_InvalidUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Try with invalid user ID
	response, err := service.GetAssetDuplicates(ctx, "not-a-valid-uuid")
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "invalid user ID")
}

func TestIntegration_GetAssetDuplicates_UserIsolation(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create two users
	user1ID := createTestUser(t, tdb, "user1@test.com")
	user2ID := createTestUser(t, tdb, "user2@test.com")

	// Both users have assets with the same checksum
	sharedChecksum := []byte("shared-across-users")
	createTestAssetWithChecksum(t, tdb, user1ID, "user1asset", sharedChecksum)
	createTestAssetWithChecksum(t, tdb, user2ID, "user2asset", sharedChecksum)

	// User1's duplicates should only show their own assets
	response1, err := service.GetAssetDuplicates(ctx, user1ID.String())
	require.NoError(t, err)
	assert.Empty(t, response1.Duplicates) // Only 1 asset per user, so no duplicates

	// User2's duplicates should only show their own assets
	response2, err := service.GetAssetDuplicates(ctx, user2ID.String())
	require.NoError(t, err)
	assert.Empty(t, response2.Duplicates) // Only 1 asset per user, so no duplicates
}

func TestIntegration_FindDuplicatesByChecksum(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create user with duplicate assets
	userID := createTestUser(t, tdb, "findchecksum@test.com")
	checksum := []byte("find-by-checksum-test")
	checksumHex := hex.EncodeToString(checksum)

	createTestAssetWithChecksum(t, tdb, userID, "find1", checksum)
	createTestAssetWithChecksum(t, tdb, userID, "find2", checksum)
	createTestAssetWithChecksum(t, tdb, userID, "other", []byte("different-checksum"))

	// Find duplicates by checksum
	duplicates, err := service.FindDuplicatesByChecksum(ctx, userID.String(), checksumHex)
	require.NoError(t, err)
	assert.Len(t, duplicates, 2)
	for _, dup := range duplicates {
		assert.Equal(t, checksumHex, dup.Checksum)
	}
}

func TestIntegration_FindDuplicatesByChecksum_NotFound(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create user with assets
	userID := createTestUser(t, tdb, "notfound@test.com")
	createTestAssetWithChecksum(t, tdb, userID, "asset1", []byte("some-checksum"))

	// Search for non-existent checksum
	nonExistentChecksum := hex.EncodeToString([]byte("non-existent-checksum"))
	duplicates, err := service.FindDuplicatesByChecksum(ctx, userID.String(), nonExistentChecksum)
	require.NoError(t, err)
	assert.Empty(t, duplicates)
}

func TestIntegration_FindDuplicatesByChecksum_InvalidChecksum(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	userID := createTestUser(t, tdb, "invalidchecksum@test.com")

	// Try with invalid hex checksum
	duplicates, err := service.FindDuplicatesByChecksum(ctx, userID.String(), "not-valid-hex!")
	assert.Error(t, err)
	assert.Nil(t, duplicates)
	assert.Contains(t, err.Error(), "invalid checksum format")
}

func TestIntegration_FindDuplicatesByChecksum_UserIsolation(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create two users with same checksum
	user1ID := createTestUser(t, tdb, "checksumuser1@test.com")
	user2ID := createTestUser(t, tdb, "checksumuser2@test.com")

	sharedChecksum := []byte("shared-checksum-isolation")
	checksumHex := hex.EncodeToString(sharedChecksum)

	createTestAssetWithChecksum(t, tdb, user1ID, "user1asset", sharedChecksum)
	createTestAssetWithChecksum(t, tdb, user2ID, "user2asset", sharedChecksum)

	// User1 should only find their own asset
	duplicates1, err := service.FindDuplicatesByChecksum(ctx, user1ID.String(), checksumHex)
	require.NoError(t, err)
	assert.Len(t, duplicates1, 1)

	// User2 should only find their own asset
	duplicates2, err := service.FindDuplicatesByChecksum(ctx, user2ID.String(), checksumHex)
	require.NoError(t, err)
	assert.Len(t, duplicates2, 1)
}

func TestIntegration_FindDuplicatesBySize(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create user with assets of same size
	userID := createTestUser(t, tdb, "findsize@test.com")
	targetSize := int64(1024 * 1024) // 1 MB

	createTestAssetWithExif(t, tdb, userID, "size1", []byte("checksum1"), targetSize)
	createTestAssetWithExif(t, tdb, userID, "size2", []byte("checksum2"), targetSize)
	createTestAssetWithExif(t, tdb, userID, "size3", []byte("checksum3"), 2*1024*1024) // 2 MB

	// Find duplicates by size
	duplicates, err := service.FindDuplicatesBySize(ctx, userID.String(), targetSize)
	require.NoError(t, err)
	assert.Len(t, duplicates, 2)
	for _, dup := range duplicates {
		assert.Equal(t, targetSize, dup.FileSizeInByte)
	}
}

func TestIntegration_FindDuplicatesBySize_NotFound(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create user with assets
	userID := createTestUser(t, tdb, "sizenotfound@test.com")
	createTestAssetWithExif(t, tdb, userID, "asset1", []byte("checksum1"), 1024)

	// Search for non-existent size
	duplicates, err := service.FindDuplicatesBySize(ctx, userID.String(), 999999999)
	require.NoError(t, err)
	assert.Empty(t, duplicates)
}

func TestIntegration_FindDuplicatesBySize_InvalidUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Try with invalid user ID
	duplicates, err := service.FindDuplicatesBySize(ctx, "invalid-uuid", 1024)
	assert.Error(t, err)
	assert.Nil(t, duplicates)
	assert.Contains(t, err.Error(), "invalid user ID")
}

// TestIntegration_FindDuplicatesBySize_RespectsUserOwnership verifies the
// server-side WHERE "ownerId" = $1 clause. Two users with assets of the
// same file size must each only see their own assets.
func TestIntegration_FindDuplicatesBySize_RespectsUserOwnership(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	userA := createTestUser(t, tdb, "owner-a@test.com")
	userB := createTestUser(t, tdb, "owner-b@test.com")
	sharedSize := int64(2048)

	// Each user has 2 assets at sharedSize plus 1 asset at a different size.
	createTestAssetWithExif(t, tdb, userA, "a1", []byte("a-checksum-1"), sharedSize)
	createTestAssetWithExif(t, tdb, userA, "a2", []byte("a-checksum-2"), sharedSize)
	createTestAssetWithExif(t, tdb, userA, "a3", []byte("a-checksum-3"), sharedSize+1)
	createTestAssetWithExif(t, tdb, userB, "b1", []byte("b-checksum-1"), sharedSize)
	createTestAssetWithExif(t, tdb, userB, "b2", []byte("b-checksum-2"), sharedSize)

	dupsA, err := service.FindDuplicatesBySize(ctx, userA.String(), sharedSize)
	require.NoError(t, err)
	assert.Len(t, dupsA, 2, "userA should see exactly 2 duplicates at sharedSize")

	dupsB, err := service.FindDuplicatesBySize(ctx, userB.String(), sharedSize)
	require.NoError(t, err)
	assert.Len(t, dupsB, 2, "userB should see exactly 2 duplicates at sharedSize")

	// Cross-leak check: userA's results must not include any of userB's
	// device-asset ids, and vice versa.
	userBDeviceIDs := map[string]struct{}{"b1": {}, "b2": {}}
	for _, d := range dupsA {
		_, leaked := userBDeviceIDs[d.DeviceAssetID]
		assert.False(t, leaked, "userA's response leaked userB's asset %q", d.DeviceAssetID)
	}
	userADeviceIDs := map[string]struct{}{"a1": {}, "a2": {}}
	for _, d := range dupsB {
		_, leaked := userADeviceIDs[d.DeviceAssetID]
		assert.False(t, leaked, "userB's response leaked userA's asset %q", d.DeviceAssetID)
	}
}

// TestIntegration_FindDuplicatesBySize_ExcludesSoftDeleted verifies the
// server-side AND a."deletedAt" IS NULL filter. A soft-deleted asset at
// the target size must NOT appear in the results.
func TestIntegration_FindDuplicatesBySize_ExcludesSoftDeleted(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	userID := createTestUser(t, tdb, "deleted-asset@test.com")
	targetSize := int64(4096)

	liveID := createTestAssetWithExif(t, tdb, userID, "live-asset", []byte("live-checksum"), targetSize)
	createTestAssetWithExif(t, tdb, userID, "deleted-asset", []byte("deleted-checksum"), targetSize)

	// Soft-delete the live asset (force=true sets deletedAt = now()).
	// After this, both seeded assets should be excluded from the result:
	// the "deleted-asset" because it was hard-deleted later, and the
	// "live-asset" because the WHERE a."deletedAt" IS NULL filter drops it.
	_ = tdb.Queries.DeleteAssets(ctx, sqlc.DeleteAssetsParams{
		Column1: []pgtype.UUID{{Bytes: liveID, Valid: true}},
		Column2: true, // true -> status='deleted', deletedAt=now()
	})
	require.NoError(t, err)

	duplicates, err := service.FindDuplicatesBySize(ctx, userID.String(), targetSize)
	require.NoError(t, err)
	assert.Empty(t, duplicates, "soft-deleted assets must be excluded")
}

// mustUUID converts a [16]byte into a uuid.UUID; fails the test if it
func TestIntegration_AssetTypeConversion(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create user with different asset types
	userID := createTestUser(t, tdb, "assettype@test.com")
	checksum := []byte("type-test-checksum")

	ownerUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Create IMAGE asset
	_, err = tdb.Queries.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    "image1",
		OwnerId:          ownerUUID,
		DeviceId:         "test-device",
		Type:             "IMAGE",
		OriginalPath:     "/test/path/image1.jpg",
		OriginalFileName: "image1.jpg",
		Checksum:         checksum,
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err)

	// Create VIDEO asset with same checksum
	_, err = tdb.Queries.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    "video1",
		OwnerId:          ownerUUID,
		DeviceId:         "test-device",
		Type:             "VIDEO",
		OriginalPath:     "/test/path/video1.mp4",
		OriginalFileName: "video1.mp4",
		Checksum:         checksum,
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err)

	// Get duplicates
	response, err := service.GetAssetDuplicates(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, response.Duplicates, 1)
	assert.Len(t, response.Duplicates[0].Assets, 2)

	// Verify asset types are correctly converted
	typeCount := make(map[AssetType]int)
	for _, asset := range response.Duplicates[0].Assets {
		typeCount[asset.Type]++
	}
	assert.Equal(t, 1, typeCount[AssetType_IMAGE])
	assert.Equal(t, 1, typeCount[AssetType_VIDEO])
}
