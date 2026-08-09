//go:build integration
// +build integration

package assets

import (
	"bytes"
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/denysvitali/immich-go-backend/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type gatedReader struct {
	reader  *bytes.Reader
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newGatedReader(data []byte) *gatedReader {
	return &gatedReader{
		reader:  bytes.NewReader(data),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (r *gatedReader) Read(p []byte) (int, error) {
	r.once.Do(func() {
		close(r.started)
		<-r.release
	})
	return r.reader.Read(p)
}

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

func TestIntegration_HardDeleteRemovesStorageBeforeDatabaseTombstone(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()
	service, _ := setupPipeline(t, tdb)
	userID := createTestUser(t, ctx, tdb)

	content := []byte("delete me")
	resp, err := service.InitiateUpload(ctx, UploadRequest{
		UserID:      userID,
		Filename:    "delete-me.txt",
		ContentType: "text/plain",
		Size:        int64(len(content)),
	})
	require.NoError(t, err)
	assetID := uuid.UUID(resp.AssetID)
	require.NoError(t, service.CompleteUpload(ctx, assetID, bytes.NewReader(content)))

	assetUUID := newTestUUID(t, assetID)
	processed := pollUntil(10*time.Second, func() (bool, error) {
		_, exifErr := tdb.Queries.GetAssetExif(ctx, assetUUID)
		return exifErr == nil, nil
	})
	require.True(t, processed, "asset processing should finish before deletion")
	asset, err := tdb.Queries.GetAssetByID(ctx, assetUUID)
	require.NoError(t, err)
	derivedPath := asset.OriginalPath + ".preview"
	require.NoError(t, service.GetStorageService().UploadBytes(ctx, derivedPath, []byte("preview"), "image/jpeg"))
	_, err = tdb.Queries.CreateAssetFile(ctx, sqlc.CreateAssetFileParams{
		AssetId: assetUUID,
		Type:    string(ThumbnailTypePreview),
		Path:    derivedPath,
	})
	require.NoError(t, err)

	require.NoError(t, service.HardDeleteAsset(ctx, assetID, userID))

	for _, path := range []string{asset.OriginalPath, derivedPath} {
		exists, existsErr := service.GetStorageService().AssetExists(ctx, path)
		require.NoError(t, existsErr)
		assert.False(t, exists, "storage object should be removed: %s", path)
	}
	_, err = tdb.Queries.GetAssetByID(ctx, assetUUID)
	assert.Error(t, err, "database tombstone should hide the asset")

	// Older releases could leave a database tombstone before storage cleanup.
	// Retrying through the current service must still discover and remove it.
	legacyID := tdb.CreateTestAsset(t, userID, "legacy-tombstone")
	legacyUUID := newTestUUID(t, legacyID)
	legacyAsset, err := tdb.Queries.GetAssetByID(ctx, legacyUUID)
	require.NoError(t, err)
	require.NoError(t, service.GetStorageService().UploadBytes(ctx, legacyAsset.OriginalPath, []byte("legacy"), "image/jpeg"))
	require.NoError(t, tdb.Queries.PermanentlyDeleteAssets(ctx, sqlc.PermanentlyDeleteAssetsParams{
		Column1: []pgtype.UUID{legacyUUID},
		OwnerId: newTestUUID(t, userID),
	}))
	require.NoError(t, service.HardDeleteAsset(ctx, legacyID, userID))
	legacyExists, err := service.GetStorageService().AssetExists(ctx, legacyAsset.OriginalPath)
	require.NoError(t, err)
	assert.False(t, legacyExists, "legacy tombstone storage should be removed on retry")
}

func TestIntegration_AssetFileFinalizationSerializesWithDeletionIntent(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()
	userID := createTestUser(t, ctx, tdb)
	assetID := tdb.CreateTestAsset(t, userID, "derived-file-lock")
	assetUUID := newTestUUID(t, assetID)

	producerTx, err := tdb.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = producerTx.Rollback(ctx) }()
	_, err = sqlc.New(producerTx).CreateAssetFile(ctx, sqlc.CreateAssetFileParams{
		AssetId: assetUUID,
		Type:    string(ThumbnailTypePreview),
		Path:    "derived/preview.jpg",
	})
	require.NoError(t, err)

	deletionDone := make(chan error, 1)
	go func() {
		_, updateErr := tdb.Queries.UpdateAssetStatus(ctx, sqlc.UpdateAssetStatusParams{
			ID:     assetUUID,
			Status: sqlc.AssetsStatusEnumDeleted,
		})
		deletionDone <- updateErr
	}()

	select {
	case updateErr := <-deletionDone:
		t.Fatalf("deletion intent did not wait for derived-file finalization: %v", updateErr)
	case <-time.After(200 * time.Millisecond):
	}
	require.NoError(t, producerTx.Commit(ctx))

	select {
	case updateErr := <-deletionDone:
		require.NoError(t, updateErr)
	case <-time.After(5 * time.Second):
		t.Fatal("deletion intent remained blocked after derived-file commit")
	}

	files, err := tdb.Queries.GetAssetFiles(ctx, assetUUID)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "derived/preview.jpg", files[0].Path)
}

func TestIntegration_CompleteUploadSerializesStorageWriteWithHardDelete(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()
	service, _ := setupPipeline(t, tdb)
	userID := createTestUser(t, ctx, tdb)
	content := []byte("write must commit before deletion sweeps it")

	response, err := service.InitiateUpload(ctx, UploadRequest{
		UserID:      userID,
		Filename:    "concurrent-complete.txt",
		ContentType: "text/plain",
		Size:        int64(len(content)),
	})
	require.NoError(t, err)
	assetID := uuid.UUID(response.AssetID)
	assetUUID := newTestUUID(t, assetID)
	asset, err := tdb.Queries.GetAssetByID(ctx, assetUUID)
	require.NoError(t, err)

	reader := newGatedReader(content)
	uploadDone := make(chan error, 1)
	go func() {
		uploadDone <- service.CompleteUpload(ctx, assetID, reader)
	}()

	select {
	case <-reader.started:
	case <-time.After(5 * time.Second):
		t.Fatal("complete upload did not reach the storage write")
	}

	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- service.HardDeleteAsset(ctx, assetID, userID)
	}()

	select {
	case deleteErr := <-deleteDone:
		t.Fatalf("hard delete did not wait for the locked original-file write: %v", deleteErr)
	case <-time.After(200 * time.Millisecond):
	}

	close(reader.release)
	select {
	case uploadErr := <-uploadDone:
		require.NoError(t, uploadErr)
	case <-time.After(10 * time.Second):
		t.Fatal("complete upload remained blocked")
	}
	select {
	case deleteErr := <-deleteDone:
		require.NoError(t, deleteErr)
	case <-time.After(10 * time.Second):
		t.Fatal("hard delete remained blocked after upload committed")
	}

	exists, err := service.GetStorageService().AssetExists(ctx, asset.OriginalPath)
	require.NoError(t, err)
	assert.False(t, exists, "hard delete must sweep the original written by the earlier transaction")
}

func TestIntegration_DeletionIntentPreventsConcurrentReplacementWrite(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()
	userID := createTestUser(t, ctx, tdb)
	assetID := tdb.CreateTestAsset(t, userID, "concurrent-replacement")
	assetUUID := newTestUUID(t, assetID)
	ownerUUID := newTestUUID(t, userID)

	deletionTx, err := tdb.Pool.Begin(ctx)
	require.NoError(t, err)
	deletionCommitted := false
	defer func() {
		if !deletionCommitted {
			_ = deletionTx.Rollback(ctx)
		}
	}()
	_, err = sqlc.New(deletionTx).UpdateAssetStatus(ctx, sqlc.UpdateAssetStatusParams{
		ID:     assetUUID,
		Status: sqlc.AssetsStatusEnumDeleted,
	})
	require.NoError(t, err)

	writeStarted := make(chan struct{}, 1)
	replacementDone := make(chan error, 1)
	go func() {
		_, replaceErr := tdb.Queries.ReplaceAssetOriginalFile(ctx, sqlc.ReplaceAssetOriginalFileParams{
			AssetID:  assetUUID,
			OwnerID:  ownerUUID,
			Checksum: []byte("replacement"),
			WriteOriginalFile: func(context.Context, sqlc.Asset) error {
				writeStarted <- struct{}{}
				return nil
			},
		})
		replacementDone <- replaceErr
	}()

	select {
	case <-writeStarted:
		t.Fatal("replacement wrote storage before deletion intent committed")
	case replaceErr := <-replacementDone:
		t.Fatalf("replacement did not wait for the deletion row lock: %v", replaceErr)
	case <-time.After(200 * time.Millisecond):
	}

	require.NoError(t, deletionTx.Commit(ctx))
	deletionCommitted = true
	select {
	case replaceErr := <-replacementDone:
		require.ErrorIs(t, replaceErr, sqlc.ErrAssetOriginalFileWriteRejected)
	case <-time.After(5 * time.Second):
		t.Fatal("replacement remained blocked after deletion intent committed")
	}
	select {
	case <-writeStarted:
		t.Fatal("replacement storage callback ran for a deletion-intent asset")
	default:
	}

	_, err = tdb.Queries.ReplaceAssetFile(ctx, sqlc.ReplaceAssetFileParams{
		Checksum: []byte("unguarded-replacement"),
		ID:       assetUUID,
		OwnerID:  ownerUUID,
	})
	require.ErrorIs(t, err, pgx.ErrNoRows, "replacement finalization must independently reject deleted assets")
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
