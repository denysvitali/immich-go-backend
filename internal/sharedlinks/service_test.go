//go:build integration
// +build integration

package sharedlinks

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

func createTestUser(t *testing.T, tdb *testdb.TestDB, email string) uuid.UUID {
	return tdb.CreateTestUser(t, email)
}

func createTestAsset(t *testing.T, tdb *testdb.TestDB, ownerID uuid.UUID, deviceAssetID string) uuid.UUID {
	return tdb.CreateTestAsset(t, ownerID, deviceAssetID)
}

func TestIntegration_CreateSharedLink(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and asset
	userID := createTestUser(t, tdb, "sharedlink@test.com")
	assetID := createTestAsset(t, tdb, userID, "sharedasset1")

	// Create a shared link
	link, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:          "INDIVIDUAL",
		AssetIDs:      []string{assetID.String()},
		Description:   "Test shared link",
		AllowDownload: true,
		AllowUpload:   false,
		ShowExif:      true,
	})
	require.NoError(t, err)
	assert.NotNil(t, link)
	assert.NotEmpty(t, link.ID)
	assert.NotEmpty(t, link.Key)
	assert.Equal(t, userID, link.UserID)
	assert.Equal(t, "INDIVIDUAL", link.Type)
	assert.Equal(t, "Test shared link", link.Description)
	assert.True(t, link.AllowDownload)
	assert.False(t, link.AllowUpload)
	assert.True(t, link.ShowExif)
	assert.Equal(t, 1, link.AssetCount)
}

func TestIntegration_CreateSharedLinkWithPassword(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and asset
	userID := createTestUser(t, tdb, "pwdlink@test.com")
	assetID := createTestAsset(t, tdb, userID, "pwdasset1")

	// Create a password-protected shared link
	link, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:          "INDIVIDUAL",
		AssetIDs:      []string{assetID.String()},
		Password:      "secretpassword",
		AllowDownload: true,
	})
	require.NoError(t, err)
	assert.NotNil(t, link)
	assert.Equal(t, "[PROTECTED]", link.Password) // Password should be masked
}

func TestIntegration_CreateSharedLinkWithExpiry(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and asset
	userID := createTestUser(t, tdb, "expirylink@test.com")
	assetID := createTestAsset(t, tdb, userID, "expiryasset1")

	// Create a shared link with expiry
	expiresAt := time.Now().Add(24 * time.Hour)
	link, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:      "INDIVIDUAL",
		AssetIDs:  []string{assetID.String()},
		ExpiresAt: &expiresAt,
	})
	require.NoError(t, err)
	assert.NotNil(t, link)
	assert.NotNil(t, link.ExpiresAt)
	assert.True(t, link.ExpiresAt.After(time.Now()))
}

func TestIntegration_GetSharedLink(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and asset
	userID := createTestUser(t, tdb, "getlink@test.com")
	assetID := createTestAsset(t, tdb, userID, "getlinkasset")

	// Create a shared link
	createLink, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:        "INDIVIDUAL",
		AssetIDs:    []string{assetID.String()},
		Description: "Get test link",
	})
	require.NoError(t, err)

	// Get the shared link
	getLink, err := service.GetSharedLink(ctx, userID, createLink.ID)
	require.NoError(t, err)
	assert.Equal(t, createLink.ID, getLink.ID)
	assert.Equal(t, "Get test link", getLink.Description)
}

func TestIntegration_GetSharedLinkAccessDenied(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "user1@test.com")
	user2ID := createTestUser(t, tdb, "user2@test.com")
	assetID := createTestAsset(t, tdb, user1ID, "user1asset")

	// Create a shared link as user1
	link, err := service.CreateSharedLink(ctx, user1ID, &CreateSharedLinkRequest{
		Type:     "INDIVIDUAL",
		AssetIDs: []string{assetID.String()},
	})
	require.NoError(t, err)

	// Try to get the link as user2
	_, err = service.GetSharedLink(ctx, user2ID, link.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestIntegration_GetSharedLinkByKey(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and asset
	userID := createTestUser(t, tdb, "keylink@test.com")
	assetID := createTestAsset(t, tdb, userID, "keylinkasset")

	// Create a shared link
	createLink, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:     "INDIVIDUAL",
		AssetIDs: []string{assetID.String()},
	})
	require.NoError(t, err)

	// Get the link by key (no password needed)
	getLink, err := service.GetSharedLinkByKey(ctx, createLink.Key, "")
	require.NoError(t, err)
	assert.Equal(t, createLink.ID, getLink.ID)
}

