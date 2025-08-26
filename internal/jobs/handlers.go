package jobs

import (
	"context"
	"encoding/json"
	"fmt"

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

	// Generate thumbnails using asset service
	sizes := []string{"thumbnail", "preview", "thumbnail_big"}
	for _, size := range sizes {
		err := h.generateThumbnail(ctx, &asset, size)
		if err != nil {
			h.logger.WithError(err).Warnf("Failed to generate %s thumbnail", size)
		}
	}

	return nil
}

// generateThumbnail generates a single thumbnail
func (h *Handlers) generateThumbnail(ctx context.Context, asset *sqlc.Asset, size string) error {
	// This would integrate with the actual thumbnail generation logic
	// For now, we'll just log the operation
	h.logger.WithFields(logrus.Fields{
		"asset_id": asset.ID,
		"size":     size,
	}).Debug("Generating thumbnail")
	
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

	h.logger.WithFields(logrus.Fields{
		"asset_id": assetID,
		"job_id":   payload.ID,
	}).Info("Extracting metadata")

	// Extract metadata using asset service
	// This would call the actual metadata extraction logic
	
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

	// Perform library scan
	// Call library scan with correct parameters
	// TODO: Get userID from context or payload
	userID := uuid.New() // Placeholder
	_, err = h.libraryService.ScanLibrary(ctx, libraryID, userID, fullScan, forceRefresh)
	if err != nil {
		return fmt.Errorf("library scan failed: %w", err)
	}

	return nil
}

// VideoTranscodePayload contains data for video transcoding
type VideoTranscodePayload struct {
	AssetID  string `json:"asset_id"`
	Quality  string `json:"quality"`
	Format   string `json:"format"`
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