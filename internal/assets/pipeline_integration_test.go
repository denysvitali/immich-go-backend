//go:build integration
// +build integration

package assets

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/denysvitali/immich-go-backend/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestConfig builds a minimal config.Config suitable for the assets service.
func newTestConfig(storageRoot string) *config.Config {
	cfg := &config.Config{}

	// Minimal server/auth defaults so that config validation is satisfied.
	cfg.Server.Address = "0.0.0.0:8080"
	cfg.Server.GRPCAddress = "0.0.0.0:9090"
	cfg.Auth.JWTSecret = "test-secret-key-that-is-long-enough-32+"

	cfg.Storage = storage.StorageConfig{
		Backend: "local",
		Local: storage.LocalConfig{
			RootPath: storageRoot,
			FileMode: "0644",
			DirMode:  "0755",
		},
		Upload: storage.UploadConfig{
			MaxFileSize: 104857600, // 100 MB
			// No extension/MIME restrictions so we can upload anything.
		},
	}

	cfg.Features = config.FeatureConfig{
		ThumbnailGenerationEnabled: true,
		EXIFExtractionEnabled:      true,
	}

	return cfg
}

// newTestUUID creates a pgtype.UUID from a uuid.UUID and fails the test on error.
func newTestUUID(t *testing.T, id uuid.UUID) pgtype.UUID {
	t.Helper()
	var u pgtype.UUID
	require.NoError(t, u.Scan(id.String()))
	return u
}

// setupPipeline sets up all the collaborators needed for the asset pipeline
// and returns a running Service together with the storage root directory.
func setupPipeline(t *testing.T, tdb *testdb.TestDB) (*Service, string) {
	t.Helper()

	storageRoot, err := os.MkdirTemp("", "immich-pipeline-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(storageRoot) })

	cfg := newTestConfig(storageRoot)

	storageService, err := storage.NewService(cfg.Storage)
	require.NoError(t, err)

	service, err := NewService(tdb.Queries, storageService, cfg, nil)
	require.NoError(t, err)

	return service, storageRoot
}

// createTestUser inserts a test user and returns its uuid.UUID.
func createTestUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	userUUID := newTestUUID(t, userID)
	_, err := tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "pipeline-test-" + userID.String() + "@example.com",
		Name:     "Pipeline Test User",
		Password: "hashed-password-not-used-in-tests",
		IsAdmin:  false,
	})
	require.NoError(t, err)
	return userID
}