func TestIntegration_GetSharedLinkByKeyWithPassword(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and asset
	userID := createTestUser(t, tdb, "keypwd@test.com")
	assetID := createTestAsset(t, tdb, userID, "keypwdasset")

	password := "secretpassword123"

	// Create a password-protected shared link
	createLink, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:     "INDIVIDUAL",
		AssetIDs: []string{assetID.String()},
		Password: password,
	})
	require.NoError(t, err)

	// Try to get without password
	_, err = service.GetSharedLinkByKey(ctx, createLink.Key, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password required")

	// Try with wrong password
	_, err = service.GetSharedLinkByKey(ctx, createLink.Key, "wrongpassword")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid password")

	// Get with correct password
	getLink, err := service.GetSharedLinkByKey(ctx, createLink.Key, password)
	require.NoError(t, err)
	assert.Equal(t, createLink.ID, getLink.ID)
}

func TestIntegration_GetSharedLinkByKeyExpired(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and asset
	userID := createTestUser(t, tdb, "expired@test.com")
	assetID := createTestAsset(t, tdb, userID, "expiredasset")

	// Create a shared link that has already expired
	expiresAt := time.Now().Add(-1 * time.Hour) // Expired 1 hour ago
	createLink, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:      "INDIVIDUAL",
		AssetIDs:  []string{assetID.String()},
		ExpiresAt: &expiresAt,
	})
	require.NoError(t, err)

	// Try to get the expired link
	_, err = service.GetSharedLinkByKey(ctx, createLink.Key, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestIntegration_GetSharedLinks(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and assets
	userID := createTestUser(t, tdb, "listlinks@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "listasset1")
	asset2ID := createTestAsset(t, tdb, userID, "listasset2")

	// Create multiple shared links
	_, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:        "INDIVIDUAL",
		AssetIDs:    []string{asset1ID.String()},
		Description: "Link 1",
	})
	require.NoError(t, err)

	_, err = service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:        "INDIVIDUAL",
		AssetIDs:    []string{asset2ID.String()},
		Description: "Link 2",
	})
	require.NoError(t, err)

	// Get all shared links
	links, err := service.GetSharedLinks(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, links, 2)
}

func TestIntegration_UpdateSharedLink(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and asset
	userID := createTestUser(t, tdb, "updatelink@test.com")
	assetID := createTestAsset(t, tdb, userID, "updateasset")

	// Create a shared link
	createLink, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:          "INDIVIDUAL",
		AssetIDs:      []string{assetID.String()},
		Description:   "Original description",
		AllowDownload: false,
		ShowExif:      false,
	})
	require.NoError(t, err)

	// Update the link
	newDesc := "Updated description"
	allowDownload := true
	showExif := true
	updateLink, err := service.UpdateSharedLink(ctx, userID, createLink.ID, &UpdateSharedLinkRequest{
		Description:   &newDesc,
		AllowDownload: &allowDownload,
		ShowExif:      &showExif,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated description", updateLink.Description)
	assert.True(t, updateLink.AllowDownload)
	assert.True(t, updateLink.ShowExif)
}

func TestIntegration_UpdateSharedLinkPassword(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and asset
	userID := createTestUser(t, tdb, "updatepwd@test.com")
	assetID := createTestAsset(t, tdb, userID, "updatepwdasset")

	// Create a shared link without password
	createLink, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:     "INDIVIDUAL",
		AssetIDs: []string{assetID.String()},
	})
	require.NoError(t, err)

	// Add password
	newPassword := "newpassword123"
	_, err = service.UpdateSharedLink(ctx, userID, createLink.ID, &UpdateSharedLinkRequest{
		Password: &newPassword,
	})
	require.NoError(t, err)

	// Verify password is now required
	_, err = service.GetSharedLinkByKey(ctx, createLink.Key, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "password required")

	// Verify correct password works
	_, err = service.GetSharedLinkByKey(ctx, createLink.Key, newPassword)
	require.NoError(t, err)
}

