//go:build integration
// +build integration

package libraries

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

func TestIntegration_CreateLibrary(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries, nil, nil)

	// Create user
	userID := createTestUser(t, tdb, "library@test.com")

	// Create library
	req := CreateLibraryRequest{
		Name:              "My Photos",
		Type:              LibraryTypeExternal,
		ImportPaths:       []string{"/photos", "/images"},
		ExclusionPatterns: []string{"*.tmp", "*.bak"},
	}

	library, err := service.CreateLibrary(ctx, userID, req)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, library.ID)
	assert.Equal(t, "My Photos", library.Name)
	assert.Equal(t, userID, library.OwnerID)
	assert.Equal(t, []string{"/photos", "/images"}, library.ImportPaths)
	assert.Equal(t, []string{"*.tmp", "*.bak"}, library.ExclusionPatterns)
}

func TestIntegration_CreateLibrary_EmptyName(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries, nil, nil)

	// Create user
	userID := createTestUser(t, tdb, "emptyname@test.com")

	// Try to create library with empty name
	req := CreateLibraryRequest{
		Name:        "",
		ImportPaths: []string{"/photos"},
	}

	_, err := service.CreateLibrary(ctx, userID, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "library name is required")
}

func TestIntegration_GetLibrary(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries, nil, nil)

	// Create user and library
	userID := createTestUser(t, tdb, "getlibrary@test.com")

	req := CreateLibraryRequest{
		Name:        "Test Library",
		ImportPaths: []string{"/test"},
	}

	created, err := service.CreateLibrary(ctx, userID, req)
	require.NoError(t, err)

	// Get the library
	library, err := service.GetLibrary(ctx, userID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, library.ID)
	assert.Equal(t, "Test Library", library.Name)
	assert.Equal(t, userID, library.OwnerID)
}

func TestIntegration_GetLibrary_NotFound(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries, nil, nil)

	// Create user
	userID := createTestUser(t, tdb, "notfound@test.com")

	// Try to get non-existent library
	_, err := service.GetLibrary(ctx, userID, uuid.New())
	assert.Error(t, err)
}

func TestIntegration_GetLibraries(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries, nil, nil)

	// Create user
	userID := createTestUser(t, tdb, "multilib@test.com")

	// Create multiple libraries
	for i := 0; i < 3; i++ {
		req := CreateLibraryRequest{
			Name:        "Library " + string(rune('A'+i)),
			ImportPaths: []string{"/path" + string(rune('A'+i))},
		}
		_, err := service.CreateLibrary(ctx, userID, req)
		require.NoError(t, err)
	}

	// Get all libraries
	libraries, err := service.GetLibraries(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, libraries, 3)
}

func TestIntegration_GetLibraries_UserIsolation(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries, nil, nil)

	// Create two users
	user1ID := createTestUser(t, tdb, "user1lib@test.com")
	user2ID := createTestUser(t, tdb, "user2lib@test.com")

	// Create libraries for user1
	for i := 0; i < 2; i++ {
		req := CreateLibraryRequest{
			Name:        "User1 Library",
			ImportPaths: []string{"/user1"},
		}
		_, err := service.CreateLibrary(ctx, user1ID, req)
		require.NoError(t, err)
	}

	// Create library for user2
	req := CreateLibraryRequest{
		Name:        "User2 Library",
		ImportPaths: []string{"/user2"},
	}
	_, err := service.CreateLibrary(ctx, user2ID, req)
	require.NoError(t, err)

	// User1 should only see their libraries
	libs1, err := service.GetLibraries(ctx, user1ID)
	require.NoError(t, err)
	assert.Len(t, libs1, 2)

	// User2 should only see their libraries
	libs2, err := service.GetLibraries(ctx, user2ID)
	require.NoError(t, err)
	assert.Len(t, libs2, 1)
}

func TestIntegration_UpdateLibrary(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries, nil, nil)

	// Create user and library
	userID := createTestUser(t, tdb, "update@test.com")

	req := CreateLibraryRequest{
		Name:              "Original Name",
		ImportPaths:       []string{"/original"},
		ExclusionPatterns: []string{"*.tmp"},
	}

	created, err := service.CreateLibrary(ctx, userID, req)
	require.NoError(t, err)

	// Update the library
	newName := "Updated Name"
	updateReq := &UpdateLibraryRequest{
		Name:              &newName,
		ImportPaths:       []string{"/updated", "/new"},
		ExclusionPatterns: []string{"*.bak"},
	}

	updated, err := service.UpdateLibrary(ctx, userID, created.ID, updateReq)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, []string{"/updated", "/new"}, updated.ImportPaths)
	assert.Equal(t, []string{"*.bak"}, updated.ExclusionPatterns)
}

