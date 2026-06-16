//go:build integration
// +build integration

package server

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

// recentlyAddedTestEnv bundles the test database and a Server wired with
// the minimal fields GetRecentlyAddedAssets needs. GetRecentlyAddedAssets
// only touches s.db, so we skip the storage / assetService plumbing.
type recentlyAddedTestEnv struct {
	srv *Server
	tdb *testdb.TestDB
}

// newRecentlyAddedTestEnv spins up a real Postgres container, an
// sqlc.Queries wrapper, and a Server literal that has its db field
// populated. Mirrors the pattern in asset_viewer_integration_test.go.
func newRecentlyAddedTestEnv(t *testing.T) *recentlyAddedTestEnv {
	t.Helper()

	tdb := testdb.SetupTestDB(t)

	srv := &Server{
		queries: tdb.Queries,
	}

	return &recentlyAddedTestEnv{
		srv: srv,
		tdb: tdb,
	}
}

// createRecentlyAddedTestUser inserts a fresh user row and returns the
// uuid.UUID form.
func createRecentlyAddedTestUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB, label string) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	var userUUID pgtype.UUID
	require.NoError(t, userUUID.Scan(userID.String()))
	_, err := tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "recently-added-" + label + "-" + userID.String() + "@example.com",
		Name:     "Recently Added Test User " + label,
		Password: "hashed-password-not-used-in-tests",
		IsAdmin:  false,
	})
	require.NoError(t, err)
	return userID
}

// seedRecentlyAddedAsset inserts one asset owned by ownerID with a
// specific fileCreatedAt timestamp. The asset is active, visible, and
// not deleted so it is eligible for GetRecentlyAddedAssets.
func seedRecentlyAddedAsset(t *testing.T, ctx context.Context, tdb *testdb.TestDB, ownerID uuid.UUID, filename string, fileCreatedAt time.Time) sqlc.Asset {
	t.Helper()
	var ownerUUID pgtype.UUID
	require.NoError(t, ownerUUID.Scan(ownerID.String()))

	ts := pgtype.Timestamptz{}
	require.NoError(t, ts.Scan(fileCreatedAt))

	storagePath := "users/" + ownerID.String() + "/" + filename

	asset, err := tdb.Queries.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    "device-" + filename,
		OwnerId:          ownerUUID,
		DeviceId:         "recently-added-test-device",
		Type:             "IMAGE",
		OriginalPath:     storagePath,
		FileCreatedAt:    ts,
		FileModifiedAt:   ts,
		LocalDateTime:    ts,
		OriginalFileName: filename,
		Checksum:         []byte("test-checksum-" + filename),
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err)
	return asset
}

// ctxWithClaims returns a context that carries auth claims for userID.
// GetRecentlyAddedAssets looks up the user via auth.GetClaimsFromStdContext.
func ctxWithClaims(t *testing.T, userID uuid.UUID) context.Context {
	t.Helper()
	claims := &auth.Claims{
		UserID: userID.String(),
		Email:  "claims-" + userID.String() + "@example.com",
	}
	return auth.WithClaims(context.Background(), claims)
}

// seedFiveAssets creates 5 assets for ownerID with strictly increasing
// fileCreatedAt timestamps so the test can assert reverse-chronological
// ordering. The base time is in the past to avoid clock-skew issues with
// the now() column default on assets.createdAt.
func seedFiveAssets(t *testing.T, ctx context.Context, tdb *testdb.TestDB, ownerID uuid.UUID) []sqlc.Asset {
	t.Helper()
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	assets := make([]sqlc.Asset, 0, 5)
	for i := 0; i < 5; i++ {
		ts := base.Add(time.Duration(i) * time.Hour)
		filename := uuid.NewString() + ".jpg"
		assets = append(assets, seedRecentlyAddedAsset(t, ctx, tdb, ownerID, filename, ts))
	}
	return assets
}