// pollUntil retries check every 50 ms for up to maxWait and returns true
// when check returns true without error, or false when time runs out.
func pollUntil(maxWait time.Duration, check func() (bool, error)) bool {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		ok, err := check()
		if err == nil && ok {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// TestIntegration_FullAssetPipeline_Image tests the complete upload → process
// pipeline for a synthetic JPEG: file stored, EXIF written, thumbnails created.
func TestIntegration_FullAssetPipeline_Image(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service, _ := setupPipeline(t, tdb)
	userID := createTestUser(t, ctx, tdb)

	// ── 1. Initiate upload ────────────────────────────────────────────────────
	jpegData := createTestJPEG(800, 600)

	resp, err := service.InitiateUpload(ctx, UploadRequest{
		UserID:      userID,
		Filename:    "test-photo.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(jpegData)),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assetID := uuid.UUID(resp.AssetID)
	assert.NotEqual(t, uuid.Nil, assetID)

	// ── 2. Complete upload (stores file + triggers processAsset in background) ─
	err = service.CompleteUpload(ctx, assetID, bytes.NewReader(jpegData))
	require.NoError(t, err)

	// ── 3. Poll until the asset file is present in storage ───────────────────
	// processAsset runs in a goroutine; give it up to 30 s.
	assetUUID := newTestUUID(t, assetID)

	var storagePath string
	stored := pollUntil(30*time.Second, func() (bool, error) {
		asset, err := tdb.Queries.GetAssetByID(ctx, assetUUID)
		if err != nil {
			return false, err
		}
		storagePath = asset.OriginalPath
		if storagePath == "" {
			return false, nil
		}
		exists, err := service.GetStorageService().AssetExists(ctx, storagePath)
		return exists, err
	})
	require.True(t, stored, "asset file should be present in storage within 30 s")

	// ── 4. Verify the file can be downloaded ─────────────────────────────────
	rc, err := service.GetStorageService().Download(ctx, storagePath)
	require.NoError(t, err)
	require.NotNil(t, rc)
	rc.Close()

	// ── 5. Poll until EXIF / metadata is written ─────────────────────────────
	exifWritten := pollUntil(30*time.Second, func() (bool, error) {
		exifRow, err := tdb.Queries.GetAssetExif(ctx, assetUUID)
		if err != nil {
			// not yet written
			return false, nil
		}
		// file size must be positive
		return exifRow.FileSizeInByte.Valid && exifRow.FileSizeInByte.Int64 > 0, nil
	})
	assert.True(t, exifWritten, "EXIF with file size should be recorded after processing")

	// ── 6. Poll until at least one thumbnail is stored ───────────────────────
	thumbsGenerated := pollUntil(30*time.Second, func() (bool, error) {
		files, err := tdb.Queries.GetAssetFiles(ctx, assetUUID)
		if err != nil {
			return false, nil
		}
		return len(files) > 0, nil
	})
	assert.True(t, thumbsGenerated, "thumbnails should be generated and recorded after processing")

	// Verify each recorded thumbnail file actually exists in storage.
	assetFiles, err := tdb.Queries.GetAssetFiles(ctx, assetUUID)
	require.NoError(t, err)
	for _, af := range assetFiles {
		exists, err := service.GetStorageService().AssetExists(ctx, af.Path)
		assert.NoError(t, err)
		assert.True(t, exists, "thumbnail file should exist in storage: %s", af.Path)
	}
}

// TestIntegration_FullAssetPipeline_TextFile verifies that a non-image upload
// is stored and has its file size recorded, but no thumbnails are generated.
func TestIntegration_FullAssetPipeline_TextFile(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service, _ := setupPipeline(t, tdb)
	userID := createTestUser(t, ctx, tdb)

	// ── 1. Initiate upload ────────────────────────────────────────────────────
	textContent := []byte("hello from the pipeline integration test\n")

	resp, err := service.InitiateUpload(ctx, UploadRequest{
		UserID:      userID,
		Filename:    "notes.txt",
		ContentType: "text/plain",
		Size:        int64(len(textContent)),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assetID := uuid.UUID(resp.AssetID)

	// ── 2. Complete upload ────────────────────────────────────────────────────
	err = service.CompleteUpload(ctx, assetID, bytes.NewReader(textContent))
	require.NoError(t, err)

	assetUUID := newTestUUID(t, assetID)

	// ── 3. Wait for the original file to appear ───────────────────────────────
	var storagePath string
	stored := pollUntil(30*time.Second, func() (bool, error) {
		asset, err := tdb.Queries.GetAssetByID(ctx, assetUUID)
		if err != nil {
			return false, err
		}
		storagePath = asset.OriginalPath
		if storagePath == "" {
			return false, nil
		}
		exists, err := service.GetStorageService().AssetExists(ctx, storagePath)
		return exists, err
	})
	require.True(t, stored, "text file should be present in storage within 30 s")

	// ── 4. Metadata: file size recorded ──────────────────────────────────────
	// processAsset records size via updateAssetMetadata.
	// For a text/plain file the metadata extractor still records the size.
	metaRecorded := pollUntil(30*time.Second, func() (bool, error) {
		exifRow, err := tdb.Queries.GetAssetExif(ctx, assetUUID)
		if err != nil {
			return false, nil
		}
		return exifRow.FileSizeInByte.Valid && exifRow.FileSizeInByte.Int64 > 0, nil
	})
	assert.True(t, metaRecorded, "file size should be recorded in exif table even for non-image assets")

	// ── 5. No thumbnails ──────────────────────────────────────────────────────
	// processAsset only generates thumbnails when CanGenerateThumbnail is true
	// for the MIME type.  text/plain is not in the supported set, so we expect
	// zero asset_files rows.  Give processing a moment to finish before checking.
	time.Sleep(2 * time.Second)

	assetFiles, err := tdb.Queries.GetAssetFiles(ctx, assetUUID)
	require.NoError(t, err)
	assert.Empty(t, assetFiles, "no thumbnails should be generated for a text file")
}

// TestIntegration_TriggerProcessing_Image tests the TriggerProcessing public
// helper by manually creating an asset record and uploading the file, then
// calling TriggerProcessing and verifying the outputs.
func TestIntegration_TriggerProcessing_Image(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service, _ := setupPipeline(t, tdb)
	userID := createTestUser(t, ctx, tdb)

	// ── 1. Initiate upload to create the DB record and get the storage path ──
	jpegData := createTestJPEG(400, 300)

	resp, err := service.InitiateUpload(ctx, UploadRequest{
		UserID:      userID,
		Filename:    "trigger-test.jpg",
		ContentType: "image/jpeg",
		Size:        int64(len(jpegData)),
	})
	require.NoError(t, err)
	assetID := uuid.UUID(resp.AssetID)
	assetUUID := newTestUUID(t, assetID)

	// Fetch the asset to learn the generated storage path.
	asset, err := tdb.Queries.GetAssetByID(ctx, assetUUID)
	require.NoError(t, err)
	storagePath := asset.OriginalPath
	require.NotEmpty(t, storagePath)

	// ── 2. Upload file to storage manually ───────────────────────────────────
	err = service.GetStorageService().Upload(ctx, storagePath, bytes.NewReader(jpegData), "image/jpeg")
	require.NoError(t, err)

	// ── 3. Trigger background processing explicitly ───────────────────────────
	service.TriggerProcessing(assetID)

	// ── 4. Poll for thumbnails ────────────────────────────────────────────────
	thumbsGenerated := pollUntil(30*time.Second, func() (bool, error) {
		files, err := tdb.Queries.GetAssetFiles(ctx, assetUUID)
		if err != nil {
			return false, nil
		}
		return len(files) > 0, nil
	})
	assert.True(t, thumbsGenerated, "thumbnails should be generated after TriggerProcessing")

	// ── 5. Verify EXIF recorded ───────────────────────────────────────────────
	exifRow, err := tdb.Queries.GetAssetExif(ctx, assetUUID)
	require.NoError(t, err)
	assert.True(t, exifRow.FileSizeInByte.Valid)
	assert.Greater(t, exifRow.FileSizeInByte.Int64, int64(0))

	// ── 6. Verify thumbnail files exist in storage ────────────────────────────
	assetFiles, err := tdb.Queries.GetAssetFiles(ctx, assetUUID)
	require.NoError(t, err)
	require.NotEmpty(t, assetFiles)

	for _, af := range assetFiles {
		exists, err := service.GetStorageService().AssetExists(ctx, af.Path)
		assert.NoError(t, err, "checking existence of thumbnail %s", af.Path)
		assert.True(t, exists, "thumbnail should exist in storage: %s", af.Path)
	}
}
