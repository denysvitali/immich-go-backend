//go:build integration
// +build integration

package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/assets"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/denysvitali/immich-go-backend/internal/storage"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createIntegrationTestJPEG creates a synthetic JPEG image large enough that
// all three thumbnail sizes will actually be smaller than the original.
func createIntegrationTestJPEG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}) //nolint:errcheck
	return buf.Bytes()
}

// newLocalStorageService creates a storage.Service backed by a temporary local
// directory. The caller must remove the directory after the test.
func newLocalStorageService(t *testing.T, rootDir string) *storage.Service {
	t.Helper()
	cfg := storage.StorageConfig{
		Backend: "local",
		Local: storage.LocalConfig{
			RootPath: rootDir,
			FileMode: "0644",
			DirMode:  "0755",
		},
		Upload: storage.UploadConfig{
			MaxFileSize: 104857600, // 100 MB — no restriction for tests
		},
	}
	svc, err := storage.NewService(cfg)
	require.NoError(t, err, "failed to create local storage service")
	return svc
}

func TestIntegration_HandleThumbnailGeneration(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	// ------------------------------------------------------------------
	// 1. Set up a temporary local storage directory.
	// ------------------------------------------------------------------
	tmpDir, err := os.MkdirTemp("", "immich-thumb-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	storageService := newLocalStorageService(t, tmpDir)

	// ------------------------------------------------------------------
	// 2. Create a user in the database.
	// ------------------------------------------------------------------
	userID := uuid.New()
	userUUID := pgtype.UUID{}
	require.NoError(t, userUUID.Scan(userID.String()))

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "thumbtest@example.com",
		Name:     "Thumb Test User",
		Password: "hashed-password-placeholder",
		IsAdmin:  false,
	})
	require.NoError(t, err, "failed to create test user")

	// ------------------------------------------------------------------
	// 3. Write a synthetic JPEG into local storage at a known path.
	// ------------------------------------------------------------------
	assetID := uuid.New()
	assetUUID := pgtype.UUID{}
	require.NoError(t, assetUUID.Scan(assetID.String()))

	// Store the original file at a path that the handler will look up via
	// asset.OriginalPath (which is the storage-relative path, i.e. the key
	// handed to storageService.Download).
	originalStoragePath := filepath.Join("uploads", userID.String(), "photo.jpg")
	jpegData := createIntegrationTestJPEG(2000, 1500)
	require.NoError(t,
		storageService.UploadBytes(ctx, originalStoragePath, jpegData, "image/jpeg"),
		"failed to upload test JPEG to local storage",
	)

	// ------------------------------------------------------------------
	// 4. Create the asset record in the database.
	// ------------------------------------------------------------------
	now := time.Now()
	nowPg := pgtype.Timestamptz{Time: now, Valid: true}

	asset, err := tdb.Queries.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    "device-" + assetID.String(),
		OwnerId:          userUUID,
		DeviceId:         "test-device",
		Type:             string(assets.AssetTypeImage),
		OriginalPath:     originalStoragePath,
		FileCreatedAt:    nowPg,
		FileModifiedAt:   nowPg,
		LocalDateTime:    nowPg,
		OriginalFileName: "photo.jpg",
		Checksum:         []byte("fakechecksum"),
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err, "failed to create test asset record")

	// ------------------------------------------------------------------
	// 5. Build the asynq.Task the handler expects.
	// ------------------------------------------------------------------
	payload := JobPayload{
		ID:     "test-thumb-job-" + assetID.String(),
		UserID: userID.String(),
		Data: map[string]interface{}{
			"asset_id": assetID.String(),
		},
		CreatedAt: time.Now(),
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	task := asynq.NewTask(string(JobTypeThumbnailGeneration), payloadBytes)

	// ------------------------------------------------------------------
	// 6. Create the Handlers instance and invoke the handler directly.
	// ------------------------------------------------------------------
	handlers := NewHandlers(tdb.Queries, nil, nil, storageService)
	err = handlers.HandleThumbnailGeneration(ctx, task)
	require.NoError(t, err, "HandleThumbnailGeneration returned an error")

	// ------------------------------------------------------------------
	// 7. Verify thumbnail files were written to storage.
	// ------------------------------------------------------------------
	generator := assets.NewThumbnailGenerator()
	expectedThumbnailTypes := []assets.ThumbnailType{
		assets.ThumbnailTypePreview,
		assets.ThumbnailTypeWebp,
		assets.ThumbnailTypeThumb,
	}

	for _, thumbType := range expectedThumbnailTypes {
		thumbPath := generator.GetThumbnailPath(originalStoragePath, thumbType)

		exists, err := storageService.AssetExists(ctx, thumbPath)
		assert.NoError(t, err, "error checking existence of thumbnail %s", thumbType)
		assert.True(t, exists, "thumbnail %s not found in storage at %s", thumbType, thumbPath)
	}

	// ------------------------------------------------------------------
	// 8. Verify thumbnail records were created in the database.
	// ------------------------------------------------------------------
	assetFiles, err := tdb.Queries.GetAssetFiles(ctx, asset.ID)
	require.NoError(t, err, "failed to query asset files from DB")

	// We expect one DB record per thumbnail type (3 total).
	assert.Len(t, assetFiles, len(expectedThumbnailTypes),
		"expected %d asset_files rows, got %d", len(expectedThumbnailTypes), len(assetFiles))

	// Build a set of the recorded types for easy membership checks.
	recordedTypes := make(map[string]bool, len(assetFiles))
	for _, f := range assetFiles {
		recordedTypes[f.Type] = true
	}

	for _, thumbType := range expectedThumbnailTypes {
		assert.True(t, recordedTypes[string(thumbType)],
			"expected asset_file record for thumbnail type %s", thumbType)
	}
}

