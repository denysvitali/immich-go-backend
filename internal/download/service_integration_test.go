//go:build integration
// +build integration

package download

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/denysvitali/immich-go-backend/internal/storage"
)

type downloadTestEnv struct {
	queries *sqlc.Queries
	service *Service
	storage *storage.Service
}

func newDownloadTestEnv(t *testing.T) *downloadTestEnv {
	t.Helper()
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	storageService := newDownloadLocalStorage(t, t.TempDir())

	return &downloadTestEnv{
		queries: tdb.Queries,
		service: NewService(tdb.Queries, storageService),
		storage: storageService,
	}
}

func newDownloadLocalStorage(t *testing.T, rootDir string) *storage.Service {
	t.Helper()

	storageService, err := storage.NewService(storage.StorageConfig{
		Backend: "local",
		Local: storage.LocalConfig{
			RootPath: rootDir,
			FileMode: "0644",
			DirMode:  "0755",
		},
		Upload: storage.UploadConfig{MaxFileSize: 1024 * 1024},
	})
	require.NoError(t, err)
	return storageService
}

func createDownloadUser(t *testing.T, ctx context.Context, env *downloadTestEnv, email string) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	_, err := env.queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:          pgUUID(userID),
		Email:       email,
		Name:        email,
		Password:    "hashed-password",
		IsOnboarded: true,
	})
	require.NoError(t, err)
	return userID
}

func seedDownloadAsset(
	t *testing.T,
	ctx context.Context,
	env *downloadTestEnv,
	ownerID uuid.UUID,
	filename string,
	content []byte,
) sqlc.Asset {
	t.Helper()

	assetID := uuid.New()
	originalPath := filepath.Join("uploads", ownerID.String(), filename)
	require.NoError(t, env.storage.UploadBytes(ctx, originalPath, content, "image/jpeg"))

	now := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	asset, err := env.queries.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    "download-test-" + assetID.String(),
		OwnerId:          pgUUID(ownerID),
		DeviceId:         "download-test-device",
		Type:             "IMAGE",
		OriginalPath:     originalPath,
		FileCreatedAt:    now,
		FileModifiedAt:   now,
		LocalDateTime:    now,
		OriginalFileName: filename,
		Checksum:         []byte(assetID.String()),
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err)

	_, err = env.queries.CreateExif(ctx, sqlc.CreateExifParams{
		AssetId:        asset.ID,
		FileSizeInByte: pgtype.Int8{Int64: int64(len(content)), Valid: true},
	})
	require.NoError(t, err)

	return asset
}

func shareDownloadAssetViaAlbum(
	t *testing.T,
	ctx context.Context,
	env *downloadTestEnv,
	ownerID uuid.UUID,
	viewerID uuid.UUID,
	asset sqlc.Asset,
) {
	t.Helper()

	album, err := env.queries.CreateAlbum(ctx, sqlc.CreateAlbumParams{
		OwnerId:   pgUUID(ownerID),
		AlbumName: "download share " + assetUUID(asset).String(),
	})
	require.NoError(t, err)
	require.NoError(t, env.queries.AddAssetToAlbum(ctx, sqlc.AddAssetToAlbumParams{
		AlbumsId: album.ID,
		AssetsId: asset.ID,
	}))
	require.NoError(t, env.queries.AddUserToAlbum(ctx, sqlc.AddUserToAlbumParams{
		AlbumsId: album.ID,
		UsersId:  pgUUID(viewerID),
		Role:     "viewer",
	}))
}

func TestIntegration_DownloadInfoAndArchiveRespectSharedAccess(t *testing.T) {
	env := newDownloadTestEnv(t)
	ctx := context.Background()

	ownerID := createDownloadUser(t, ctx, env, "download-owner@example.com")
	viewerID := createDownloadUser(t, ctx, env, "download-viewer@example.com")

	owned := seedDownloadAsset(t, ctx, env, viewerID, "owned.jpg", []byte("owned-by-viewer"))
	shared := seedDownloadAsset(t, ctx, env, ownerID, "shared.jpg", []byte("shared-with-viewer"))
	private := seedDownloadAsset(t, ctx, env, ownerID, "private.jpg", []byte("private-owner-only"))
	shareDownloadAssetViaAlbum(t, ctx, env, ownerID, viewerID, shared)

	ownedID := assetUUID(owned).String()
	sharedID := assetUUID(shared).String()
	privateID := assetUUID(private).String()
	req := &DownloadRequest{AssetIDs: []string{
		ownedID,
		privateID,
		"not-a-uuid",
		uuid.NewString(),
		sharedID,
	}}

	info, err := env.service.GetDownloadInfo(ctx, viewerID, req)
	require.NoError(t, err)
	assert.Equal(t, []string{ownedID, sharedID}, info.AssetIDs)
	assert.Equal(t, 2, info.AssetCount)
	assert.Equal(t, int64(len("owned-by-viewer")+len("shared-with-viewer")), info.TotalSize)

	var archive bytes.Buffer
	require.NoError(t, env.service.DownloadArchive(ctx, viewerID, req, &archive))
	entries := readZipEntriesByBaseName(t, archive.Bytes())
	assert.Equal(t, map[string]string{
		"owned.jpg":  "owned-by-viewer",
		"shared.jpg": "shared-with-viewer",
	}, entries)
	assert.NotContains(t, entries, "private.jpg")
}

func readZipEntriesByBaseName(t *testing.T, data []byte) map[string]string {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)

	entries := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		body, err := readZipFile(file)
		require.NoError(t, err)
		entries[path.Base(file.Name)] = string(body)
	}
	return entries
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}

	data, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return data, nil
}
