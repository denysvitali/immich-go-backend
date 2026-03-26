package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/immich-go-backend/internal/assets"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/libraries"
	"github.com/denysvitali/immich-go-backend/internal/storage"
)

// Handlers contains all job handlers
type Handlers struct {
	db             *sqlc.Queries
	assetService   *assets.Service
	libraryService *libraries.Service
	storageService *storage.Service
	logger         *logrus.Logger
}

// NewHandlers creates new job handlers
func NewHandlers(
	db *sqlc.Queries,
	assetService *assets.Service,
	libraryService *libraries.Service,
	storageService *storage.Service,
) *Handlers {
	return &Handlers{
		db:             db,
		assetService:   assetService,
		libraryService: libraryService,
		storageService: storageService,
		logger:         logrus.StandardLogger(),
	}
}

// ThumbnailGenerationPayload contains data for thumbnail generation
type ThumbnailGenerationPayload struct {
	AssetID string   `json:"asset_id"`
	Sizes   []string `json:"sizes"`
}

// HandleThumbnailGeneration processes thumbnail generation jobs
func (h *Handlers) HandleThumbnailGeneration(ctx context.Context, task *asynq.Task) error {
	var payload JobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	assetIDStr, ok := payload.Data["asset_id"].(string)
	if !ok {
		return fmt.Errorf("invalid asset_id in payload")
	}

	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		return fmt.Errorf("invalid asset UUID: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"asset_id": assetID,
		"job_id":   payload.ID,
	}).Info("Generating thumbnails")

	// Get asset from database
	asset, err := h.db.GetAsset(ctx, pgtype.UUID{Bytes: assetID, Valid: true})
	if err != nil {
		return fmt.Errorf("failed to get asset: %w", err)
	}

	// Only generate thumbnails for image assets
	if !strings.EqualFold(asset.Type, string(assets.AssetTypeImage)) {
		h.logger.WithFields(logrus.Fields{
			"asset_id":   asset.ID,
			"asset_type": asset.Type,
		}).Info("Skipping thumbnail generation for non-image asset")
		return nil
	}

	// Download the original asset from storage
	reader, err := h.storageService.Download(ctx, asset.OriginalPath)
	if err != nil {
		return fmt.Errorf("failed to download original asset: %w", err)
	}
	defer reader.Close()

	// Generate all thumbnail sizes at once
	generator := assets.NewThumbnailGenerator()
	thumbnails, err := generator.GenerateThumbnails(ctx, reader, asset.OriginalFileName)
	if err != nil {
		return fmt.Errorf("failed to generate thumbnails: %w", err)
	}

	// Content type mapping per thumbnail type
	thumbContentType := map[assets.ThumbnailType]string{
		assets.ThumbnailTypePreview: "image/jpeg",
		assets.ThumbnailTypeWebp:    "image/jpeg", // falls back to JPEG encoding
		assets.ThumbnailTypeThumb:   "image/jpeg",
	}

	// Upload each thumbnail and record it in the database
	for thumbType, data := range thumbnails {
		thumbPath := generator.GetThumbnailPath(asset.OriginalPath, thumbType)
		contentType := thumbContentType[thumbType]

		if err := h.storageService.UploadBytes(ctx, thumbPath, data, contentType); err != nil {
			h.logger.WithError(err).WithFields(logrus.Fields{
				"asset_id":   asset.ID,
				"thumb_type": thumbType,
				"thumb_path": thumbPath,
			}).Warn("Failed to upload thumbnail")
			continue
		}

		if _, err := h.db.CreateAssetFile(ctx, sqlc.CreateAssetFileParams{
			AssetId: asset.ID,
			Type:    string(thumbType),
			Path:    thumbPath,
		}); err != nil {
			h.logger.WithError(err).WithFields(logrus.Fields{
				"asset_id":   asset.ID,
				"thumb_type": thumbType,
				"thumb_path": thumbPath,
			}).Warn("Failed to store thumbnail record in database")
			continue
		}

		h.logger.WithFields(logrus.Fields{
			"asset_id":   asset.ID,
			"thumb_type": thumbType,
			"thumb_path": thumbPath,
			"size_bytes": len(data),
		}).Debug("Thumbnail generated and stored")
	}

	return nil
}

