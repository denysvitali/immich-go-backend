// +build integration

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

func TestIntegration_Register(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}

	service := NewService(cfg, tdb.Queries)

	// Register a new user
	response, err := service.Register(ctx, RegisterRequest{
		Email:    "newuser@test.com",
		Password: "SecurePass123!",
		Name:     "New User",
	})
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotEmpty(t, response.AccessToken)
	assert.NotEmpty(t, response.RefreshToken)
	assert.Equal(t, "newuser@test.com", response.User.Email)
	assert.Equal(t, "New User", response.User.Name)
	assert.False(t, response.User.IsAdmin)

	// Verify user was created in database
	user, err := tdb.Queries.GetUserByEmail(ctx, "newuser@test.com")
	require.NoError(t, err)
	assert.Equal(t, "New User", user.Name)
}

func TestIntegration_RegisterDuplicate(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}

	service := NewService(cfg, tdb.Queries)

	// Register first user
	_, err := service.Register(ctx, RegisterRequest{
		Email:    "duplicate@test.com",
		Password: "SecurePass123!",
		Name:     "First User",
	})
	require.NoError(t, err)

	// Try to register with same email
	response, err := service.Register(ctx, RegisterRequest{
		Email:    "duplicate@test.com",
		Password: "AnotherPass123!",
		Name:     "Second User",
	})
	assert.Error(t, err)
	assert.Nil(t, response)

	authErr, ok := err.(*AuthError)
	require.True(t, ok)
	assert.Equal(t, ErrUserExists, authErr.Type)
}

func TestIntegration_RegisterDisabled(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		RegistrationEnabled: false, // Registration disabled
		PasswordMinLength:   8,
	}

	service := NewService(cfg, tdb.Queries)

	response, err := service.Register(ctx, RegisterRequest{
		Email:    "newuser@test.com",
		Password: "SecurePass123!",
		Name:     "New User",
	})
	assert.Error(t, err)
	assert.Nil(t, response)

	authErr, ok := err.(*AuthError)
	require.True(t, ok)
	assert.Equal(t, ErrRegistrationDisabled, authErr.Type)
}

func TestIntegration_Login(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}

	service := NewService(cfg, tdb.Queries)

	// First create a user with a known password
	password := "TestPassword123!"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	userID := uuid.New()
	userUUID := pgtype.UUID{}
	err = userUUID.Scan(userID.String())
	require.NoError(t, err)

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "login@test.com",
		Name:     "Login User",
		Password: string(hashedPassword),
		IsAdmin:  false,
	})
	require.NoError(t, err)

	// Login with correct credentials
	response, err := service.Login(ctx, LoginRequest{
		Email:    "login@test.com",
		Password: password,
	})
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotEmpty(t, response.AccessToken)
	assert.NotEmpty(t, response.RefreshToken)
	assert.Equal(t, "login@test.com", response.User.Email)
	assert.Equal(t, "Login User", response.User.Name)
}

func TestIntegration_LoginInvalidPassword(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}

	service := NewService(cfg, tdb.Queries)

	// Create a user
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("CorrectPassword123!"), bcrypt.DefaultCost)
	require.NoError(t, err)

	userID := uuid.New()
	userUUID := pgtype.UUID{}
	err = userUUID.Scan(userID.String())
	require.NoError(t, err)

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "wrongpass@test.com",
		Name:     "Wrong Pass User",
		Password: string(hashedPassword),
		IsAdmin:  false,
	})
	require.NoError(t, err)

	// Try to login with wrong password
	response, err := service.Login(ctx, LoginRequest{
		Email:    "wrongpass@test.com",
		Password: "WrongPassword123!",
	})
	assert.Error(t, err)
	assert.Nil(t, response)

	authErr, ok := err.(*AuthError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidCredentials, authErr.Type)
}

func TestIntegration_LoginUserNotFound(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}

	service := NewService(cfg, tdb.Queries)

	// Try to login with non-existent user
	response, err := service.Login(ctx, LoginRequest{
		Email:    "nonexistent@test.com",
		Password: "SomePassword123!",
	})
	assert.Error(t, err)
	assert.Nil(t, response)

	authErr, ok := err.(*AuthError)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidCredentials, authErr.Type)
}