func TestIntegration_DeleteSharedLink(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and asset
	userID := createTestUser(t, tdb, "deletelink@test.com")
	assetID := createTestAsset(t, tdb, userID, "deleteasset")

	// Create a shared link
	createLink, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:     "INDIVIDUAL",
		AssetIDs: []string{assetID.String()},
	})
	require.NoError(t, err)

	// Delete the link
	err = service.DeleteSharedLink(ctx, userID, createLink.ID)
	require.NoError(t, err)

	// Verify link is deleted
	_, err = service.GetSharedLink(ctx, userID, createLink.ID)
	assert.Error(t, err)
}

func TestIntegration_DeleteSharedLinkAccessDenied(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "deluser1@test.com")
	user2ID := createTestUser(t, tdb, "deluser2@test.com")
	assetID := createTestAsset(t, tdb, user1ID, "deluser1asset")

	// Create a shared link as user1
	link, err := service.CreateSharedLink(ctx, user1ID, &CreateSharedLinkRequest{
		Type:     "INDIVIDUAL",
		AssetIDs: []string{assetID.String()},
	})
	require.NoError(t, err)

	// Try to delete as user2
	err = service.DeleteSharedLink(ctx, user2ID, link.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestIntegration_AddAssetsToSharedLink(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and assets
	userID := createTestUser(t, tdb, "addassets@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "addasset1")
	asset2ID := createTestAsset(t, tdb, userID, "addasset2")

	// Create a shared link with one asset
	createLink, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:     "INDIVIDUAL",
		AssetIDs: []string{asset1ID.String()},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, createLink.AssetCount)

	// Add another asset
	updatedLink, err := service.AddAssetsToSharedLink(ctx, userID, createLink.ID, []string{asset2ID.String()})
	require.NoError(t, err)
	assert.Equal(t, 2, updatedLink.AssetCount)
	assert.Len(t, updatedLink.AssetIDs, 2)
	assert.Contains(t, updatedLink.AssetIDs, asset1ID)
	assert.Contains(t, updatedLink.AssetIDs, asset2ID)

	// Verify asset count
	getLink, err := service.GetSharedLink(ctx, userID, createLink.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, getLink.AssetCount)
}

func TestIntegration_RemoveAssetsFromSharedLink(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and assets
	userID := createTestUser(t, tdb, "removeassets@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "removeasset1")
	asset2ID := createTestAsset(t, tdb, userID, "removeasset2")

	// Create a shared link with two assets
	createLink, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:     "INDIVIDUAL",
		AssetIDs: []string{asset1ID.String(), asset2ID.String()},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, createLink.AssetCount)

	// Remove one asset
	updatedLink, err := service.RemoveAssetsFromSharedLink(ctx, userID, createLink.ID, []string{asset2ID.String()})
	require.NoError(t, err)
	assert.Equal(t, 1, updatedLink.AssetCount)
	assert.Len(t, updatedLink.AssetIDs, 1)
	assert.Contains(t, updatedLink.AssetIDs, asset1ID)

	// Verify asset count
	getLink, err := service.GetSharedLink(ctx, userID, createLink.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, getLink.AssetCount)
}

func TestIntegration_GetSharedLinkAssets(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and assets
	userID := createTestUser(t, tdb, "getassets@test.com")
	asset1ID := createTestAsset(t, tdb, userID, "getasset1")
	asset2ID := createTestAsset(t, tdb, userID, "getasset2")

	// Create a shared link with two assets
	createLink, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:     "INDIVIDUAL",
		AssetIDs: []string{asset1ID.String(), asset2ID.String()},
	})
	require.NoError(t, err)

	// Get assets from the shared link
	assets, err := service.GetSharedLinkAssets(ctx, createLink.Key, "")
	require.NoError(t, err)
	assert.Len(t, assets, 2)
}

