//go:build integration
// +build integration

package server

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/denysvitali/immich-go-backend/internal/assets"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/storage"
)

// assetViewerTestEnv bundles every dependency needed to exercise the
// gRPC server-level asset viewer handlers. GetAsset only needs s.db, but
// DownloadAsset / GetAssetThumbnail / PlayAssetVideo also reach into
// s.assetService, so we wire both.
type assetViewerTestEnv struct {
	srv         *Server
	tdb         *testdb.TestDB
	storageRoot string
}

// newAssetViewerTestEnv spins up a real Postgres container, a real local
// storage root, and a Server literal whose fields are populated just enough
// to exercise the asset viewer handlers. It mirrors the structure used in
// auth_login_onboarded_test.go (a small Server literal + a real *sqlc.Queries
// backed by a testcontainer).
func newAssetViewerTestEnv(t *testing.T) *assetViewerTestEnv {
	t.Helper()

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	// Build a real *db.Conn so that the Server.db field (typed *db.Conn)
	// can be populated. db.Conn embeds *sqlc.Queries, which is what every
	// asset handler in asset.go actually uses.
	conn, err := db.New(ctx, tdb.ConnStr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// Build a real *assets.Service backed by the test container DB and a
	// local storage root. Server.DownloadAsset / GetAssetThumbnail /
	// PlayAssetVideo call s.assetService.GetStorageService().
	storageRoot, err := os.MkdirTemp("", "immich-asset-viewer-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(storageRoot) })

	cfg := &config.Config{
		Storage: storage.StorageConfig{
			Backend: "local",
			Local: storage.LocalConfig{
				RootPath: storageRoot,
				FileMode: "0644",
				DirMode:  "0755",
			},
			Upload: storage.UploadConfig{
				MaxFileSize: 104857600, // 100 MB
			},
		},
		Features: config.FeatureConfig{
			ThumbnailGenerationEnabled: true,
			EXIFExtractionEnabled:      true,
		},
	}

	storageService, err := storage.NewService(cfg.Storage)
	require.NoError(t, err)

	assetService, err := assets.NewService(tdb.Queries, storageService, cfg, nil)
	require.NoError(t, err)

	srv := &Server{
		db:           conn,
		queries:      tdb.Queries,
		assetService: assetService,
	}

	return &assetViewerTestEnv{
		srv:         srv,
		tdb:         tdb,
		storageRoot: storageRoot,
	}
}

// createAssetViewerTestUser inserts a fresh user row and returns the
// uuid.UUID form. We re-implement this (instead of using a helper from the
// assets package) so the test file has no cross-package coupling beyond
// what it already needs.
func createAssetViewerTestUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	var userUUID pgtype.UUID
	require.NoError(t, userUUID.Scan(userID.String()))
	_, err := tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "asset-viewer-" + userID.String() + "@example.com",
		Name:     "Asset Viewer Test User",
		Password: "hashed-password-not-used-in-tests",
		IsAdmin:  false,
	})
	require.NoError(t, err)
	return userID
}

// seedAsset creates an asset row owned by ownerID and uploads the raw bytes
// to the local storage root at the asset's OriginalPath so that download /
// thumbnail / video handlers can find them.
func seedAsset(t *testing.T, ctx context.Context, env *assetViewerTestEnv, ownerID uuid.UUID, filename, mimeType string, payload []byte) sqlc.Asset {
	t.Helper()
	var ownerUUID pgtype.UUID
	require.NoError(t, ownerUUID.Scan(ownerID.String()))

	assetType := "IMAGE"
	if mimeType == "video/mp4" {
		assetType = "VIDEO"
	}

	now := pgtype.Timestamptz{}
	require.NoError(t, now.Scan(nil))

	storagePath := "users/" + ownerID.String() + "/" + filename

	asset, err := env.tdb.Queries.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    "device-" + filename,
		OwnerId:          ownerUUID,
		DeviceId:         "asset-viewer-test-device",
		Type:             assetType,
		OriginalPath:     storagePath,
		FileCreatedAt:    now,
		FileModifiedAt:   now,
		LocalDateTime:    now,
		OriginalFileName: filename,
		Checksum:         []byte("test-checksum"),
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err)

	storageService := env.srv.assetService.GetStorageService()
	err = storageService.Upload(ctx, storagePath, bytes.NewReader(payload), mimeType)
	require.NoError(t, err)

	return asset
}

