//go:build integration
// +build integration

package auth

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsInitializedRemainsTrueAfterAdminDeletion verifies that the durable
// bootstrap claim, rather than the current live-user count, controls whether
// the unauthenticated setup screen can ever be reopened.
func TestIsInitializedRemainsTrueAfterAdminDeletion(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := newTestAuthService(t, tdb)

	initialized, err := service.IsInitialized(ctx)
	require.NoError(t, err)
	assert.False(t, initialized, "a fresh database should not be initialized")

	registered, err := service.AdminSignUp(ctx, RegisterRequest{
		Email:    "is-initialized@test.com",
		Name:     "Initial Admin",
		Password: "TestPassword123!",
	})
	require.NoError(t, err)
	require.NotNil(t, registered)

	initialized, err = service.IsInitialized(ctx)
	require.NoError(t, err)
	assert.True(t, initialized, "database should be initialized once bootstrap succeeds")

	userID := mustUUID(t, uuid.MustParse(registered.User.ID))
	require.NoError(t, tdb.Queries.HardDeleteUser(ctx, userID))

	initialized, err = service.IsInitialized(ctx)
	require.NoError(t, err)
	assert.True(t, initialized, "deleting every user must not reopen bootstrap")
}
