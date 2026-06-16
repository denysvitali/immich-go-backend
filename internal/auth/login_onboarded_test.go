package auth

import (
	"context"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func newTestAuthService(t *testing.T, tdb *testdb.TestDB) *Service {
	t.Helper()
	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		JWTRefreshExpiry:    time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}
	return NewService(cfg, tdb.Queries)
}

func insertUser(t *testing.T, tdb *testdb.TestDB, email, name, password string, isOnboarded bool) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	userID := uuid.New()
	userUUID := pgtype.UUID{}
	require.NoError(t, userUUID.Scan(userID.String()))

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:          userUUID,
		Email:       email,
		Name:        name,
		Password:    string(hashedPassword),
		IsAdmin:     false,
		IsOnboarded: isOnboarded,
	})
	require.NoError(t, err)
	return userID
}

// TestLoginFirstLoginMarksUserOnboarded verifies that a brand-new user (with
// isOnboarded=false at insert time) is auto-onboarded on their first successful
// login: the response carries IsOnboarded=true and the row in the DB has been
// updated to isOnboarded=true.
func TestLoginFirstLoginMarksUserOnboarded(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := newTestAuthService(t, tdb)

	email := "firstlogin-onboarded@test.com"
	password := "TestPassword123!"

	userID := insertUser(t, tdb, email, "First Login User", password, false)

	// Sanity: pre-condition — DB row starts at isOnboarded=false.
	pre, err := tdb.Queries.GetUserOnboarded(ctx, mustUUID(t, userID))
	require.NoError(t, err)
	assert.False(t, pre, "user should start with isOnboarded=false in the DB")

	// First successful login.
	response, err := service.Login(ctx, LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.True(t, response.User.IsOnboarded, "first login should return IsOnboarded=true")

	// DB row must now be isOnboarded=true.
	persisted, err := tdb.Queries.GetUserOnboarded(ctx, mustUUID(t, userID))
	require.NoError(t, err)
	assert.True(t, persisted, "user row should be marked onboarded after first login")
}

// TestLoginSecondLoginKeepsUserOnboarded verifies the idempotency of the
// auto-onboarding flow: after the first login has flipped the column to true,
// subsequent logins must still return IsOnboarded=true (no re-set) and the
// column must remain true.
func TestLoginSecondLoginKeepsUserOnboarded(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := newTestAuthService(t, tdb)

	email := "secondlogin-onboarded@test.com"
	password := "TestPassword123!"

	userID := insertUser(t, tdb, email, "Second Login User", password, false)

	// First login (auto-onboards).
	first, err := service.Login(ctx, LoginRequest{Email: email, Password: password})
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.True(t, first.User.IsOnboarded)

	// Second login — column is now true; SetUserOnboarded must NOT run, but
	// the response must still surface IsOnboarded=true.
	second, err := service.Login(ctx, LoginRequest{Email: email, Password: password})
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.True(t, second.User.IsOnboarded, "second login should still return IsOnboarded=true")

	persisted, err := tdb.Queries.GetUserOnboarded(ctx, mustUUID(t, userID))
	require.NoError(t, err)
	assert.True(t, persisted, "user row must remain onboarded=true")
}

// TestLoginUserCreatedWithoutIsOnboardedDefaultsFalse inserts a user using
// raw SQL (omitting the isOnboarded column) and asserts that the column
// defaults to false at the database level. This protects the schema default
// from regressions.
func TestLoginUserCreatedWithoutIsOnboardedDefaultsFalse(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("TestPassword123!"), bcrypt.DefaultCost)
	require.NoError(t, err)

	userID := uuid.New()
	userUUID := pgtype.UUID{}
	require.NoError(t, userUUID.Scan(userID.String()))

	// Insert without specifying "isOnboarded" — schema default must kick in.
	_, err = tdb.Pool.Exec(ctx, `
		INSERT INTO users (id, email, name, password, "isAdmin", "createdAt", "updatedAt")
		VALUES ($1, $2, $3, $4, false, NOW(), NOW())
	`, userUUID, "default-onboarded@test.com", "Default User", string(hashedPassword))
	require.NoError(t, err)

	// Now go through the auth Login path and confirm the column is read as
	// false (precondition for the auto-onboard flow).
	persisted, err := tdb.Queries.GetUserOnboarded(ctx, userUUID)
	require.NoError(t, err)
	assert.False(t, persisted, "isOnboarded column should default to false when not provided at insert time")

	// And the Login response must reflect IsOnboarded=false BEFORE the
	// auto-onboard logic flips it.
	service := newTestAuthService(t, tdb)
	response, err := service.Login(ctx, LoginRequest{
		Email:    "default-onboarded@test.com",
		Password: "TestPassword123!",
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.True(t, response.User.IsOnboarded, "after the login call the user must be marked onboarded")
}

// TestLoginAlreadyOnboardedUserReturnsTrue covers the "user was created with
// isOnboarded=true" path: no SetUserOnboarded write is needed and the response
// must echo the persisted value.
func TestLoginAlreadyOnboardedUserReturnsTrue(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := newTestAuthService(t, tdb)

	email := "already-onboarded@test.com"
	password := "TestPassword123!"

	userID := insertUser(t, tdb, email, "Already Onboarded User", password, true)

	response, err := service.Login(ctx, LoginRequest{Email: email, Password: password})
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.True(t, response.User.IsOnboarded)

	persisted, err := tdb.Queries.GetUserOnboarded(ctx, mustUUID(t, userID))
	require.NoError(t, err)
	assert.True(t, persisted)
}

func mustUUID(t *testing.T, u uuid.UUID) pgtype.UUID {
	t.Helper()
	out := pgtype.UUID{}
	require.NoError(t, out.Scan(u.String()))
	return out
}
