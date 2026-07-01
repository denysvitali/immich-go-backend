//go:build integration
// +build integration

package auth

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIsInitializedFalseUntilFirstUser verifies that IsInitialized reflects
// the real user count: false on an empty database, true once any user
// exists. The frontend uses this to decide whether to show the "create
// admin account" registration screen instead of the login screen.
func TestIsInitializedFalseUntilFirstUser(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := newTestAuthService(t, tdb)

	initialized, err := service.IsInitialized(ctx)
	require.NoError(t, err)
	assert.False(t, initialized, "a fresh database should not be initialized")

	insertUser(t, tdb, "is-initialized@test.com", "First User", "TestPassword123!", false)

	initialized, err = service.IsInitialized(ctx)
	require.NoError(t, err)
	assert.True(t, initialized, "database should be initialized once a user exists")
}