// MetadataExtractionPayload contains data for metadata extraction
type MetadataExtractionPayload struct {
	AssetID string `json:"asset_id"`
}

// HandleMetadataExtraction processes metadata extraction jobs
func (h *Handlers) HandleMetadataExtraction(ctx context.Context, task *asynq.Task) error {
	var payload JobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	assetIDStr, ok := payload.Data["asset_id"].(string)
	if !ok {
		return fmt.Errorf("invalid asset_id in payload")
	}

	assetID, err := uuid.Parse(assetIDStr)
	if err != nil {
		return fmt.Errorf("invalid asset UUID: %w", err)
	}

	pgAssetID := pgtype.UUID{Bytes: assetID, Valid: true}

	log := h.logger.WithFields(logrus.Fields{
		"asset_id": assetID,
		"job_id":   payload.ID,
	})
	log.Info("Extracting metadata")

	// 1. Fetch asset from DB.
	asset, err := h.db.GetAsset(ctx, pgAssetID)
	if err != nil {
		return fmt.Errorf("failed to get asset %s: %w", assetID, err)
	}

	// 2. Get file metadata from storage (size, content type).
	fileMeta, err := h.storageService.GetAssetMetadata(ctx, asset.OriginalPath)
	if err != nil {
		return fmt.Errorf("failed to get storage metadata for asset %s: %w", assetID, err)
	}
	fileSize := fileMeta.Size

	// 3. Determine content type from the filename extension.
	contentType := mime.TypeByExtension(filepath.Ext(asset.OriginalFileName))
	if contentType == "" {
		// Fall back to whatever the storage backend reported.
		contentType = fileMeta.ContentType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 4. Download the file from storage.
	reader, err := h.storageService.Download(ctx, asset.OriginalPath)
	if err != nil {
		return fmt.Errorf("failed to download asset %s for metadata extraction: %w", assetID, err)
	}
	defer reader.Close()

	// 5. Extract metadata.
	extractor := assets.NewMetadataExtractor()
	meta, err := extractor.ExtractMetadata(ctx, reader, asset.OriginalFileName, contentType, fileSize)
	if err != nil {
		// ExtractMetadata itself never returns a hard error — but guard anyway.
		log.WithError(err).Warn("Metadata extraction returned an error; continuing with partial data")
	}

	log.WithFields(logrus.Fields{
		"content_type": contentType,
		"file_size":    fileSize,
		"has_exif":     meta.DateTaken != nil || meta.Make != nil || meta.Width != nil,
	}).Debug("Metadata extracted")

	// 6. Build SQLC params and write EXIF data to DB.
	exifParams := sqlc.CreateOrUpdateExifParams{
		AssetId:     pgAssetID,
		Description: "", // non-nullable in schema; populated below if present
	}

	if meta.Make != nil {
		exifParams.Make = pgtype.Text{String: *meta.Make, Valid: true}
	}
	if meta.Model != nil {
		exifParams.Model = pgtype.Text{String: *meta.Model, Valid: true}
	}
	if meta.Width != nil {
		exifParams.ExifImageWidth = pgtype.Int4{Int32: *meta.Width, Valid: true}
	}
	if meta.Height != nil {
		exifParams.ExifImageHeight = pgtype.Int4{Int32: *meta.Height, Valid: true}
	}
	exifParams.FileSizeInByte = pgtype.Int8{Int64: fileSize, Valid: true}
	if meta.LensModel != nil {
		exifParams.LensModel = pgtype.Text{String: *meta.LensModel, Valid: true}
	}
	if meta.FNumber != nil {
		exifParams.FNumber = pgtype.Float8{Float64: *meta.FNumber, Valid: true}
	}
	if meta.FocalLength != nil {
		exifParams.FocalLength = pgtype.Float8{Float64: *meta.FocalLength, Valid: true}
	}
	if meta.ISO != nil {
		exifParams.Iso = pgtype.Int4{Int32: *meta.ISO, Valid: true}
	}
	if meta.Latitude != nil {
		exifParams.Latitude = pgtype.Float8{Float64: *meta.Latitude, Valid: true}
	}
	if meta.Longitude != nil {
		exifParams.Longitude = pgtype.Float8{Float64: *meta.Longitude, Valid: true}
	}
	if meta.City != nil {
		exifParams.City = pgtype.Text{String: *meta.City, Valid: true}
	}
	if meta.State != nil {
		exifParams.State = pgtype.Text{String: *meta.State, Valid: true}
	}
	if meta.Country != nil {
		exifParams.Country = pgtype.Text{String: *meta.Country, Valid: true}
	}
	if meta.Description != nil {
		exifParams.Description = *meta.Description
	}
	if meta.DateTaken != nil {
		exifParams.DateTimeOriginal = pgtype.Timestamptz{Time: *meta.DateTaken, Valid: true}
		exifParams.ModifyDate = pgtype.Timestamptz{Time: *meta.DateTaken, Valid: true}
	}

	if _, err := h.db.CreateOrUpdateExif(ctx, exifParams); err != nil {
		return fmt.Errorf("failed to persist EXIF data for asset %s: %w", assetID, err)
	}

	log.WithFields(logrus.Fields{
		"make":       meta.Make,
		"model":      meta.Model,
		"date_taken": meta.DateTaken,
		"width":      meta.Width,
		"height":     meta.Height,
		"latitude":   meta.Latitude,
		"longitude":  meta.Longitude,
	}).Info("Metadata extraction complete")

	// 7. Update the asset job status to record that metadata extraction finished.
	now := time.Now()
	if _, err := h.db.UpdateAssetJobStatus(ctx, sqlc.UpdateAssetJobStatusParams{
		AssetId:             pgAssetID,
		MetadataExtractedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		// Non-fatal: the EXIF data was already written; just warn.
		log.WithError(err).Warn("Failed to update asset job status after metadata extraction")
	}

	return nil
}

// LibraryScanPayload contains data for library scanning
type LibraryScanPayload struct {
	LibraryID    string `json:"library_id"`
	FullScan     bool   `json:"full_scan"`
	ForceRefresh bool   `json:"force_refresh"`
}

// HandleLibraryScan processes library scanning jobs
func (h *Handlers) HandleLibraryScan(ctx context.Context, task *asynq.Task) error {
	var payload JobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	libraryIDStr, ok := payload.Data["library_id"].(string)
	if !ok {
		return fmt.Errorf("invalid library_id in payload")
	}

	libraryID, err := uuid.Parse(libraryIDStr)
	if err != nil {
		return fmt.Errorf("invalid library UUID: %w", err)
	}

	fullScan, _ := payload.Data["full_scan"].(bool)
	forceRefresh, _ := payload.Data["force_refresh"].(bool)

	h.logger.WithFields(logrus.Fields{
		"library_id":    libraryID,
		"full_scan":     fullScan,
		"force_refresh": forceRefresh,
		"job_id":        payload.ID,
	}).Info("Scanning library")

	// Get userID from payload
	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		return fmt.Errorf("invalid user UUID in payload: %w", err)
	}

	// Perform library scan
	_, err = h.libraryService.ScanLibrary(ctx, libraryID, userID, fullScan, forceRefresh)
	if err != nil {
		return fmt.Errorf("library scan failed: %w", err)
	}

	return nil
}

// VideoTranscodePayload contains data for video transcoding
type VideoTranscodePayload struct {
	AssetID string `json:"asset_id"`
	Quality string `json:"quality"`
	Format  string `json:"format"`
}

// HandleVideoTranscode processes video transcoding jobs
func (h *Handlers) HandleVideoTranscode(ctx context.Context, task *asynq.Task) error {
	var payload JobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	assetIDStr, ok := payload.Data["asset_id"].(string)
	if !ok {
		return fmt.Errorf("invalid asset_id in payload")
	}

	quality, _ := payload.Data["quality"].(string)
	format, _ := payload.Data["format"].(string)

	h.logger.WithFields(logrus.Fields{
		"asset_id": assetIDStr,
		"quality":  quality,
		"format":   format,
		"job_id":   payload.ID,
	}).Info("Transcoding video")

	// Video transcoding logic would go here
	// This would typically use ffmpeg or similar tools

	return nil
}

// FaceDetectionPayload contains data for face detection
type FaceDetectionPayload struct {
	AssetID string `json:"asset_id"`
}

// HandleFaceDetection processes face detection jobs
func (h *Handlers) HandleFaceDetection(ctx context.Context, task *asynq.Task) error {
	var payload JobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	assetIDStr, ok := payload.Data["asset_id"].(string)
	if !ok {
		return fmt.Errorf("invalid asset_id in payload")
	}

	h.logger.WithFields(logrus.Fields{
		"asset_id": assetIDStr,
		"job_id":   payload.ID,
	}).Info("Detecting faces")

	// Face detection logic would go here
	// This would integrate with ML models for face detection

	return nil
}

// SmartSearchIndexPayload contains data for smart search indexing
type SmartSearchIndexPayload struct {
	AssetID string `json:"asset_id"`
}

// HandleSmartSearchIndex processes smart search indexing jobs
func (h *Handlers) HandleSmartSearchIndex(ctx context.Context, task *asynq.Task) error {
	var payload JobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	assetIDStr, ok := payload.Data["asset_id"].(string)
	if !ok {
		return fmt.Errorf("invalid asset_id in payload")
	}

	h.logger.WithFields(logrus.Fields{
		"asset_id": assetIDStr,
		"job_id":   payload.ID,
	}).Info("Indexing for smart search")

	// Smart search indexing logic would go here
	// This would use CLIP or similar models for embeddings

	return nil
}

// DuplicateDetectionPayload contains data for duplicate detection
type DuplicateDetectionPayload struct {
	UserID string `json:"user_id"`
}

// HandleDuplicateDetection processes duplicate detection jobs
func (h *Handlers) HandleDuplicateDetection(ctx context.Context, task *asynq.Task) error {
	var payload JobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"user_id": payload.UserID,
		"job_id":  payload.ID,
	}).Info("Detecting duplicates")

	// Duplicate detection logic would go here
	// This would use perceptual hashing or similar techniques

	return nil
}

