// +build integration

package users

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

func TestIntegration_CreateAndGetUser(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	// Create service
	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create a user directly in the database
	userID := uuid.New()
	userUUID := pgtype.UUID{}
	err = userUUID.Scan(userID.String())
	require.NoError(t, err)

	createdUser, err := tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "integration@test.com",
		Name:     "Integration Test User",
		Password: "$2a$10$hashedpassword",
		IsAdmin:  false,
	})
	require.NoError(t, err)

	// Use the service to get the user
	user, err := service.GetUser(ctx, userID)
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "integration@test.com", user.Email)
	assert.Equal(t, "Integration Test User", user.Name)
	assert.False(t, user.IsAdmin)

	// Verify the ID matches
	var createdUUID uuid.UUID
	uuidBytes, err := createdUser.ID.Value()
	require.NoError(t, err)
	if uuidStr, ok := uuidBytes.(string); ok {
		createdUUID, err = uuid.Parse(uuidStr)
		require.NoError(t, err)
	} else if uuidArr, ok := uuidBytes.([16]byte); ok {
		createdUUID = uuid.UUID(uuidArr)
	}
	assert.Equal(t, createdUUID, user.ID)
}

func TestIntegration_GetUserByEmail(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create a user directly in the database
	userID := uuid.New()
	userUUID := pgtype.UUID{}
	err = userUUID.Scan(userID.String())
	require.NoError(t, err)

	email := "email-lookup@test.com"
	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    email,
		Name:     "Email Lookup User",
		Password: "$2a$10$hashedpassword",
		IsAdmin:  false,
	})
	require.NoError(t, err)

	// Get user by email using the service
	user, err := service.GetUserByEmail(ctx, email)
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, email, user.Email)
	assert.Equal(t, "Email Lookup User", user.Name)
}

func TestIntegration_GetUserNotFound(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Try to get a non-existent user
	nonExistentID := uuid.New()
	user, err := service.GetUser(ctx, nonExistentID)
	assert.Error(t, err)
	assert.Nil(t, user)

	// Check that it's a UserNotFound error
	userErr, ok := err.(*UserError)
	require.True(t, ok)
	assert.Equal(t, ErrUserNotFound, userErr.Type)
}

func TestIntegration_ListUsers(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create multiple users
	for i := 0; i < 5; i++ {
		userID := uuid.New()
		userUUID := pgtype.UUID{}
		err = userUUID.Scan(userID.String())
		require.NoError(t, err)

		_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
			ID:       userUUID,
			Email:    "user" + string(rune('0'+i)) + "@test.com",
			Name:     "Test User " + string(rune('0'+i)),
			Password: "$2a$10$hashedpassword",
			IsAdmin:  false,
		})
		require.NoError(t, err)
	}

	// List users with pagination
	response, err := service.ListUsers(ctx, ListUsersRequest{
		Limit:  10,
		Offset: 0,
	})
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Len(t, response.Users, 5)
	assert.Equal(t, int64(5), int64(response.Total))

	// Test pagination
	response, err = service.ListUsers(ctx, ListUsersRequest{
		Limit:  2,
		Offset: 0,
	})
	require.NoError(t, err)
	assert.Len(t, response.Users, 2)
}

func TestIntegration_UpdateUser(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create a user
	userID := uuid.New()
	userUUID := pgtype.UUID{}
	err = userUUID.Scan(userID.String())
	require.NoError(t, err)

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "update@test.com",
		Name:     "Original Name",
		Password: "$2a$10$hashedpassword",
		IsAdmin:  false,
	})
	require.NoError(t, err)

	// Update the user
	newName := "Updated Name"
	newColor := "blue"
	updatedUser, err := service.UpdateUser(ctx, userID, UpdateUserRequest{
		Name:        &newName,
		AvatarColor: &newColor,
	})
	require.NoError(t, err)
	assert.NotNil(t, updatedUser)
	assert.Equal(t, "Updated Name", updatedUser.Name)
	assert.Equal(t, "blue", *updatedUser.AvatarColor)

	// Verify the update persisted
	user, err := service.GetUser(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", user.Name)
}

func TestIntegration_UpdateUserPassword(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create a user
	userID := uuid.New()
	userUUID := pgtype.UUID{}
	err = userUUID.Scan(userID.String())
	require.NoError(t, err)

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "password@test.com",
		Name:     "Password User",
		Password: "$2a$10$oldpassword",
		IsAdmin:  false,
	})
	require.NoError(t, err)

	// Update the password
	err = service.UpdateUserPassword(ctx, userID, UpdatePasswordRequest{
		NewPassword: "NewSecurePassword123!",
	})
	require.NoError(t, err)

	// Verify the password was updated (check it's different from original)
	user, err := tdb.Queries.GetUser(ctx, userUUID)
	require.NoError(t, err)
	assert.NotEqual(t, "$2a$10$oldpassword", user.Password)
}

func TestIntegration_DeleteUser(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create a user
	userID := uuid.New()
	userUUID := pgtype.UUID{}
	err = userUUID.Scan(userID.String())
	require.NoError(t, err)

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "delete@test.com",
		Name:     "Delete User",
		Password: "$2a$10$hashedpassword",
		IsAdmin:  false,
	})
	require.NoError(t, err)

	// Soft delete the user
	err = service.DeleteUser(ctx, userID, false)
	require.NoError(t, err)

	// Try to get the deleted user - should fail
	user, err := service.GetUser(ctx, userID)
	assert.Error(t, err)
	assert.Nil(t, user)

	// Check that it's a UserDeleted error
	userErr, ok := err.(*UserError)
	require.True(t, ok)
	assert.Equal(t, ErrUserDeleted, userErr.Type)
}

func TestIntegration_AdminUser(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := &config.Config{}
	service, err := NewService(tdb.Queries, cfg)
	require.NoError(t, err)

	// Create an admin user
	userID := uuid.New()
	userUUID := pgtype.UUID{}
	err = userUUID.Scan(userID.String())
	require.NoError(t, err)

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "admin@test.com",
		Name:     "Admin User",
		Password: "$2a$10$hashedpassword",
		IsAdmin:  true,
	})
	require.NoError(t, err)

	// Get the admin user
	user, err := service.GetUser(ctx, userID)
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.True(t, user.IsAdmin)
}

func TestIntegration_UniqueEmailConstraint(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	// Create first user
	userID1 := uuid.New()
	userUUID1 := pgtype.UUID{}
	err := userUUID1.Scan(userID1.String())
	require.NoError(t, err)

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID1,
		Email:    "unique@test.com",
		Name:     "First User",
		Password: "$2a$10$hashedpassword",
		IsAdmin:  false,
	})
	require.NoError(t, err)

	// Try to create second user with same email
	userID2 := uuid.New()
	userUUID2 := pgtype.UUID{}
	err = userUUID2.Scan(userID2.String())
	require.NoError(t, err)

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID2,
		Email:    "unique@test.com", // Same email
		Name:     "Second User",
		Password: "$2a$10$hashedpassword",
		IsAdmin:  false,
	})
	assert.Error(t, err) // Should fail due to unique constraint
}
