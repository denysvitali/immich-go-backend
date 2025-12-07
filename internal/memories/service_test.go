// +build integration

package memories

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

func TestIntegration_CreateMemory(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "memory@test.com")

	// Create memory
	memory := &Memory{
		UserID:      userID.String(),
		Title:       "Summer Vacation",
		Description: "Photos from beach trip",
		Type:        "custom",
	}

	result, err := service.CreateMemory(ctx, memory)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, userID.String(), result.UserID)
	assert.Equal(t, "Summer Vacation", result.Title)
	assert.Equal(t, "Photos from beach trip", result.Description)
	assert.Equal(t, "custom", result.Type)
}

func TestIntegration_CreateMemory_InvalidUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Try to create memory with invalid user ID
	memory := &Memory{
		UserID: "not-a-valid-uuid",
		Title:  "Test Memory",
		Type:   "custom",
	}

	_, err := service.CreateMemory(ctx, memory)
	assert.Error(t, err)
}

func TestIntegration_GetMemory(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and memory
	userID := createTestUser(t, tdb, "getmemory@test.com")

	memory := &Memory{
		UserID:      userID.String(),
		Title:       "Birthday Party",
		Description: "Photos from the party",
		Type:        "on_this_day",
	}

	created, err := service.CreateMemory(ctx, memory)
	require.NoError(t, err)

	// Get the memory
	result, err := service.GetMemory(ctx, userID.String(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, result.ID)
	assert.Equal(t, "Birthday Party", result.Title)
	assert.Equal(t, "on_this_day", result.Type)
}

func TestIntegration_GetMemory_NotOwned(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "owner@test.com")
	user2ID := createTestUser(t, tdb, "notowner@test.com")

	// Create memory as user1
	memory := &Memory{
		UserID: user1ID.String(),
		Title:  "Private Memory",
		Type:   "custom",
	}

	created, err := service.CreateMemory(ctx, memory)
	require.NoError(t, err)

	// User2 tries to get user1's memory
	_, err = service.GetMemory(ctx, user2ID.String(), created.ID)
	assert.Error(t, err)
}

func TestIntegration_GetMemories(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "memories@test.com")

	// Create multiple memories
	for i := 0; i < 3; i++ {
		memory := &Memory{
			UserID: userID.String(),
			Title:  "Memory " + string(rune('A'+i)),
			Type:   "custom",
		}
		_, err := service.CreateMemory(ctx, memory)
		require.NoError(t, err)
	}

	// Get all memories
	memories, err := service.GetMemories(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, memories, 3)
}

func TestIntegration_GetMemories_UserIsolation(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "user1@test.com")
	user2ID := createTestUser(t, tdb, "user2@test.com")

	// Create memories for user1
	for i := 0; i < 2; i++ {
		memory := &Memory{
			UserID: user1ID.String(),
			Title:  "User1 Memory",
			Type:   "custom",
		}
		_, err := service.CreateMemory(ctx, memory)
		require.NoError(t, err)
	}

	// Create memory for user2
	memory := &Memory{
		UserID: user2ID.String(),
		Title:  "User2 Memory",
		Type:   "custom",
	}
	_, err := service.CreateMemory(ctx, memory)
	require.NoError(t, err)

	// User1 should only see their memories
	memories1, err := service.GetMemories(ctx, user1ID.String())
	require.NoError(t, err)
	assert.Len(t, memories1, 2)

	// User2 should only see their memories
	memories2, err := service.GetMemories(ctx, user2ID.String())
	require.NoError(t, err)
	assert.Len(t, memories2, 1)
}

func TestIntegration_UpdateMemory(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and memory
	userID := createTestUser(t, tdb, "update@test.com")

	memory := &Memory{
		UserID: userID.String(),
		Title:  "Original Title",
		Type:   "custom",
	}

	created, err := service.CreateMemory(ctx, memory)
	require.NoError(t, err)

	// Update the memory
	updates := map[string]interface{}{
		"title":       "Updated Title",
		"description": "New description",
	}

	updated, err := service.UpdateMemory(ctx, userID.String(), created.ID, updates)
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", updated.Title)
	assert.Equal(t, "New description", updated.Description)
}

func TestIntegration_UpdateMemory_NotOwned(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "updateowner@test.com")
	user2ID := createTestUser(t, tdb, "updatenotowner@test.com")

	// Create memory as user1
	memory := &Memory{
		UserID: user1ID.String(),
		Title:  "User1 Memory",
		Type:   "custom",
	}

	created, err := service.CreateMemory(ctx, memory)
	require.NoError(t, err)

	// User2 tries to update user1's memory
	updates := map[string]interface{}{
		"title": "Hijacked Title",
	}

	_, err = service.UpdateMemory(ctx, user2ID.String(), created.ID, updates)
	assert.Error(t, err)
}

func TestIntegration_DeleteMemory(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and memory
	userID := createTestUser(t, tdb, "delete@test.com")

	memory := &Memory{
		UserID: userID.String(),
		Title:  "To Be Deleted",
		Type:   "custom",
	}

	created, err := service.CreateMemory(ctx, memory)
	require.NoError(t, err)

	// Delete the memory
	err = service.DeleteMemory(ctx, userID.String(), created.ID)
	require.NoError(t, err)

	// Verify memory is deleted (GetMemory should fail)
	_, err = service.GetMemory(ctx, userID.String(), created.ID)
	assert.Error(t, err)
}