// TestServer_GetAsset_NotFound verifies that a request for a random UUID
// surfaces the right gRPC code (NotFound) without leaking internal details.
func TestServer_GetAsset_NotFound(t *testing.T) {
	env := newAssetViewerTestEnv(t)
	ctx := context.Background()

	resp, err := env.srv.GetAsset(ctx, &immichv1.GetAssetRequest{
		AssetId: uuid.New().String(),
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok, "expected a gRPC status error")
	assert.Equal(t, codes.NotFound, st.Code())
}

// TestServer_GetAsset_OK seeds a real user + asset and asserts the gRPC
// handler returns a proto.Asset with the seeded ID and a non-empty
// Checksum / original filename.
func TestServer_GetAsset_OK(t *testing.T) {
	env := newAssetViewerTestEnv(t)
	ctx := context.Background()

	userID := createAssetViewerTestUser(t, ctx, env.tdb)
	asset := seedAsset(t, ctx, env, userID, "ok-test.jpg", "image/jpeg", []byte("fake-jpeg-bytes"))

	resp, err := env.srv.GetAsset(ctx, &immichv1.GetAssetRequest{
		AssetId: uuid.UUID(asset.ID.Bytes).String(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, uuid.UUID(asset.ID.Bytes).String(), resp.GetId())
	assert.Equal(t, asset.OriginalFileName, resp.GetOriginalFileName())
	assert.Equal(t, immichv1.AssetType_ASSET_TYPE_IMAGE, resp.GetType())
}

// TestServer_GetAssetThumbnail_NotFound covers the NotFound path of the
// thumbnail handler. Because the asset does not exist in the DB, the
// handler returns codes.NotFound before ever touching storage.
func TestServer_GetAssetThumbnail_NotFound(t *testing.T) {
	env := newAssetViewerTestEnv(t)
	ctx := context.Background()

	resp, err := env.srv.GetAssetThumbnail(ctx, &immichv1.GetAssetThumbnailRequest{
		AssetId: uuid.New().String(),
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// TestServer_GetAssetOriginal_NotFound covers the DownloadAsset NotFound
// path. It uses a random UUID and asserts codes.NotFound, mirroring the
// upstream behavior tested in auth_login_onboarded_test.go.
func TestServer_GetAssetOriginal_NotFound(t *testing.T) {
	env := newAssetViewerTestEnv(t)
	ctx := context.Background()

	resp, err := env.srv.DownloadAsset(ctx, &immichv1.DownloadAssetRequest{
		AssetId: uuid.New().String(),
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// TestServer_AssetViewer_UserIsolation documents the current authorization
// behavior of the asset viewer endpoints. As of this writing, Server.GetAsset
// / Server.DownloadAsset / Server.GetAssetThumbnail / Server.PlayAssetVideo
// do NOT consult auth claims and will happily return any asset whose ID is
// supplied, regardless of the requesting user. This is a known gap relative
// to the upstream Immich server and the assets.Service.GetAsset helper, which
// joins on ownerId. The test asserts the current (permissive) behavior so
// that a future fix that adds user isolation will trip a red test, and so
// that we have a regression net for the NoOp path in the meantime.
//
// TODO(security): when the handlers are updated to enforce ownership (by
// reading auth claims and joining on ownerId, or by routing through the
// assets.Service), flip the assertions below to expect codes.PermissionDenied
// or to assert the user-isolated payload.
func TestServer_AssetViewer_UserIsolation(t *testing.T) {
	env := newAssetViewerTestEnv(t)
	ctx := context.Background()

	userA := createAssetViewerTestUser(t, ctx, env.tdb)
	userB := createAssetViewerTestUser(t, ctx, env.tdb)

	// User A owns the asset; user B has no assets.
	assetA := seedAsset(t, ctx, env, userA, "userA-only.jpg", "image/jpeg", []byte("userA-bytes"))

	// Sanity-check: there is no asset for user B.
	assetsB, err := env.tdb.Queries.GetUserAssets(ctx, sqlc.GetUserAssetsParams{
		OwnerId: mustUUID(t, userB),
		Limit:   pgtype.Int4{Int32: 100, Valid: true},
		Offset:  pgtype.Int4{Int32: 0, Valid: true},
	})
	require.NoError(t, err)
	assert.Empty(t, assetsB, "user B should not own any assets")

	// Calling Server.GetAsset with user A's asset ID succeeds today, even
	// when no claims for user A are present in the context. This documents
	// the current behavior; see the TODO above.
	resp, err := env.srv.GetAsset(ctx, &immichv1.GetAssetRequest{
		AssetId: uuid.UUID(assetA.ID.Bytes).String(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, uuid.UUID(assetA.ID.Bytes).String(), resp.GetId())
}

// mustUUID is a small helper to convert a uuid.UUID to pgtype.UUID with a
// test-fatal error on the unlikely conversion failure.
func mustUUID(t *testing.T, id uuid.UUID) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	require.NoError(t, u.Scan(id.String()))
	return u
}
