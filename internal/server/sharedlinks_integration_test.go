//go:build integration
// +build integration

package server

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/sharedlinks"
)

// sharedLinksTestEnv bundles the test database and a Server wired with the
// minimal fields the shared-link handlers need.
type sharedLinksTestEnv struct {
	srv *Server
	tdb *testdb.TestDB
}

func newSharedLinksTestEnv(t *testing.T) *sharedLinksTestEnv {
	t.Helper()

	tdb := testdb.SetupTestDB(t)

	srv := &Server{
		queries:            tdb.Queries,
		sharedLinksService: sharedlinks.NewService(tdb.Queries),
	}

	return &sharedLinksTestEnv{
		srv: srv,
		tdb: tdb,
	}
}

func createSharedLinksTestUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	var userUUID pgtype.UUID
	require.NoError(t, userUUID.Scan(userID.String()))
	_, err := tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "shared-links-" + userID.String() + "@example.com",
		Name:     "Shared Links Test User",
		Password: "hashed-password-not-used-in-tests",
		IsAdmin:  false,
	})
	require.NoError(t, err)
	return userID
}

func seedSharedLinksTestAsset(t *testing.T, ctx context.Context, tdb *testdb.TestDB, ownerID uuid.UUID, filename string) uuid.UUID {
	t.Helper()

	var ownerUUID pgtype.UUID
	require.NoError(t, ownerUUID.Scan(ownerID.String()))

	asset, err := tdb.Queries.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    "device-" + filename,
		OwnerId:          ownerUUID,
		DeviceId:         "shared-links-test-device",
		Type:             "IMAGE",
		OriginalPath:     "/test/path/" + filename + ".jpg",
		OriginalFileName: filename + ".jpg",
		Checksum:         []byte("test-checksum-" + filename),
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err)
	return asset.ID.Bytes
}

func TestIntegration_GetMySharedLink_RedactsMetadataWhenDisabled(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	env := newSharedLinksTestEnv(t)
	ctx := context.Background()

	userID := createSharedLinksTestUser(t, ctx, env.tdb)
	assetID := seedSharedLinksTestAsset(t, ctx, env.tdb, userID, "redacted-photo")

	// Create a shared link with showMetadata=false.
	link, err := env.srv.sharedLinksService.CreateSharedLink(ctx, userID, &sharedlinks.CreateSharedLinkRequest{
		Type:     sharedlinks.SharedLinkTypeIndividual,
		AssetIDs: []string{assetID.String()},
		ShowExif: false,
	})
	require.NoError(t, err)

	resp, err := env.srv.GetMySharedLink(ctx, &immichv1.GetMySharedLinkRequest{
		Token: proto.String(link.Key),
	})
	require.NoError(t, err)
	require.Len(t, resp.Assets, 1)
	assert.False(t, resp.ShowMetadata)
	assert.Equal(t, assetID.String(), resp.AssetIds[0])
	assert.Empty(t, resp.Assets[0].OriginalFileName)
	assert.Empty(t, resp.Assets[0].OriginalPath)
	assert.Nil(t, resp.Assets[0].ExifInfo)
}

func TestIntegration_GetMySharedLink_IncludesMetadataWhenEnabled(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	env := newSharedLinksTestEnv(t)
	ctx := context.Background()

	userID := createSharedLinksTestUser(t, ctx, env.tdb)
	assetID := seedSharedLinksTestAsset(t, ctx, env.tdb, userID, "visible-photo")

	// Create a shared link with showMetadata=true (upstream default).
	link, err := env.srv.sharedLinksService.CreateSharedLink(ctx, userID, &sharedlinks.CreateSharedLinkRequest{
		Type:     sharedlinks.SharedLinkTypeIndividual,
		AssetIDs: []string{assetID.String()},
		ShowExif: true,
	})
	require.NoError(t, err)

	resp, err := env.srv.GetMySharedLink(ctx, &immichv1.GetMySharedLinkRequest{
		Token: proto.String(link.Key),
	})
	require.NoError(t, err)
	require.Len(t, resp.Assets, 1)
	assert.True(t, resp.ShowMetadata)
	assert.Equal(t, assetID.String(), resp.AssetIds[0])
	assert.Equal(t, "visible-photo.jpg", resp.Assets[0].OriginalFileName)
	assert.Equal(t, "/test/path/visible-photo.jpg", resp.Assets[0].OriginalPath)
}

func TestIntegration_SharedLinkLogin_RedactsMetadataWhenDisabled(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	env := newSharedLinksTestEnv(t)
	ctx := context.Background()

	userID := createSharedLinksTestUser(t, ctx, env.tdb)
	assetID := seedSharedLinksTestAsset(t, ctx, env.tdb, userID, "login-redacted-photo")

	link, err := env.srv.sharedLinksService.CreateSharedLink(ctx, userID, &sharedlinks.CreateSharedLinkRequest{
		Type:     sharedlinks.SharedLinkTypeIndividual,
		AssetIDs: []string{assetID.String()},
		ShowExif: false,
	})
	require.NoError(t, err)

	resp, err := env.srv.SharedLinkLogin(ctx, &immichv1.SharedLinkLoginRequest{
		Key: proto.String(link.Key),
	})
	require.NoError(t, err)
	require.Len(t, resp.Assets, 1)
	assert.False(t, resp.ShowMetadata)
	assert.Empty(t, resp.Assets[0].OriginalFileName)
	assert.Empty(t, resp.Assets[0].OriginalPath)
	assert.Nil(t, resp.Assets[0].ExifInfo)
}

func TestIntegration_SharedLinkLogin_IncludesMetadataWhenEnabled(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	env := newSharedLinksTestEnv(t)
	ctx := context.Background()

	userID := createSharedLinksTestUser(t, ctx, env.tdb)
	assetID := seedSharedLinksTestAsset(t, ctx, env.tdb, userID, "login-visible-photo")

	link, err := env.srv.sharedLinksService.CreateSharedLink(ctx, userID, &sharedlinks.CreateSharedLinkRequest{
		Type:     sharedlinks.SharedLinkTypeIndividual,
		AssetIDs: []string{assetID.String()},
		ShowExif: true,
	})
	require.NoError(t, err)

	resp, err := env.srv.SharedLinkLogin(ctx, &immichv1.SharedLinkLoginRequest{
		Key: proto.String(link.Key),
	})
	require.NoError(t, err)
	require.Len(t, resp.Assets, 1)
	assert.True(t, resp.ShowMetadata)
	assert.Equal(t, "login-visible-photo.jpg", resp.Assets[0].OriginalFileName)
	assert.Equal(t, "/test/path/login-visible-photo.jpg", resp.Assets[0].OriginalPath)
}
