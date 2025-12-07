// +build integration

package apikeys

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

func TestIntegration_GenerateAPIKey(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)

	service := NewService(tdb.Queries)

	// Generate API key
	key, err := service.GenerateAPIKey()
	require.NoError(t, err)
	assert.NotEmpty(t, key)
	assert.Greater(t, len(key), 20) // Should be a reasonably long key

	// Generate another key - should be different
	key2, err := service.GenerateAPIKey()
	require.NoError(t, err)
	assert.NotEqual(t, key, key2)
}

func TestIntegration_HashAndVerifyAPIKey(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)

	service := NewService(tdb.Queries)

	// Generate and hash a key
	key, err := service.GenerateAPIKey()
	require.NoError(t, err)

	hash, err := service.HashAPIKey(key)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, key, hash)

	// Verify correct key
	assert.True(t, service.VerifyAPIKey(key, hash))

	// Verify wrong key
	assert.False(t, service.VerifyAPIKey("wrong-key", hash))
}

func TestIntegration_CreateAPIKey(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "apikey@test.com")

	// Create API key
	apiKey, rawKey, err := service.CreateAPIKey(ctx, userID, "My API Key")
	require.NoError(t, err)
	assert.NotNil(t, apiKey)
	assert.NotEmpty(t, rawKey)
	assert.Equal(t, "My API Key", apiKey.Name)
	assert.True(t, apiKey.ID.Valid)

	// The raw key should be verifiable against the stored hash
	assert.True(t, service.VerifyAPIKey(rawKey, apiKey.Key))
}

func TestIntegration_GetAPIKeysByUser(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "multikey@test.com")

	// Create multiple API keys
	_, _, err := service.CreateAPIKey(ctx, userID, "Key 1")
	require.NoError(t, err)
	_, _, err = service.CreateAPIKey(ctx, userID, "Key 2")
	require.NoError(t, err)
	_, _, err = service.CreateAPIKey(ctx, userID, "Key 3")
	require.NoError(t, err)

	// Get all keys
	keys, err := service.GetAPIKeysByUser(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, keys, 3)

	// Verify names
	names := make(map[string]bool)
	for _, key := range keys {
		names[key.Name] = true
	}
	assert.True(t, names["Key 1"])
	assert.True(t, names["Key 2"])
	assert.True(t, names["Key 3"])
}

func TestIntegration_GetAPIKeysByUser_UserIsolation(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "user1apikey@test.com")
	user2ID := createTestUser(t, tdb, "user2apikey@test.com")

	// Create keys for user1
	_, _, err := service.CreateAPIKey(ctx, user1ID, "User1 Key 1")
	require.NoError(t, err)
	_, _, err = service.CreateAPIKey(ctx, user1ID, "User1 Key 2")
	require.NoError(t, err)

	// Create key for user2
	_, _, err = service.CreateAPIKey(ctx, user2ID, "User2 Key")
	require.NoError(t, err)

	// User1 should only see their keys
	keys1, err := service.GetAPIKeysByUser(ctx, user1ID)
	require.NoError(t, err)
	assert.Len(t, keys1, 2)

	// User2 should only see their keys
	keys2, err := service.GetAPIKeysByUser(ctx, user2ID)
	require.NoError(t, err)
	assert.Len(t, keys2, 1)
}

func TestIntegration_DeleteAPIKey(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "deletekey@test.com")

	// Create API key
	apiKey, _, err := service.CreateAPIKey(ctx, userID, "To Be Deleted")
	require.NoError(t, err)

	// Verify key exists
	keys, err := service.GetAPIKeysByUser(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, keys, 1)

	// Delete the key
	keyID := uuid.UUID(apiKey.ID.Bytes)
	err = service.DeleteAPIKey(ctx, keyID, userID)
	require.NoError(t, err)

	// Verify key is deleted
	keys, err = service.GetAPIKeysByUser(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestIntegration_DeleteAPIKey_WrongUser(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "deleteowner@test.com")
	user2ID := createTestUser(t, tdb, "deletenotowner@test.com")

	// Create API key for user1
	apiKey, _, err := service.CreateAPIKey(ctx, user1ID, "Protected Key")
	require.NoError(t, err)

	// User2 tries to delete user1's key
	keyID := uuid.UUID(apiKey.ID.Bytes)
	err = service.DeleteAPIKey(ctx, keyID, user2ID)
	// This should fail or not affect the key
	// Note: The current implementation may not enforce this at the service level
	// but the query should only delete if both keyID and userID match

	// Key should still exist for user1
	keys, err := service.GetAPIKeysByUser(ctx, user1ID)
	require.NoError(t, err)
	assert.Len(t, keys, 1)
}

func TestIntegration_MultipleKeysLifecycle(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "lifecycle@test.com")

	// Initially no keys
	keys, err := service.GetAPIKeysByUser(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, keys)

	// Create first key
	key1, _, err := service.CreateAPIKey(ctx, userID, "Key 1")
	require.NoError(t, err)

	// Should have 1 key
	keys, err = service.GetAPIKeysByUser(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, keys, 1)

	// Create second key
	key2, _, err := service.CreateAPIKey(ctx, userID, "Key 2")
	require.NoError(t, err)

	// Should have 2 keys
	keys, err = service.GetAPIKeysByUser(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, keys, 2)

	// Delete first key
	err = service.DeleteAPIKey(ctx, uuid.UUID(key1.ID.Bytes), userID)
	require.NoError(t, err)

	// Should have 1 key
	keys, err = service.GetAPIKeysByUser(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Equal(t, "Key 2", keys[0].Name)

	// Delete second key
	err = service.DeleteAPIKey(ctx, uuid.UUID(key2.ID.Bytes), userID)
	require.NoError(t, err)

	// Should have no keys
	keys, err = service.GetAPIKeysByUser(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestIntegration_APIKeyWithEmptyName(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "emptyname@test.com")

	// Create API key with empty name
	apiKey, rawKey, err := service.CreateAPIKey(ctx, userID, "")
	require.NoError(t, err)
	assert.NotNil(t, apiKey)
	assert.NotEmpty(t, rawKey)
	assert.Equal(t, "", apiKey.Name)
}