func TestIntegration_DeleteMemory_NotOwned(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "deleteowner@test.com")
	user2ID := createTestUser(t, tdb, "deletenotowner@test.com")

	// Create memory as user1
	memory := &Memory{
		UserID: user1ID.String(),
		Title:  "Protected Memory",
		Type:   "custom",
	}

	created, err := service.CreateMemory(ctx, memory)
	require.NoError(t, err)

	// User2 tries to delete user1's memory
	err = service.DeleteMemory(ctx, user2ID.String(), created.ID)
	assert.Error(t, err)
}

func TestIntegration_AddAssetsToMemory(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user, memory, and assets
	userID := createTestUser(t, tdb, "addassets@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "asset1")
	asset2ID := createTestAsset(t, tdb, userID, "asset2")

	memory := &Memory{
		UserID: userID.String(),
		Title:  "Memory With Assets",
		Type:   "custom",
	}

	created, err := service.CreateMemory(ctx, memory)
	require.NoError(t, err)

	// Add assets to memory
	err = service.AddAssetsToMemory(ctx, userID.String(), created.ID, []string{
		asset1ID.String(),
		asset2ID.String(),
	})
	require.NoError(t, err)

	// Verify assets are associated
	assetIDs, err := service.GetMemoryAssets(ctx, userID.String(), created.ID)
	require.NoError(t, err)
	assert.Len(t, assetIDs, 2)
}

func TestIntegration_AddAssetsToMemory_NotOwned(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "assetsowner@test.com")
	user2ID := createTestUser(t, tdb, "assetsnotowner@test.com")
	assetID := createTestAsset(t, tdb, user1ID, "asset")

	// Create memory as user1
	memory := &Memory{
		UserID: user1ID.String(),
		Title:  "Protected Memory",
		Type:   "custom",
	}

	created, err := service.CreateMemory(ctx, memory)
	require.NoError(t, err)

	// User2 tries to add assets to user1's memory
	err = service.AddAssetsToMemory(ctx, user2ID.String(), created.ID, []string{assetID.String()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestIntegration_RemoveAssetsFromMemory(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user, memory, and assets
	userID := createTestUser(t, tdb, "removeassets@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "asset1")
	asset2ID := createTestAsset(t, tdb, userID, "asset2")

	memory := &Memory{
		UserID: userID.String(),
		Title:  "Memory With Assets",
		Type:   "custom",
	}

	created, err := service.CreateMemory(ctx, memory)
	require.NoError(t, err)

	// Add assets to memory
	err = service.AddAssetsToMemory(ctx, userID.String(), created.ID, []string{
		asset1ID.String(),
		asset2ID.String(),
	})
	require.NoError(t, err)

	// Verify 2 assets
	assetIDs, err := service.GetMemoryAssets(ctx, userID.String(), created.ID)
	require.NoError(t, err)
	assert.Len(t, assetIDs, 2)

	// Remove one asset
	err = service.RemoveAssetsFromMemory(ctx, userID.String(), created.ID, []string{asset1ID.String()})
	require.NoError(t, err)

	// Verify only 1 asset remains
	assetIDs, err = service.GetMemoryAssets(ctx, userID.String(), created.ID)
	require.NoError(t, err)
	assert.Len(t, assetIDs, 1)
}

func TestIntegration_GetMemoryAssets_NotOwned(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "getassetsowner@test.com")
	user2ID := createTestUser(t, tdb, "getassetsnotowner@test.com")

	// Create memory as user1
	memory := &Memory{
		UserID: user1ID.String(),
		Title:  "Protected Memory",
		Type:   "custom",
	}

	created, err := service.CreateMemory(ctx, memory)
	require.NoError(t, err)

	// User2 tries to get assets from user1's memory
	_, err = service.GetMemoryAssets(ctx, user2ID.String(), created.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestIntegration_MemoryTypes(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "types@test.com")

	// Test different memory types
	types := []string{"on_this_day", "year_ago", "custom"}

	for _, memType := range types {
		memory := &Memory{
			UserID: userID.String(),
			Title:  "Memory of type " + memType,
			Type:   memType,
		}

		created, err := service.CreateMemory(ctx, memory)
		require.NoError(t, err)
		assert.Equal(t, memType, created.Type)

		// Verify on get
		retrieved, err := service.GetMemory(ctx, userID.String(), created.ID)
		require.NoError(t, err)
		assert.Equal(t, memType, retrieved.Type)
	}
}

func TestIntegration_InvalidMemoryID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	userID := createTestUser(t, tdb, "invalid@test.com")

	// Test with invalid memory ID
	_, err := service.GetMemory(ctx, userID.String(), "not-a-valid-uuid")
	assert.Error(t, err)

	err = service.DeleteMemory(ctx, userID.String(), "not-a-valid-uuid")
	assert.Error(t, err)

	err = service.AddAssetsToMemory(ctx, userID.String(), "not-a-valid-uuid", []string{uuid.New().String()})
	assert.Error(t, err)

	err = service.RemoveAssetsFromMemory(ctx, userID.String(), "not-a-valid-uuid", []string{uuid.New().String()})
	assert.Error(t, err)

	_, err = service.GetMemoryAssets(ctx, userID.String(), "not-a-valid-uuid")
	assert.Error(t, err)
}
