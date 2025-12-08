//go:build integration
// +build integration

package stacks

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/config"
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

func TestIntegration_CreateStack(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user and assets
	userID := createTestUser(t, tdb, "stacktest@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "asset1")
	asset2ID := createTestAsset(t, tdb, userID, "asset2")
	asset3ID := createTestAsset(t, tdb, userID, "asset3")

	// Create a stack with the assets
	response, err := service.CreateStack(ctx, CreateStackRequest{
		AssetIDs: []string{
			asset1ID.String(),
			asset2ID.String(),
			asset3ID.String(),
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotEmpty(t, response.ID)
	assert.Equal(t, asset1ID.String(), response.PrimaryAssetID) // First asset is primary
	assert.Equal(t, int32(3), response.AssetCount)
	assert.Len(t, response.AssetIDs, 3)
}

func TestIntegration_CreateStackEmptyAssets(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Try to create a stack with no assets
	response, err := service.CreateStack(ctx, CreateStackRequest{
		AssetIDs: []string{},
	})
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "at least one asset ID is required")
}

func TestIntegration_GetStack(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user and assets
	userID := createTestUser(t, tdb, "getstack@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "getasset1")
	asset2ID := createTestAsset(t, tdb, userID, "getasset2")

	// Create a stack
	createResponse, err := service.CreateStack(ctx, CreateStackRequest{
		AssetIDs: []string{asset1ID.String(), asset2ID.String()},
	})
	require.NoError(t, err)

	// Get the stack
	getResponse, err := service.GetStack(ctx, createResponse.ID)
	require.NoError(t, err)
	assert.NotNil(t, getResponse)
	assert.Equal(t, createResponse.ID, getResponse.ID)
	assert.Equal(t, asset1ID.String(), getResponse.PrimaryAssetID)
	assert.Equal(t, int32(2), getResponse.AssetCount)
}

func TestIntegration_GetStackNotFound(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Try to get a non-existent stack
	randomID := uuid.New().String()
	response, err := service.GetStack(ctx, randomID)
	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestIntegration_UpdateStackPrimaryAsset(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user and assets
	userID := createTestUser(t, tdb, "updatestack@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "updateasset1")
	asset2ID := createTestAsset(t, tdb, userID, "updateasset2")

	// Create a stack
	createResponse, err := service.CreateStack(ctx, CreateStackRequest{
		AssetIDs: []string{asset1ID.String(), asset2ID.String()},
	})
	require.NoError(t, err)
	assert.Equal(t, asset1ID.String(), createResponse.PrimaryAssetID)

	// Update primary asset to asset2
	asset2IDStr := asset2ID.String()
	updateResponse, err := service.UpdateStack(ctx, createResponse.ID, UpdateStackRequest{
		PrimaryAssetID: &asset2IDStr,
	})
	require.NoError(t, err)
	assert.Equal(t, asset2ID.String(), updateResponse.PrimaryAssetID)
}

func TestIntegration_DeleteStack(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user and assets
	userID := createTestUser(t, tdb, "deletestack@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "deleteasset1")
	asset2ID := createTestAsset(t, tdb, userID, "deleteasset2")

	// Create a stack
	createResponse, err := service.CreateStack(ctx, CreateStackRequest{
		AssetIDs: []string{asset1ID.String(), asset2ID.String()},
	})
	require.NoError(t, err)

	// Delete the stack
	err = service.DeleteStack(ctx, createResponse.ID)
	require.NoError(t, err)

	// Verify stack is deleted
	getResponse, err := service.GetStack(ctx, createResponse.ID)
	assert.Error(t, err)
	assert.Nil(t, getResponse)
}

func TestIntegration_DeleteStacks(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user and assets
	userID := createTestUser(t, tdb, "deletestacks@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "bulkasset1")
	asset2ID := createTestAsset(t, tdb, userID, "bulkasset2")
	asset3ID := createTestAsset(t, tdb, userID, "bulkasset3")
	asset4ID := createTestAsset(t, tdb, userID, "bulkasset4")

	// Create two stacks
	stack1, err := service.CreateStack(ctx, CreateStackRequest{
		AssetIDs: []string{asset1ID.String(), asset2ID.String()},
	})
	require.NoError(t, err)

	stack2, err := service.CreateStack(ctx, CreateStackRequest{
		AssetIDs: []string{asset3ID.String(), asset4ID.String()},
	})
	require.NoError(t, err)

	// Delete both stacks
	err = service.DeleteStacks(ctx, []string{stack1.ID, stack2.ID})
	require.NoError(t, err)

	// Verify both stacks are deleted
	_, err = service.GetStack(ctx, stack1.ID)
	assert.Error(t, err)

	_, err = service.GetStack(ctx, stack2.ID)
	assert.Error(t, err)
}

func TestIntegration_GetUserStacks(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user and assets
	userID := createTestUser(t, tdb, "userstacks@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "userasset1")
	asset2ID := createTestAsset(t, tdb, userID, "userasset2")
	asset3ID := createTestAsset(t, tdb, userID, "userasset3")
	asset4ID := createTestAsset(t, tdb, userID, "userasset4")

	// Create two stacks for the user
	_, err = service.CreateStack(ctx, CreateStackRequest{
		AssetIDs: []string{asset1ID.String(), asset2ID.String()},
	})
	require.NoError(t, err)

	_, err = service.CreateStack(ctx, CreateStackRequest{
		AssetIDs: []string{asset3ID.String(), asset4ID.String()},
	})
	require.NoError(t, err)

	// Get user's stacks
	response, err := service.GetUserStacks(ctx, userID.String(), 10, 0)
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Stacks, 2)
}

func TestIntegration_AddAssetsToStack(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user and assets
	userID := createTestUser(t, tdb, "addassets@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "addasset1")
	asset2ID := createTestAsset(t, tdb, userID, "addasset2")
	asset3ID := createTestAsset(t, tdb, userID, "addasset3")

	// Create a stack with 2 assets
	createResponse, err := service.CreateStack(ctx, CreateStackRequest{
		AssetIDs: []string{asset1ID.String(), asset2ID.String()},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), createResponse.AssetCount)

	// Add a third asset
	err = service.AddAssetsToStack(ctx, createResponse.ID, []string{asset3ID.String()})
	require.NoError(t, err)

	// Verify stack now has 3 assets
	getResponse, err := service.GetStack(ctx, createResponse.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(3), getResponse.AssetCount)
}

func TestIntegration_RemoveAssetsFromStack(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user and assets
	userID := createTestUser(t, tdb, "removeassets@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "removeasset1")
	asset2ID := createTestAsset(t, tdb, userID, "removeasset2")
	asset3ID := createTestAsset(t, tdb, userID, "removeasset3")

	// Create a stack with 3 assets
	createResponse, err := service.CreateStack(ctx, CreateStackRequest{
		AssetIDs: []string{asset1ID.String(), asset2ID.String(), asset3ID.String()},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(3), createResponse.AssetCount)

	// Remove one asset
	err = service.RemoveAssetsFromStack(ctx, []string{asset3ID.String()})
	require.NoError(t, err)

	// Verify stack now has 2 assets
	getResponse, err := service.GetStack(ctx, createResponse.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(2), getResponse.AssetCount)
}

func TestIntegration_SearchStacks(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create test user and assets
	userID := createTestUser(t, tdb, "searchstacks@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "searchasset1")
	asset2ID := createTestAsset(t, tdb, userID, "searchasset2")

	// Create a stack
	createResponse, err := service.CreateStack(ctx, CreateStackRequest{
		AssetIDs: []string{asset1ID.String(), asset2ID.String()},
	})
	require.NoError(t, err)

	// Search stacks by user
	userIDStr := userID.String()
	searchResponse, err := service.SearchStacks(ctx, SearchStacksRequest{
		UserID: &userIDStr,
	})
	require.NoError(t, err)
	assert.NotNil(t, searchResponse)
	assert.Len(t, searchResponse.Stacks, 1)
	assert.Equal(t, createResponse.ID, searchResponse.Stacks[0].ID)

	// Search stacks by primary asset
	primaryAssetID := asset1ID.String()
	searchResponse, err = service.SearchStacks(ctx, SearchStacksRequest{
		UserID:         &userIDStr,
		PrimaryAssetID: &primaryAssetID,
	})
	require.NoError(t, err)
	assert.NotNil(t, searchResponse)
	assert.Len(t, searchResponse.Stacks, 1)
}

func TestIntegration_InvalidUUIDs(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Test invalid stack ID
	_, err = service.GetStack(ctx, "not-a-valid-uuid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid stack ID")

	// Test invalid asset IDs
	_, err = service.CreateStack(ctx, CreateStackRequest{
		AssetIDs: []string{"not-a-valid-uuid"},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid asset IDs")

	// Test invalid user ID in search
	invalidUserID := "not-a-valid-uuid"
	_, err = service.SearchStacks(ctx, SearchStacksRequest{
		UserID: &invalidUserID,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid user ID")
}