func TestIntegration_MultipleLinksPerAsset(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and asset
	userID := createTestUser(t, tdb, "multilink@test.com")
	assetID := createTestAsset(t, tdb, userID, "multilinkasset")

	// Create multiple shared links for the same asset
	link1, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:        "INDIVIDUAL",
		AssetIDs:    []string{assetID.String()},
		Description: "Link 1",
	})
	require.NoError(t, err)

	link2, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:        "INDIVIDUAL",
		AssetIDs:    []string{assetID.String()},
		Description: "Link 2",
	})
	require.NoError(t, err)

	// Both links should work independently
	assert.NotEqual(t, link1.Key, link2.Key)

	getLink1, err := service.GetSharedLinkByKey(ctx, link1.Key, "")
	require.NoError(t, err)
	assert.Equal(t, "Link 1", getLink1.Description)

	getLink2, err := service.GetSharedLinkByKey(ctx, link2.Key, "")
	require.NoError(t, err)
	assert.Equal(t, "Link 2", getLink2.Description)
}

func TestIntegration_AddAssetsToSharedLinkAccessDenied(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users and an asset owned by user2
	user1ID := createTestUser(t, tdb, "adddenied1@test.com")
	user2ID := createTestUser(t, tdb, "adddenied2@test.com")
	assetID := createTestAsset(t, tdb, user1ID, "adddeniedasset")
	otherAssetID := createTestAsset(t, tdb, user2ID, "adddeniedotherasset")

	// Create a shared link as user1
	link, err := service.CreateSharedLink(ctx, user1ID, &CreateSharedLinkRequest{
		Type:     "INDIVIDUAL",
		AssetIDs: []string{assetID.String()},
	})
	require.NoError(t, err)

	// Try to add an asset as user2
	_, err = service.AddAssetsToSharedLink(ctx, user2ID, link.ID, []string{otherAssetID.String()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestIntegration_RemoveAssetsFromSharedLinkAccessDenied(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users and assets
	user1ID := createTestUser(t, tdb, "removedenied1@test.com")
	user2ID := createTestUser(t, tdb, "removedenied2@test.com")
	asset1ID := createTestAsset(t, tdb, user1ID, "removedeniedasset1")
	asset2ID := createTestAsset(t, tdb, user1ID, "removedeniedasset2")

	// Create a shared link as user1
	link, err := service.CreateSharedLink(ctx, user1ID, &CreateSharedLinkRequest{
		Type:     "INDIVIDUAL",
		AssetIDs: []string{asset1ID.String(), asset2ID.String()},
	})
	require.NoError(t, err)

	// Try to remove an asset as user2
	_, err = service.RemoveAssetsFromSharedLink(ctx, user2ID, link.ID, []string{asset1ID.String()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestIntegration_AddAssetsToAlbumSharedLinkRejected(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user, asset, and album
	userID := createTestUser(t, tdb, "albumadd@test.com")
	assetID := createTestAsset(t, tdb, userID, "albumaddasset")
	album, err := tdb.Queries.CreateAlbum(ctx, sqlc.CreateAlbumParams{
		OwnerId:     pgtype.UUID{Bytes: userID, Valid: true},
		AlbumName:   "Test Album",
		Description: "",
	})
	require.NoError(t, err)

	albumID := uuid.UUID(album.ID.Bytes)
	link, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:    "ALBUM",
		AlbumID: ptr(albumID.String()),
	})
	require.NoError(t, err)

	// Adding assets to an album shared link is not supported
	_, err = service.AddAssetsToSharedLink(ctx, userID, link.ID, []string{assetID.String()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "asset modification is only supported for individual shared links")
}

func TestIntegration_RemoveAssetsFromAlbumSharedLinkRejected(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create test user and album
	userID := createTestUser(t, tdb, "albumremove@test.com")
	album, err := tdb.Queries.CreateAlbum(ctx, sqlc.CreateAlbumParams{
		OwnerId:     pgtype.UUID{Bytes: userID, Valid: true},
		AlbumName:   "Test Album",
		Description: "",
	})
	require.NoError(t, err)

	albumID := uuid.UUID(album.ID.Bytes)
	link, err := service.CreateSharedLink(ctx, userID, &CreateSharedLinkRequest{
		Type:    "ALBUM",
		AlbumID: ptr(albumID.String()),
	})
	require.NoError(t, err)

	assetID := uuid.New()

	// Removing assets from an album shared link is not supported
	_, err = service.RemoveAssetsFromSharedLink(ctx, userID, link.ID, []string{assetID.String()})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "asset modification is only supported for individual shared links")
}

func ptr(s string) *string {
	return &s
}