// StorageMigrationPayload contains data for storage migration
type StorageMigrationPayload struct {
	AssetID     string `json:"asset_id"`
	FromStorage string `json:"from_storage"`
	ToStorage   string `json:"to_storage"`
}

// HandleStorageMigration processes storage migration jobs
func (h *Handlers) HandleStorageMigration(ctx context.Context, task *asynq.Task) error {
	var payload JobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	assetIDStr, _ := payload.Data["asset_id"].(string)
	fromStorage, _ := payload.Data["from_storage"].(string)
	toStorage, _ := payload.Data["to_storage"].(string)

	h.logger.WithFields(logrus.Fields{
		"asset_id":     assetIDStr,
		"from_storage": fromStorage,
		"to_storage":   toStorage,
		"job_id":       payload.ID,
	}).Info("Migrating storage")

	// Storage migration logic would go here
	// This would move files between different storage backends

	return nil
}

// RegisterAllHandlers registers all job handlers with the service
func (h *Handlers) RegisterAllHandlers(service *Service) {
	// Asset processing
	service.RegisterHandler(JobTypeThumbnailGeneration, h.HandleThumbnailGeneration)
	service.RegisterHandler(JobTypeMetadataExtraction, h.HandleMetadataExtraction)
	service.RegisterHandler(JobTypeVideoTranscode, h.HandleVideoTranscode)

	// Machine learning
	service.RegisterHandler(JobTypeFaceDetection, h.HandleFaceDetection)
	service.RegisterHandler(JobTypeSmartSearch, h.HandleSmartSearchIndex)

	// Library management
	service.RegisterHandler(JobTypeLibraryScan, h.HandleLibraryScan)
	service.RegisterHandler(JobTypeDuplicateDetect, h.HandleDuplicateDetection)

	// Storage
	service.RegisterHandler(JobTypeStorageMigration, h.HandleStorageMigration)

	h.logger.Info("All job handlers registered")
}