func TestIntegration_LoginDeletedUser(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}

	service := NewService(cfg, tdb.Queries)

	// Create and then soft-delete a user
	password := "TestPassword123!"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	userID := uuid.New()
	userUUID := pgtype.UUID{}
	err = userUUID.Scan(userID.String())
	require.NoError(t, err)

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "deleted@test.com",
		Name:     "Deleted User",
		Password: string(hashedPassword),
		IsAdmin:  false,
	})
	require.NoError(t, err)

	// Soft delete the user
	err = tdb.Queries.SoftDeleteUser(ctx, userUUID)
	require.NoError(t, err)

	// Try to login with deleted user
	response, err := service.Login(ctx, LoginRequest{
		Email:    "deleted@test.com",
		Password: password,
	})
	assert.Error(t, err)
	assert.Nil(t, response)

	authErr, ok := err.(*AuthError)
	require.True(t, ok)
	assert.Equal(t, ErrUserDeleted, authErr.Type)
}

func TestIntegration_TokenValidation(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}

	service := NewService(cfg, tdb.Queries)

	// Register a user to get tokens
	response, err := service.Register(ctx, RegisterRequest{
		Email:    "tokentest@test.com",
		Password: "SecurePass123!",
		Name:     "Token Test User",
	})
	require.NoError(t, err)

	// Validate the access token
	claims, err := service.ValidateToken(response.AccessToken)
	require.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, "tokentest@test.com", claims.Email)
	assert.False(t, claims.IsAdmin)
}

func TestIntegration_RefreshToken(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}

	service := NewService(cfg, tdb.Queries)

	// Register a user to get tokens
	registerResponse, err := service.Register(ctx, RegisterRequest{
		Email:    "refreshtest@test.com",
		Password: "SecurePass123!",
		Name:     "Refresh Test User",
	})
	require.NoError(t, err)

	// Use the refresh token to get new tokens
	refreshResponse, err := service.RefreshToken(ctx, RefreshRequest{
		RefreshToken: registerResponse.RefreshToken,
	})
	require.NoError(t, err)
	assert.NotNil(t, refreshResponse)
	assert.NotEmpty(t, refreshResponse.AccessToken)
	assert.NotEmpty(t, refreshResponse.RefreshToken)

	// The new access token should be different from the old one
	assert.NotEqual(t, registerResponse.AccessToken, refreshResponse.AccessToken)
}

func TestIntegration_ChangePassword(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}

	service := NewService(cfg, tdb.Queries)

	// Create a user with a known password
	originalPassword := "OriginalPass123!"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(originalPassword), bcrypt.DefaultCost)
	require.NoError(t, err)

	userID := uuid.New()
	userUUID := pgtype.UUID{}
	err = userUUID.Scan(userID.String())
	require.NoError(t, err)

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "changepass@test.com",
		Name:     "Change Pass User",
		Password: string(hashedPassword),
		IsAdmin:  false,
	})
	require.NoError(t, err)

	// Change the password
	newPassword := "NewSecurePass123!"
	err = service.ChangePassword(ctx, userID.String(), ChangePasswordRequest{
		CurrentPassword: originalPassword,
		NewPassword:     newPassword,
	})
	require.NoError(t, err)

	// Verify can login with new password
	response, err := service.Login(ctx, LoginRequest{
		Email:    "changepass@test.com",
		Password: newPassword,
	})
	require.NoError(t, err)
	assert.NotNil(t, response)

	// Verify cannot login with old password
	response, err = service.Login(ctx, LoginRequest{
		Email:    "changepass@test.com",
		Password: originalPassword,
	})
	assert.Error(t, err)
	assert.Nil(t, response)
}

func TestIntegration_AdminUser(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}

	service := NewService(cfg, tdb.Queries)

	// Create an admin user
	password := "AdminPass123!"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	userID := uuid.New()
	userUUID := pgtype.UUID{}
	err = userUUID.Scan(userID.String())
	require.NoError(t, err)

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "admin@test.com",
		Name:     "Admin User",
		Password: string(hashedPassword),
		IsAdmin:  true,
	})
	require.NoError(t, err)

	// Login as admin
	response, err := service.Login(ctx, LoginRequest{
		Email:    "admin@test.com",
		Password: password,
	})
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.True(t, response.User.IsAdmin)

	// Validate token contains admin claim
	claims, err := service.ValidateToken(response.AccessToken)
	require.NoError(t, err)
	assert.True(t, claims.IsAdmin)
}
