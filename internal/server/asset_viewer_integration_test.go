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
	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/denysvitali/immich-go-backend/internal/jobs"
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

func assetViewerContext(userID uuid.UUID) context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{
		UserID: userID.String(),
		Email:  "asset-viewer-" + userID.String() + "@example.com",
	})
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
	userID := createAssetViewerTestUser(t, ctx, env.tdb)

	resp, err := env.srv.GetAsset(assetViewerContext(userID), &immichv1.GetAssetRequest{
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

	resp, err := env.srv.GetAsset(assetViewerContext(userID), &immichv1.GetAssetRequest{
		AssetId: uuid.UUID(asset.ID.Bytes).String(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, uuid.UUID(asset.ID.Bytes).String(), resp.GetId())
	assert.Equal(t, asset.OriginalFileName, resp.GetOriginalFileName())
	assert.Equal(t, immichv1.AssetType_ASSET_TYPE_IMAGE, resp.GetType())
}

func TestServer_UpdateAsset_UserIsolation(t *testing.T) {
	env := newAssetViewerTestEnv(t)
	ctx := context.Background()

	userA := createAssetViewerTestUser(t, ctx, env.tdb)
	userB := createAssetViewerTestUser(t, ctx, env.tdb)
	assetA := seedAsset(t, ctx, env, userA, "update-userA-only.jpg", "image/jpeg", []byte("userA-bytes"))
	assetAID := uuid.UUID(assetA.ID.Bytes).String()

	favorite := true
	assertAssetViewerNotFound(t, func() error {
		_, err := env.srv.UpdateAsset(assetViewerContext(userB), &immichv1.UpdateAssetRequest{
			AssetId:    assetAID,
			IsFavorite: &favorite,
		})
		return err
	})

	reloaded, err := env.tdb.Queries.GetAssetByID(ctx, assetA.ID)
	require.NoError(t, err)
	assert.False(t, reloaded.IsFavorite)

	resp, err := env.srv.UpdateAsset(assetViewerContext(userA), &immichv1.UpdateAssetRequest{
		AssetId:    assetAID,
		IsFavorite: &favorite,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.GetIsFavorite())

	reloaded, err = env.tdb.Queries.GetAssetByID(ctx, assetA.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.IsFavorite)
}

func TestServer_UpdateAssets_UserIsolation(t *testing.T) {
	env := newAssetViewerTestEnv(t)
	ctx := context.Background()

	userA := createAssetViewerTestUser(t, ctx, env.tdb)
	userB := createAssetViewerTestUser(t, ctx, env.tdb)
	assetA := seedAsset(t, ctx, env, userA, "bulk-update-userA.jpg", "image/jpeg", []byte("userA-bytes"))
	assetB := seedAsset(t, ctx, env, userB, "bulk-update-userB.jpg", "image/jpeg", []byte("userB-bytes"))
	assetAID := uuid.UUID(assetA.ID.Bytes).String()
	assetBID := uuid.UUID(assetB.ID.Bytes).String()

	favorite := true
	assertAssetViewerNotFound(t, func() error {
		_, err := env.srv.UpdateAssets(assetViewerContext(userA), &immichv1.UpdateAssetsRequest{
			AssetIds:   []string{assetAID, assetBID},
			IsFavorite: &favorite,
		})
		return err
	})

	reloadedA, err := env.tdb.Queries.GetAssetByID(ctx, assetA.ID)
	require.NoError(t, err)
	assert.False(t, reloadedA.IsFavorite, "bulk update should not partially update owned assets before access checks finish")
	reloadedB, err := env.tdb.Queries.GetAssetByID(ctx, assetB.ID)
	require.NoError(t, err)
	assert.False(t, reloadedB.IsFavorite)

	resp, err := env.srv.UpdateAssets(assetViewerContext(userA), &immichv1.UpdateAssetsRequest{
		AssetIds:   []string{assetAID},
		IsFavorite: &favorite,
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	reloadedA, err = env.tdb.Queries.GetAssetByID(ctx, assetA.ID)
	require.NoError(t, err)
	assert.True(t, reloadedA.IsFavorite)
	reloadedB, err = env.tdb.Queries.GetAssetByID(ctx, assetB.ID)
	require.NoError(t, err)
	assert.False(t, reloadedB.IsFavorite)
}

func TestServer_ReplaceAsset_UserIsolation(t *testing.T) {
	env := newAssetViewerTestEnv(t)
	ctx := context.Background()

	userA := createAssetViewerTestUser(t, ctx, env.tdb)
	userB := createAssetViewerTestUser(t, ctx, env.tdb)
	assetA := seedAsset(t, ctx, env, userA, "replace-userA-only.jpg", "image/jpeg", []byte("userA-bytes"))
	assetAID := uuid.UUID(assetA.ID.Bytes).String()

	assertAssetViewerNotFound(t, func() error {
		_, err := env.srv.ReplaceAsset(assetViewerContext(userB), &immichv1.ReplaceAssetRequest{
			AssetId: assetAID,
		})
		return err
	})

	resp, err := env.srv.ReplaceAsset(assetViewerContext(userA), &immichv1.ReplaceAssetRequest{
		AssetId: assetAID,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, assetAID, resp.GetId())
	assert.Equal(t, userA.String(), resp.GetOwnerId())
	assert.Equal(t, assetA.OriginalFileName, resp.GetOriginalFileName())
}

func TestServer_DeleteAssets_UserIsolation(t *testing.T) {
	env := newAssetViewerTestEnv(t)
	ctx := context.Background()

	userA := createAssetViewerTestUser(t, ctx, env.tdb)
	userB := createAssetViewerTestUser(t, ctx, env.tdb)
	assetA := seedAsset(t, ctx, env, userA, "delete-userA-only.jpg", "image/jpeg", []byte("userA-bytes"))
	assetAID := uuid.UUID(assetA.ID.Bytes).String()

	resp, err := env.srv.DeleteAssets(assetViewerContext(userB), &immichv1.DeleteAssetsRequest{
		Ids: []string{assetAID},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	reloaded, err := env.tdb.Queries.GetAssetByID(ctx, assetA.ID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.AssetsStatusEnumActive, reloaded.Status)

	resp, err = env.srv.DeleteAssets(assetViewerContext(userA), &immichv1.DeleteAssetsRequest{
		Ids: []string{assetAID},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)

	reloaded, err = env.tdb.Queries.GetAssetByID(ctx, assetA.ID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.AssetsStatusEnumTrashed, reloaded.Status)
}

func TestServer_AssetJobs_UserIsolation(t *testing.T) {
	env := newAssetViewerTestEnv(t)
	ctx := context.Background()

	userA := createAssetViewerTestUser(t, ctx, env.tdb)
	userB := createAssetViewerTestUser(t, ctx, env.tdb)
	assetA := seedAsset(t, ctx, env, userA, "job-userA-only.jpg", "image/jpeg", []byte("userA-bytes"))
	assetAID := uuid.UUID(assetA.ID.Bytes).String()

	assertAssetViewerNotFound(t, func() error {
		return env.srv.enqueueAssetJobsForAssets(
			ctx,
			mustUUID(t, userB),
			[]string{assetAID},
			jobs.JobTypeThumbnailGeneration,
			jobs.PriorityHigh,
		)
	})
}

// TestServer_GetAssetThumbnail_NotFound covers the NotFound path of the
// thumbnail handler. Because the asset does not exist in the DB, the
// handler returns codes.NotFound before ever touching storage.
func TestServer_GetAssetThumbnail_NotFound(t *testing.T) {
	env := newAssetViewerTestEnv(t)
	ctx := context.Background()
	userID := createAssetViewerTestUser(t, ctx, env.tdb)

	resp, err := env.srv.GetAssetThumbnail(assetViewerContext(userID), &immichv1.GetAssetThumbnailRequest{
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
	userID := createAssetViewerTestUser(t, ctx, env.tdb)

	resp, err := env.srv.DownloadAsset(assetViewerContext(userID), &immichv1.DownloadAssetRequest{
		AssetId: uuid.New().String(),
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// TestServer_AssetViewer_UserIsolation verifies that the server-level asset
// viewer handlers scope asset IDs to the authenticated owner before returning
// metadata or file bytes.
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

	resp, err := env.srv.GetAsset(assetViewerContext(userA), &immichv1.GetAssetRequest{
		AssetId: uuid.UUID(assetA.ID.Bytes).String(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, uuid.UUID(assetA.ID.Bytes).String(), resp.GetId())

	assertAssetViewerNotFound(t, func() error {
		_, err := env.srv.GetAsset(assetViewerContext(userB), &immichv1.GetAssetRequest{
			AssetId: uuid.UUID(assetA.ID.Bytes).String(),
		})
		return err
	})

	assertAssetViewerNotFound(t, func() error {
		_, err := env.srv.DownloadAsset(assetViewerContext(userB), &immichv1.DownloadAssetRequest{
			AssetId: uuid.UUID(assetA.ID.Bytes).String(),
		})
		return err
	})

	assertAssetViewerNotFound(t, func() error {
		_, err := env.srv.GetAssetThumbnail(assetViewerContext(userB), &immichv1.GetAssetThumbnailRequest{
			AssetId: uuid.UUID(assetA.ID.Bytes).String(),
		})
		return err
	})

	videoA := seedAsset(t, ctx, env, userA, "userA-only.mp4", "video/mp4", []byte("fake-mp4-bytes"))
	assertAssetViewerNotFound(t, func() error {
		_, err := env.srv.PlayAssetVideo(assetViewerContext(userB), &immichv1.PlayAssetVideoRequest{
			AssetId: uuid.UUID(videoA.ID.Bytes).String(),
		})
		return err
	})
}

func assertAssetViewerNotFound(t *testing.T, call func() error) {
	t.Helper()
	err := call()
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

// mustUUID is a small helper to convert a uuid.UUID to pgtype.UUID with a
// test-fatal error on the unlikely conversion failure.
func mustUUID(t *testing.T, id uuid.UUID) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	require.NoError(t, u.Scan(id.String()))
	return u
}