// TestIntegration_HandleMetadataExtraction_PlainJPEG tests the full
// metadata-extraction pipeline with a synthetic plain JPEG (no EXIF).
// It verifies that:
//  1. HandleMetadataExtraction returns no error.
//  2. An EXIF record is created in the database.
//  3. The file size stored in the EXIF record matches the uploaded file size.
func TestIntegration_HandleMetadataExtraction_PlainJPEG(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "immich-meta-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	storageSvc := newLocalStorageService(t, tmpDir)

	// 1. Create a user.
	userID := uuid.New()
	userUUID := pgtype.UUID{}
	require.NoError(t, userUUID.Scan(userID.String()))

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "meta-test@example.com",
		Name:     "Meta Test User",
		Password: "hashed-password",
		IsAdmin:  false,
	})
	require.NoError(t, err, "failed to create test user")

	// 2. Prepare the JPEG bytes and choose a storage path.
	imgBytes := createIntegrationTestJPEG(80, 60)
	originalFileName := "test-photo.jpg"
	originalPath := "uploads/" + userID.String() + "/test-photo.jpg"

	// 3. Upload the JPEG to local storage at the path that will be stored in the asset.
	err = storageSvc.UploadBytes(ctx, originalPath, imgBytes, "image/jpeg")
	require.NoError(t, err, "failed to upload test JPEG to local storage")

	// 4. Create an asset record in the DB pointing to the stored file.
	now := time.Now()
	pgNow := pgtype.Timestamptz{Time: now, Valid: true}

	asset, err := tdb.Queries.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    uuid.New().String(),
		OwnerId:          userUUID,
		DeviceId:         "test-device",
		Type:             "IMAGE",
		OriginalPath:     originalPath,
		FileCreatedAt:    pgNow,
		FileModifiedAt:   pgNow,
		LocalDateTime:    pgNow,
		OriginalFileName: originalFileName,
		Checksum:         []byte("dummy-checksum"),
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err, "failed to create test asset")

	pgAssetID := asset.ID

	// 5. Create the required asset_job_status row (foreign-key constraint).
	_, err = tdb.Queries.CreateAssetJobStatus(ctx, sqlc.CreateAssetJobStatusParams{
		AssetId: pgAssetID,
	})
	require.NoError(t, err, "failed to create asset_job_status row")

	// 6. Build the Handlers with a real DB and real storage service.
	handlers := NewHandlers(tdb.Queries, nil, nil, storageSvc)

	// 7. Build and dispatch the asynq task.
	assetIDStr := uuid.UUID(pgAssetID.Bytes).String()
	payload := JobPayload{
		ID:     "test-meta-extraction-job",
		UserID: userID.String(),
		Data: map[string]interface{}{
			"asset_id": assetIDStr,
		},
		CreatedAt: time.Now(),
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)
	task := asynq.NewTask(string(JobTypeMetadataExtraction), payloadBytes)

	err = handlers.HandleMetadataExtraction(ctx, task)
	require.NoError(t, err, "HandleMetadataExtraction returned an unexpected error")

	// 8. Verify the EXIF record was created in the database.
	exif, err := tdb.Queries.GetAssetExif(ctx, pgAssetID)
	require.NoError(t, err, "expected an EXIF record to exist after metadata extraction")

	// The file size must have been recorded correctly.
	assert.True(t, exif.FileSizeInByte.Valid, "FileSizeInByte should be valid")
	assert.Equal(t, int64(len(imgBytes)), exif.FileSizeInByte.Int64, "stored file size must match uploaded JPEG size")
}