// TestServer_GetRecentlyAddedAssets_OK seeds 5 assets for one user, calls
// Server.GetRecentlyAddedAssets, and asserts that the response contains
// all 5 assets in reverse fileCreatedAt order (newest first).
func TestServer_GetRecentlyAddedAssets_OK(t *testing.T) {
	testdb.SkipIfNoDocker(t)
	env := newRecentlyAddedTestEnv(t)
	ctx := context.Background()

	userID := createRecentlyAddedTestUser(t, ctx, env.tdb, "ok")
	seeded := seedFiveAssets(t, ctx, env.tdb, userID)

	resp, err := env.srv.GetRecentlyAddedAssets(ctxWithClaims(t, userID), &immichv1.GetRecentlyAddedAssetsRequest{
		Limit: 0,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.GetAssets(), 5, "all 5 seeded assets should be returned")

	// Expected order: reverse of seeded order (newest fileCreatedAt first).
	for i, asset := range resp.GetAssets() {
		expectedID := uuid.UUID(seeded[len(seeded)-1-i].ID.Bytes).String()
		assert.Equal(t, expectedID, asset.GetId(), "asset at index %d should be the %d-th newest", i, i)
	}
}

// TestServer_GetRecentlyAddedAssets_Limit verifies that the Limit field
// caps the number of returned assets.
func TestServer_GetRecentlyAddedAssets_Limit(t *testing.T) {
	testdb.SkipIfNoDocker(t)
	env := newRecentlyAddedTestEnv(t)
	ctx := context.Background()

	userID := createRecentlyAddedTestUser(t, ctx, env.tdb, "limit")
	seedFiveAssets(t, ctx, env.tdb, userID)

	resp, err := env.srv.GetRecentlyAddedAssets(ctxWithClaims(t, userID), &immichv1.GetRecentlyAddedAssetsRequest{
		Limit: 2,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.GetAssets(), 2, "limit=2 should return exactly 2 assets")
}

// TestServer_GetRecentlyAddedAssets_DefaultLimit verifies that Limit=0
// uses the server's default of 12. With 5 seeded assets we still get all
// 5 back because 5 < 12.
func TestServer_GetRecentlyAddedAssets_DefaultLimit(t *testing.T) {
	testdb.SkipIfNoDocker(t)
	env := newRecentlyAddedTestEnv(t)
	ctx := context.Background()

	userID := createRecentlyAddedTestUser(t, ctx, env.tdb, "default")
	seedFiveAssets(t, ctx, env.tdb, userID)

	resp, err := env.srv.GetRecentlyAddedAssets(ctxWithClaims(t, userID), &immichv1.GetRecentlyAddedAssetsRequest{
		Limit: 0,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Len(t, resp.GetAssets(), 5, "limit=0 should fall back to default 12, returning all 5 seeded assets")
}

// TestServer_GetRecentlyAddedAssets_UserIsolation seeds 5 assets for
// userA and 5 for userB, then verifies that userA's call returns only
// userA's assets (and vice versa).
func TestServer_GetRecentlyAddedAssets_UserIsolation(t *testing.T) {
	testdb.SkipIfNoDocker(t)
	env := newRecentlyAddedTestEnv(t)
	ctx := context.Background()

	userA := createRecentlyAddedTestUser(t, ctx, env.tdb, "isolation-a")
	userB := createRecentlyAddedTestUser(t, ctx, env.tdb, "isolation-b")

	seededA := seedFiveAssets(t, ctx, env.tdb, userA)
	seededAssetsB := seedFiveAssets(t, ctx, env.tdb, userB)

	// Build a set of userB asset IDs so we can assert userA's response
	// contains none of them.
	userBIDs := make(map[string]struct{}, len(seededAssetsB))
	for _, a := range seededAssetsB {
		userBIDs[uuid.UUID(a.ID.Bytes).String()] = struct{}{}
	}

	// userA's call should only return userA's assets.
	respA, err := env.srv.GetRecentlyAddedAssets(ctxWithClaims(t, userA), &immichv1.GetRecentlyAddedAssetsRequest{
		Limit: 0,
	})
	require.NoError(t, err)
	require.NotNil(t, respA)
	require.Len(t, respA.GetAssets(), 5, "userA should see exactly 5 assets")
	for _, a := range respA.GetAssets() {
		_, isB := userBIDs[a.GetId()]
		assert.False(t, isB, "userA's response must not include userB's asset %q", a.GetId())
	}

	// userB's call should only return userB's assets, and the IDs must
	// match the seeded set in reverse order.
	userAIDs := make(map[string]struct{}, len(seededA))
	for _, a := range seededA {
		userAIDs[uuid.UUID(a.ID.Bytes).String()] = struct{}{}
	}

	respB, err := env.srv.GetRecentlyAddedAssets(ctxWithClaims(t, userB), &immichv1.GetRecentlyAddedAssetsRequest{
		Limit: 0,
	})
	require.NoError(t, err)
	require.NotNil(t, respB)
	require.Len(t, respB.GetAssets(), 5, "userB should see exactly 5 assets")
	for _, a := range respB.GetAssets() {
		_, isA := userAIDs[a.GetId()]
		assert.False(t, isA, "userB's response must not include userA's asset %q", a.GetId())
	}
	for i, a := range respB.GetAssets() {
		expectedID := uuid.UUID(seededAssetsB[len(seededAssetsB)-1-i].ID.Bytes).String()
		assert.Equal(t, expectedID, a.GetId(), "userB's asset at index %d should be the %d-th newest", i, i)
	}
}
