package server

import (
	"context"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// TestServerLoginReturnsIsOnboarded is an integration test that goes through
// the gRPC-style Server.Login entry point and asserts that the proto-level
// LoginResponse carries the IsOnboarded flag the same way the auth service
// does. It also verifies the mark-on-first-login semantics and the DB
// persistence of the column.
func TestServerLoginReturnsIsOnboarded(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		JWTRefreshExpiry:    time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}

	// Build a minimal Server: only authService is required for Login.
	srv := &Server{
		authService: auth.NewService(cfg, tdb.Queries),
	}

	email := "server-login-onboarded@test.com"
	password := "TestPassword123!"

	// Pre-create a user with isOnboarded=false so the auto-onboard code path
	// runs on the first Server.Login call.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	userID := uuid.New()
	userUUID := pgtype.UUID{}
	require.NoError(t, userUUID.Scan(userID.String()))

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:          userUUID,
		Email:       email,
		Name:        "Server Login User",
		Password:    string(hashedPassword),
		IsAdmin:     false,
		IsOnboarded: false,
	})
	require.NoError(t, err)

	// First login — Server.Login must return IsOnboarded=true and the row
	// in the DB must be updated.
	resp, err := srv.Login(ctx, &immichv1.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.GetIsOnboarded(), "Server.Login should propagate IsOnboarded=true on first login")
	assert.Equal(t, userID.String(), resp.GetUserId())
	assert.Equal(t, email, resp.GetUserEmail())
	assert.NotEmpty(t, resp.GetAccessToken())

	persisted, err := tdb.Queries.GetUserOnboarded(ctx, userUUID)
	require.NoError(t, err)
	assert.True(t, persisted, "user row should be marked onboarded after first Server.Login")

	// Second login — the column is now true; Server.Login must still return
	// IsOnboarded=true (idempotent).
	resp2, err := srv.Login(ctx, &immichv1.LoginRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err)
	require.NotNil(t, resp2)
	assert.True(t, resp2.GetIsOnboarded(), "second Server.Login should still return IsOnboarded=true")
}