// TestIntegration_HandleMetadataExtraction_InvalidAssetID verifies that the
// handler returns a descriptive error when the task payload contains a
// malformed asset UUID.
func TestIntegration_HandleMetadataExtraction_InvalidAssetID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "immich-meta-invalid-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	storageSvc := newLocalStorageService(t, tmpDir)
	handlers := NewHandlers(tdb.Queries, nil, nil, storageSvc)

	payload := JobPayload{
		ID:     "bad-uuid-job",
		UserID: uuid.New().String(),
		Data: map[string]interface{}{
			"asset_id": "not-a-valid-uuid",
		},
		CreatedAt: time.Now(),
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)
	task := asynq.NewTask(string(JobTypeMetadataExtraction), payloadBytes)

	err = handlers.HandleMetadataExtraction(ctx, task)
	require.Error(t, err, "expected an error for an invalid asset UUID")
	assert.Contains(t, err.Error(), "invalid asset UUID", "error message should mention invalid UUID")
}

// TestIntegration_HandleMetadataExtraction_MissingAssetID verifies that the
// handler returns a proper error when the task payload is missing the
// asset_id key entirely.
func TestIntegration_HandleMetadataExtraction_MissingAssetID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "immich-meta-missing-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	storageSvc := newLocalStorageService(t, tmpDir)
	handlers := NewHandlers(tdb.Queries, nil, nil, storageSvc)

	payload := JobPayload{
		ID:        "missing-asset-job",
		UserID:    uuid.New().String(),
		Data:      map[string]interface{}{}, // no "asset_id" key
		CreatedAt: time.Now(),
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)
	task := asynq.NewTask(string(JobTypeMetadataExtraction), payloadBytes)

	err = handlers.HandleMetadataExtraction(ctx, task)
	require.Error(t, err, "expected an error when asset_id is missing from payload")
	assert.Contains(t, err.Error(), "invalid asset_id", "error message should mention invalid asset_id")
}

// TestIntegration_HandleMetadataExtraction_AssetNotInDB verifies that the
// handler returns an error when the asset_id is a valid UUID but the asset
// does not exist in the database.
func TestIntegration_HandleMetadataExtraction_AssetNotInDB(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "immich-meta-notfound-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	storageSvc := newLocalStorageService(t, tmpDir)
	handlers := NewHandlers(tdb.Queries, nil, nil, storageSvc)

	nonExistentAssetID := uuid.New()
	payload := JobPayload{
		ID:     "nonexistent-asset-job",
		UserID: uuid.New().String(),
		Data: map[string]interface{}{
			"asset_id": nonExistentAssetID.String(),
		},
		CreatedAt: time.Now(),
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)
	task := asynq.NewTask(string(JobTypeMetadataExtraction), payloadBytes)

	err = handlers.HandleMetadataExtraction(ctx, task)
	require.Error(t, err, "expected an error when the asset does not exist in the DB")
}

// TestIntegration_HandleThumbnailGeneration_NonImageAsset verifies that the
// handler skips thumbnail generation for non-image assets and returns no error.
func TestIntegration_HandleThumbnailGeneration_NonImageAsset(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "immich-thumb-skip-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	storageService := newLocalStorageService(t, tmpDir)

	// Create a user.
	userID := uuid.New()
	userUUID := pgtype.UUID{}
	require.NoError(t, userUUID.Scan(userID.String()))

	_, err = tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    "skiptest@example.com",
		Name:     "Skip Test User",
		Password: "hashed-password-placeholder",
		IsAdmin:  false,
	})
	require.NoError(t, err)

	// Create a VIDEO asset (no file needs to be in storage since handler exits early).
	assetID := uuid.New()
	assetUUID := pgtype.UUID{}
	require.NoError(t, assetUUID.Scan(assetID.String()))

	now := time.Now()
	nowPg := pgtype.Timestamptz{Time: now, Valid: true}

	asset, err := tdb.Queries.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    "device-" + assetID.String(),
		OwnerId:          userUUID,
		DeviceId:         "test-device",
		Type:             string(assets.AssetTypeVideo),
		OriginalPath:     "uploads/" + userID.String() + "/video.mp4",
		FileCreatedAt:    nowPg,
		FileModifiedAt:   nowPg,
		LocalDateTime:    nowPg,
		OriginalFileName: "video.mp4",
		Checksum:         []byte("fakechecksum"),
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	require.NoError(t, err)

	payload := JobPayload{
		ID:        "test-skip-job-" + assetID.String(),
		UserID:    userID.String(),
		Data:      map[string]interface{}{"asset_id": assetID.String()},
		CreatedAt: time.Now(),
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	task := asynq.NewTask(string(JobTypeThumbnailGeneration), payloadBytes)

	handlers := NewHandlers(tdb.Queries, nil, nil, storageService)
	err = handlers.HandleThumbnailGeneration(ctx, task)
	require.NoError(t, err, "HandleThumbnailGeneration should not error for non-image asset")

	// No thumbnail files should have been created.
	assetFiles, err := tdb.Queries.GetAssetFiles(ctx, asset.ID)
	require.NoError(t, err)
	assert.Empty(t, assetFiles, "no thumbnail records should exist for a video asset")
}
