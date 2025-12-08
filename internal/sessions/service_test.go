//go:build integration
// +build integration

package sessions

import (
	"context"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"
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

func createTestService(t *testing.T, tdb *testdb.TestDB) *Service {
	t.Helper()

	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	// Create auth config for testing
	authConfig := config.AuthConfig{
		JWTSecret: "test-jwt-secret-key-for-testing",
	}

	// Create auth service with test config
	authService := auth.NewService(authConfig, tdb.Queries)

	return NewService(tdb.Queries, authService, logger)
}

func TestIntegration_CreateSession(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Create user
	userID := createTestUser(t, tdb, "session@test.com")

	// Create session
	session, err := service.CreateSession(ctx, userID.String(), "mobile", "iOS")
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, userID.String(), session.UserID)
	assert.Equal(t, "mobile", session.DeviceType)
	assert.Equal(t, "iOS", session.DeviceOS)
	assert.NotEmpty(t, session.Token)
	assert.True(t, session.ExpiresAt.After(time.Now()))
}

func TestIntegration_CreateSession_InvalidUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Try to create session with invalid user ID
	_, err := service.CreateSession(ctx, "not-a-valid-uuid", "mobile", "iOS")
	assert.Error(t, err)
}

func TestIntegration_CreateSession_UserNotFound(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Try to create session with non-existent user
	_, err := service.CreateSession(ctx, uuid.New().String(), "mobile", "iOS")
	assert.Error(t, err)
}

func TestIntegration_GetSessionsByUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Create user
	userID := createTestUser(t, tdb, "multisession@test.com")

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		_, err := service.CreateSession(ctx, userID.String(), "mobile", "iOS")
		require.NoError(t, err)
	}

	// Get all sessions
	sessions, err := service.GetSessionsByUserID(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, sessions, 3)
}

func TestIntegration_GetSessionsByUserID_UserIsolation(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Create two users
	user1ID := createTestUser(t, tdb, "user1session@test.com")
	user2ID := createTestUser(t, tdb, "user2session@test.com")

	// Create sessions for user1
	for i := 0; i < 2; i++ {
		_, err := service.CreateSession(ctx, user1ID.String(), "mobile", "iOS")
		require.NoError(t, err)
	}

	// Create session for user2
	_, err := service.CreateSession(ctx, user2ID.String(), "desktop", "Windows")
	require.NoError(t, err)

	// User1 should only see their sessions
	sessions1, err := service.GetSessionsByUserID(ctx, user1ID.String())
	require.NoError(t, err)
	assert.Len(t, sessions1, 2)

	// User2 should only see their sessions
	sessions2, err := service.GetSessionsByUserID(ctx, user2ID.String())
	require.NoError(t, err)
	assert.Len(t, sessions2, 1)
}

func TestIntegration_GetSessionByID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Create user and session
	userID := createTestUser(t, tdb, "getsession@test.com")
	created, err := service.CreateSession(ctx, userID.String(), "tablet", "Android")
	require.NoError(t, err)

	// Get session by ID
	session, err := service.GetSessionByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, session.ID)
	assert.Equal(t, userID.String(), session.UserID)
	assert.Equal(t, "tablet", session.DeviceType)
}

func TestIntegration_GetSessionByID_NotFound(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Try to get non-existent session
	_, err := service.GetSessionByID(ctx, uuid.New().String())
	assert.Error(t, err)
}

func TestIntegration_GetSessionByToken(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Create user and session
	userID := createTestUser(t, tdb, "tokentest@test.com")
	created, err := service.CreateSession(ctx, userID.String(), "mobile", "iOS")
	require.NoError(t, err)

	// Get session by token
	session, err := service.GetSessionByToken(ctx, created.Token)
	require.NoError(t, err)
	assert.Equal(t, created.ID, session.ID)
	assert.Equal(t, created.Token, session.Token)
}

func TestIntegration_GetSessionByToken_NotFound(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Try to get session with invalid token
	_, err := service.GetSessionByToken(ctx, "invalid-token")
	assert.Error(t, err)
}

func TestIntegration_GetSessionByToken_Empty(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Try to get session with empty token
	_, err := service.GetSessionByToken(ctx, "")
	assert.Error(t, err)
}