func TestIntegration_UpdateLibrary_PartialUpdate(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries, nil, nil)

	// Create user and library
	userID := createTestUser(t, tdb, "partial@test.com")

	req := CreateLibraryRequest{
		Name:              "Original Name",
		ImportPaths:       []string{"/original"},
		ExclusionPatterns: []string{"*.tmp"},
	}

	created, err := service.CreateLibrary(ctx, userID, req)
	require.NoError(t, err)

	// Update only the name
	newName := "New Name Only"
	updateReq := &UpdateLibraryRequest{
		Name: &newName,
	}

	updated, err := service.UpdateLibrary(ctx, userID, created.ID, updateReq)
	require.NoError(t, err)
	assert.Equal(t, "New Name Only", updated.Name)
	// Other fields should remain unchanged
	assert.Equal(t, []string{"/original"}, updated.ImportPaths)
	assert.Equal(t, []string{"*.tmp"}, updated.ExclusionPatterns)
}

func TestIntegration_DeleteLibrary(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries, nil, nil)

	// Create user and library
	userID := createTestUser(t, tdb, "delete@test.com")

	req := CreateLibraryRequest{
		Name:        "To Be Deleted",
		ImportPaths: []string{"/delete"},
	}

	created, err := service.CreateLibrary(ctx, userID, req)
	require.NoError(t, err)

	// Verify library exists
	libs, err := service.GetLibraries(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, libs, 1)

	// Delete the library
	err = service.DeleteLibrary(ctx, userID, created.ID)
	require.NoError(t, err)

	// Verify library is deleted
	libs, err = service.GetLibraries(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, libs)
}

func TestIntegration_GetLibraryStatistics(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries, nil, nil)

	// Create user and library
	userID := createTestUser(t, tdb, "stats@test.com")

	req := CreateLibraryRequest{
		Name:        "Stats Library",
		ImportPaths: []string{"/stats"},
	}

	created, err := service.CreateLibrary(ctx, userID, req)
	require.NoError(t, err)

	// Get statistics (should be empty initially)
	stats, err := service.GetLibraryStatistics(ctx, userID, created.ID)
	require.NoError(t, err)
	assert.NotNil(t, stats)
	// Initially no assets
	assert.GreaterOrEqual(t, stats.Photos, int64(0))
}

func TestIntegration_ValidateImportPath(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)

	service := NewService(tdb.Queries, nil, nil)

	// Test empty path
	valid, msg := service.ValidateImportPath("")
	assert.False(t, valid)
	assert.Equal(t, "Path cannot be empty", msg)

	// Test valid path format
	valid, msg = service.ValidateImportPath("/some/path")
	assert.True(t, valid)
	assert.Empty(t, msg)
}

func TestIntegration_LibraryLifecycle(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries, nil, nil)

	// Create user
	userID := createTestUser(t, tdb, "lifecycle@test.com")

	// Initially no libraries
	libs, err := service.GetLibraries(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, libs)

	// Create library
	req := CreateLibraryRequest{
		Name:        "Lifecycle Library",
		ImportPaths: []string{"/photos"},
	}
	created, err := service.CreateLibrary(ctx, userID, req)
	require.NoError(t, err)

	// Should have 1 library
	libs, err = service.GetLibraries(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, libs, 1)

	// Update library
	newName := "Updated Lifecycle"
	updateReq := &UpdateLibraryRequest{Name: &newName}
	_, err = service.UpdateLibrary(ctx, userID, created.ID, updateReq)
	require.NoError(t, err)

	// Verify update
	lib, err := service.GetLibrary(ctx, userID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Lifecycle", lib.Name)

	// Delete library
	err = service.DeleteLibrary(ctx, userID, created.ID)
	require.NoError(t, err)

	// Should have no libraries
	libs, err = service.GetLibraries(ctx, userID)
	require.NoError(t, err)
	assert.Empty(t, libs)
}

func TestIntegration_MultipleLibraries(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries, nil, nil)

	// Create user
	userID := createTestUser(t, tdb, "multi@test.com")

	// Create multiple libraries with different configurations
	configs := []CreateLibraryRequest{
		{Name: "Photos", ImportPaths: []string{"/photos"}, ExclusionPatterns: []string{"*.tmp"}},
		{Name: "Videos", ImportPaths: []string{"/videos"}, ExclusionPatterns: []string{"*.bak"}},
		{Name: "Mixed", ImportPaths: []string{"/photos", "/videos"}, ExclusionPatterns: []string{}},
	}

	var libraryIDs []uuid.UUID
	for _, cfg := range configs {
		lib, err := service.CreateLibrary(ctx, userID, cfg)
		require.NoError(t, err)
		libraryIDs = append(libraryIDs, lib.ID)
	}

	// Get all libraries
	libs, err := service.GetLibraries(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, libs, 3)

	// Delete middle library
	err = service.DeleteLibrary(ctx, userID, libraryIDs[1])
	require.NoError(t, err)

	// Should have 2 libraries
	libs, err = service.GetLibraries(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, libs, 2)

	// Verify correct libraries remain
	names := make(map[string]bool)
	for _, lib := range libs {
		names[lib.Name] = true
	}
	assert.True(t, names["Photos"])
	assert.True(t, names["Mixed"])
	assert.False(t, names["Videos"])
}