func TestIntegration_DeleteSession(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Create user and session
	userID := createTestUser(t, tdb, "deletesession@test.com")
	created, err := service.CreateSession(ctx, userID.String(), "mobile", "iOS")
	require.NoError(t, err)

	// Delete session
	err = service.DeleteSession(ctx, created.ID)
	require.NoError(t, err)

	// Verify session is deleted
	_, err = service.GetSessionByID(ctx, created.ID)
	assert.Error(t, err)
}

func TestIntegration_DeleteAllSessionsByUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Create user
	userID := createTestUser(t, tdb, "deleteall@test.com")

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		_, err := service.CreateSession(ctx, userID.String(), "mobile", "iOS")
		require.NoError(t, err)
	}

	// Verify sessions exist
	sessions, err := service.GetSessionsByUserID(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, sessions, 3)

	// Delete all sessions
	err = service.DeleteAllSessionsByUserID(ctx, userID.String())
	require.NoError(t, err)

	// Verify all sessions are deleted
	sessions, err = service.GetSessionsByUserID(ctx, userID.String())
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestIntegration_ValidateSession(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Create user and session
	userID := createTestUser(t, tdb, "validate@test.com")
	created, err := service.CreateSession(ctx, userID.String(), "mobile", "iOS")
	require.NoError(t, err)

	// Validate session
	session, err := service.ValidateSession(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, session.ID)
}

func TestIntegration_RefreshSession(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Create user and session
	userID := createTestUser(t, tdb, "refresh@test.com")
	created, err := service.CreateSession(ctx, userID.String(), "mobile", "iOS")
	require.NoError(t, err)

	// Wait a tiny bit for time difference
	time.Sleep(10 * time.Millisecond)

	// Refresh session
	err = service.RefreshSession(ctx, created.ID)
	require.NoError(t, err)

	// Get session and verify it's still valid
	session, err := service.GetSessionByID(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, session.UpdatedAt.After(created.CreatedAt) || session.UpdatedAt.Equal(created.CreatedAt))
}

func TestIntegration_LockSession(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Create user and session
	userID := createTestUser(t, tdb, "lock@test.com")
	created, err := service.CreateSession(ctx, userID.String(), "mobile", "iOS")
	require.NoError(t, err)

	// Lock session
	err = service.LockSession(ctx, created.ID)
	require.NoError(t, err)

	// Verify session is locked (deleted)
	_, err = service.GetSessionByID(ctx, created.ID)
	assert.Error(t, err)
}

func TestIntegration_MultipleDevices(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Create user
	userID := createTestUser(t, tdb, "multidevice@test.com")

	// Create sessions on different devices
	mobile, err := service.CreateSession(ctx, userID.String(), "mobile", "iOS")
	require.NoError(t, err)

	desktop, err := service.CreateSession(ctx, userID.String(), "desktop", "Windows")
	require.NoError(t, err)

	tablet, err := service.CreateSession(ctx, userID.String(), "tablet", "Android")
	require.NoError(t, err)

	// Get all sessions
	sessions, err := service.GetSessionsByUserID(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, sessions, 3)

	// Verify different device types
	deviceTypes := make(map[string]bool)
	for _, s := range sessions {
		deviceTypes[s.DeviceType] = true
	}
	assert.True(t, deviceTypes["mobile"])
	assert.True(t, deviceTypes["desktop"])
	assert.True(t, deviceTypes["tablet"])

	// Delete one session
	err = service.DeleteSession(ctx, mobile.ID)
	require.NoError(t, err)

	// Verify only 2 sessions remain
	sessions, err = service.GetSessionsByUserID(ctx, userID.String())
	require.NoError(t, err)
	assert.Len(t, sessions, 2)

	// The other sessions should still be valid
	_, err = service.GetSessionByID(ctx, desktop.ID)
	require.NoError(t, err)

	_, err = service.GetSessionByID(ctx, tablet.ID)
	require.NoError(t, err)
}

func TestIntegration_InvalidSessionID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := createTestService(t, tdb)

	// Test with invalid session ID
	_, err := service.GetSessionByID(ctx, "not-a-valid-uuid")
	assert.Error(t, err)

	err = service.DeleteSession(ctx, "not-a-valid-uuid")
	assert.Error(t, err)

	err = service.RefreshSession(ctx, "not-a-valid-uuid")
	assert.Error(t, err)

	err = service.LockSession(ctx, "not-a-valid-uuid")
	assert.Error(t, err)

	_, err = service.ValidateSession(ctx, "not-a-valid-uuid")
	assert.Error(t, err)
}
