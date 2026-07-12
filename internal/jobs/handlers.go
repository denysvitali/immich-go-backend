package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"

	"github.com/denysvitali/immich-go-backend/internal/assets"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/ffmpeg"
	"github.com/denysvitali/immich-go-backend/internal/libraries"
	"github.com/denysvitali/immich-go-backend/internal/ml"
	"github.com/denysvitali/immich-go-backend/internal/storage"
)

// Handlers contains all job handlers
type Handlers struct {
	db             *sqlc.Queries
	assetService   *assets.Service
	libraryService *libraries.Service
	storageService *storage.Service
	mlClient       *ml.Client
	config         *config.Config
	logger         *logrus.Logger
}

// NewHandlers creates new job handlers. mlClient and cfg may be nil (ML jobs skip).
func NewHandlers(
	db *sqlc.Queries,
	assetService *assets.Service,
	libraryService *libraries.Service,
	storageService *storage.Service,
	mlClient *ml.Client,
	cfg *config.Config,
) *Handlers {
	return &Handlers{
		db:             db,
		assetService:   assetService,
		libraryService: libraryService,
		storageService: storageService,
		mlClient:       mlClient,
		config:         cfg,
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
	var payload ThumbnailGenerationPayload
	if err := unmarshalTypedPayload(task, &payload); err != nil {
		return err
	}

	assetID, err := uuid.Parse(payload.AssetID)
	if err != nil {
		return fmt.Errorf("invalid asset UUID: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"asset_id": assetID,
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
	var payload MetadataExtractionPayload
	if err := unmarshalTypedPayload(task, &payload); err != nil {
		return err
	}

	assetID, err := uuid.Parse(payload.AssetID)
	if err != nil {
		return fmt.Errorf("invalid asset UUID: %w", err)
	}

	pgAssetID := pgtype.UUID{Bytes: assetID, Valid: true}

	log := h.logger.WithFields(logrus.Fields{
		"asset_id": assetID,
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

	// Mirror assets.Service.updateAssetMetadata: the timeline buckets group
	// by assets."localDateTime", so the EXIF capture date must be written
	// there too. This handler and the inline TriggerProcessing path are
	// parallel implementations — date handling must stay in sync.
	if meta.DateTaken != nil {
		if err := h.db.UpdateAssetLocalDateTime(ctx, sqlc.UpdateAssetLocalDateTimeParams{
			ID:            pgAssetID,
			LocalDateTime: pgtype.Timestamptz{Time: *meta.DateTaken, Valid: true},
		}); err != nil {
			return fmt.Errorf("failed to update asset timeline date for asset %s: %w", assetID, err)
		}
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
	if err := h.markAssetJobStatus(ctx, sqlc.UpdateAssetJobStatusParams{
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
	UserID       string `json:"user_id"`
	FullScan     bool   `json:"full_scan"`
	ForceRefresh bool   `json:"force_refresh"`
}

// HandleLibraryScan processes library scanning jobs
func (h *Handlers) HandleLibraryScan(ctx context.Context, task *asynq.Task) error {
	var payload LibraryScanPayload
	if err := unmarshalTypedPayload(task, &payload); err != nil {
		return err
	}

	libraryID, err := uuid.Parse(payload.LibraryID)
	if err != nil {
		return fmt.Errorf("invalid library UUID: %w", err)
	}
	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		return fmt.Errorf("invalid user UUID in payload: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"library_id":    libraryID,
		"user_id":       userID,
		"full_scan":     payload.FullScan,
		"force_refresh": payload.ForceRefresh,
	}).Info("Scanning library")

	// Perform library scan
	_, err = h.libraryService.ScanLibrary(ctx, libraryID, userID, payload.FullScan, payload.ForceRefresh)
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
	var payload VideoTranscodePayload
	if err := unmarshalTypedPayload(task, &payload); err != nil {
		return err
	}

	assetID, err := uuid.Parse(payload.AssetID)
	if err != nil {
		return fmt.Errorf("invalid asset UUID: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"asset_id": assetID,
		"quality":  payload.Quality,
		"format":   payload.Format,
	}).Info("Transcoding video")

	asset, err := h.db.GetAsset(ctx, pgtype.UUID{Bytes: assetID, Valid: true})
	if err != nil {
		return fmt.Errorf("failed to get asset %s: %w", assetID, err)
	}
	if !strings.EqualFold(asset.Type, string(assets.AssetTypeVideo)) {
		h.logger.WithFields(logrus.Fields{
			"asset_id":   asset.ID,
			"asset_type": asset.Type,
		}).Info("Skipping video transcode for non-video asset")
		return nil
	}

	// If ffmpeg is not available, skip gracefully (no infinite retry)
	if !ffmpeg.IsAvailable() {
		h.logger.WithField("asset_id", assetID).Warn("ffmpeg not available, skipping video transcode")
		return nil
	}

	// 1. Download original video to a temp file
	tmpFile, err := os.CreateTemp("", "video-transcode-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file for transcode: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	reader, err := h.storageService.Download(ctx, asset.OriginalPath)
	if err != nil {
		return fmt.Errorf("failed to download original video for transcode: %w", err)
	}
	defer reader.Close()

	if _, err := io.Copy(tmpFile, reader); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write video to temp file: %w", err)
	}
	tmpFile.Close()

	// 2. Determine output path: encoded-videos/<assetID>.mp4 under the same storage root
	// We derive the path from the asset's original path to keep it deterministic
	encodedPath := h.encodedVideoPath(asset.OriginalPath, assetID.String())

	// 3. Run ffmpeg transcode to H.264 MP4
	tmpOutput, err := os.CreateTemp("", "video-output-*.mp4")
	if err != nil {
		return fmt.Errorf("failed to create temp output file: %w", err)
	}
	tmpOutputPath := tmpOutput.Name()
	tmpOutput.Close()
	defer os.Remove(tmpOutputPath)

	opts := ffmpeg.DefaultTranscodeOptions()
	if payload.Quality != "" {
		// Map quality string to CRF (lower = better)
		switch payload.Quality {
		case "original":
			opts.CRF = 18
		case "high":
			opts.CRF = 21
		case "medium":
			opts.CRF = 23
		case "low":
			opts.CRF = 28
		}
	}

	if err := ffmpeg.TranscodeToH264(ctx, tmpPath, tmpOutputPath, opts); err != nil {
		return fmt.Errorf("failed to transcode video %s: %w", assetID, err)
	}

	// 4. Upload output to storage
	outputFile, err := os.Open(tmpOutputPath)
	if err != nil {
		return fmt.Errorf("failed to open transcoded output: %w", err)
	}
	defer outputFile.Close()

	if err := h.storageService.Upload(ctx, encodedPath, outputFile, "video/mp4"); err != nil {
		return fmt.Errorf("failed to upload transcoded video: %w", err)
	}

	// 5. Update asset encodedVideoPath in database
	pgAssetID := pgtype.UUID{Bytes: assetID, Valid: true}
	_, err = h.db.UpdateAssetEncodedVideoPath(ctx, sqlc.UpdateAssetEncodedVideoPathParams{
		ID:               pgAssetID,
		EncodedVideoPath: pgtype.Text{String: encodedPath, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to update asset encoded video path: %w", err)
	}

	// 6. Record asset file of type 'encoded_video'
	_, err = h.db.CreateAssetFile(ctx, sqlc.CreateAssetFileParams{
		AssetId: pgAssetID,
		Type:    "encoded-video",
		Path:    encodedPath,
	})
	if err != nil {
		h.logger.WithError(err).WithFields(logrus.Fields{
			"asset_id": assetID,
			"path":     encodedPath,
		}).Warn("Failed to record encoded video asset file")
		// Non-fatal: the asset already has the encodedVideoPath set
	}

	// 7. Update asset job status to record that video conversion finished
	now := time.Now()
	if err := h.markAssetJobStatus(ctx, sqlc.UpdateAssetJobStatusParams{
		AssetId:   pgAssetID,
		PreviewAt: pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		h.logger.WithError(err).Warn("Failed to update asset job status after video transcode")
	}

	h.logger.WithFields(logrus.Fields{
		"asset_id":     assetID,
		"encoded_path": encodedPath,
	}).Info("Video transcode complete")

	return nil
}

// encodedVideoPath derives a deterministic storage path for the encoded video.
// It places the encoded file in an "encoded-videos" directory at the same level
// as the original asset's directory.
func (h *Handlers) encodedVideoPath(originalPath, assetID string) string {
	// Get the directory of the original asset
	dir := filepath.Dir(originalPath)
	// Place encoded video in a sibling directory
	encodedDir := filepath.Join(filepath.Dir(dir), "encoded-videos")
	return filepath.Join(encodedDir, assetID+".mp4")
}

// FaceDetectionPayload contains data for face detection
type FaceDetectionPayload struct {
	AssetID string `json:"asset_id"`
}

// HandleFaceDetection processes face detection jobs via the Immich ML service.
// When ML/face recognition is disabled the job is skipped (success).
// When ML is enabled but unreachable the error is returned so asynq can retry.
func (h *Handlers) HandleFaceDetection(ctx context.Context, task *asynq.Task) error {
	var payload FaceDetectionPayload
	if err := unmarshalTypedPayload(task, &payload); err != nil {
		return err
	}

	assetID, err := uuid.Parse(payload.AssetID)
	if err != nil {
		return fmt.Errorf("invalid asset UUID: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"asset_id": assetID,
	}).Info("Detecting faces")

	if h.config == nil || !h.config.FaceRecognitionActive() || h.mlClient == nil || !h.mlClient.Enabled() {
		h.logger.WithField("asset_id", assetID).Info("Skipping face detection: ML/face recognition disabled")
		return nil
	}

	assetUUID := pgtype.UUID{Bytes: assetID, Valid: true}
	asset, err := h.db.GetAsset(ctx, assetUUID)
	if err != nil {
		return fmt.Errorf("failed to get asset %s: %w", assetID, err)
	}
	if !strings.EqualFold(asset.Type, string(assets.AssetTypeImage)) {
		h.logger.WithFields(logrus.Fields{
			"asset_id":   asset.ID,
			"asset_type": asset.Type,
		}).Info("Skipping face detection for non-image asset")
		return nil
	}

	imageBytes, err := h.loadAssetImageBytes(ctx, asset)
	if err != nil {
		return err
	}

	fr := h.config.MachineLearning.FacialRecognition
	detection, err := h.mlClient.DetectFaces(ctx, imageBytes, fr.ModelName, fr.MinScore)
	if err != nil {
		if errors.Is(err, ml.ErrDisabled) {
			return nil
		}
		return fmt.Errorf("face detection ML call: %w", err)
	}

	// Replace previous ML-sourced faces for this asset.
	if err := h.db.DeleteAssetFacesByAsset(ctx, assetUUID); err != nil {
		return fmt.Errorf("clear existing faces: %w", err)
	}

	imgW := int32(detection.ImageWidth)
	imgH := int32(detection.ImageHeight)
	for _, face := range detection.Faces {
		created, err := h.db.CreateAssetFace(ctx, sqlc.CreateAssetFaceParams{
			AssetId:       assetUUID,
			PersonId:      pgtype.UUID{}, // unassigned until recognition
			ImageWidth:    imgW,
			ImageHeight:   imgH,
			BoundingBoxX1: face.BoundingBox.X1,
			BoundingBoxY1: face.BoundingBox.Y1,
			BoundingBoxX2: face.BoundingBox.X2,
			BoundingBoxY2: face.BoundingBox.Y2,
		})
		if err != nil {
			return fmt.Errorf("create asset face: %w", err)
		}

		vector := face.EmbeddingRaw
		if vector == "" {
			vector = ml.FormatVector(face.Embedding)
		}
		if _, err := h.db.UpsertFaceSearch(ctx, sqlc.UpsertFaceSearchParams{
			FaceId:    created.ID,
			Embedding: vector,
		}); err != nil {
			return fmt.Errorf("store face embedding: %w", err)
		}

		// Best-effort immediate person match against existing labeled faces.
		if err := h.tryAssignPersonByEmbedding(ctx, created.ID, vector, fr.MaxDistance); err != nil {
			h.logger.WithError(err).WithField("face_id", created.ID).Debug("face person match skipped")
		}
	}

	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	if err := h.markAssetJobStatus(ctx, sqlc.UpdateAssetJobStatusParams{
		AssetId:           assetUUID,
		FacesRecognizedAt: now,
	}); err != nil {
		return fmt.Errorf("update face job status: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"asset_id":    assetID,
		"face_count":  len(detection.Faces),
		"image_width": detection.ImageWidth,
	}).Info("Face detection complete")
	return nil
}

// FaceRecognitionPayload contains data for recognizing a single face.
type FaceRecognitionPayload struct {
	FaceID string `json:"face_id"`
}

// HandleFaceRecognition assigns an unassigned face to a person by embedding
// similarity when a close match already has a person label.
func (h *Handlers) HandleFaceRecognition(ctx context.Context, task *asynq.Task) error {
	var payload FaceRecognitionPayload
	if err := unmarshalTypedPayload(task, &payload); err != nil {
		return err
	}
	if payload.FaceID == "" {
		return fmt.Errorf("face_id is required")
	}

	if h.config == nil || !h.config.FaceRecognitionActive() || h.mlClient == nil {
		h.logger.WithField("face_id", payload.FaceID).Info("Skipping face recognition: disabled")
		return nil
	}

	faceID, err := uuid.Parse(payload.FaceID)
	if err != nil {
		return fmt.Errorf("invalid face UUID: %w", err)
	}
	faceUUID := pgtype.UUID{Bytes: faceID, Valid: true}

	rows, err := h.db.GetFaceSearch(ctx, faceUUID)
	if err != nil {
		return fmt.Errorf("get face embedding: %w", err)
	}
	if len(rows) == 0 {
		h.logger.WithField("face_id", faceID).Info("No embedding for face, skipping recognition")
		return nil
	}

	maxDistance := h.config.MachineLearning.FacialRecognition.MaxDistance
	return h.tryAssignPersonByEmbedding(ctx, faceUUID, rows[0].Embedding, maxDistance)
}

func (h *Handlers) tryAssignPersonByEmbedding(ctx context.Context, faceID pgtype.UUID, embedding any, maxDistance float64) error {
	if maxDistance <= 0 {
		maxDistance = 0.5
	}
	matches, err := h.db.SearchFacesByEmbedding(ctx, sqlc.SearchFacesByEmbeddingParams{
		Embedding:   embedding,
		MaxDistance: maxDistance,
		ResultLimit: 5,
	})
	if err != nil {
		return err
	}
	for _, match := range matches {
		if match.FaceId == faceID {
			continue
		}
		if !match.PersonID.Valid {
			continue
		}
		if _, err := h.db.UpdateAssetFace(ctx, sqlc.UpdateAssetFaceParams{
			ID:       faceID,
			PersonID: match.PersonID,
		}); err != nil {
			return fmt.Errorf("assign person to face: %w", err)
		}
		h.logger.WithFields(logrus.Fields{
			"face_id":   faceID,
			"person_id": match.PersonID,
		}).Info("Assigned face to person by embedding match")
		return nil
	}
	return nil
}

// SmartSearchIndexPayload contains data for smart search indexing
type SmartSearchIndexPayload struct {
	AssetID string `json:"asset_id"`
}

// HandleSmartSearchIndex encodes an asset with CLIP and upserts smart_search.
func (h *Handlers) HandleSmartSearchIndex(ctx context.Context, task *asynq.Task) error {
	var payload SmartSearchIndexPayload
	if err := unmarshalTypedPayload(task, &payload); err != nil {
		return err
	}

	assetID, err := uuid.Parse(payload.AssetID)
	if err != nil {
		return fmt.Errorf("invalid asset UUID: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"asset_id": assetID,
	}).Info("Indexing for smart search")

	if h.config == nil || !h.config.CLIPActive() || h.mlClient == nil || !h.mlClient.Enabled() {
		h.logger.WithField("asset_id", assetID).Info("Skipping smart search index: CLIP/ML disabled")
		return nil
	}

	assetUUID := pgtype.UUID{Bytes: assetID, Valid: true}
	asset, err := h.db.GetAsset(ctx, assetUUID)
	if err != nil {
		return fmt.Errorf("failed to get asset %s: %w", assetID, err)
	}
	if !strings.EqualFold(asset.Type, string(assets.AssetTypeImage)) {
		h.logger.WithFields(logrus.Fields{
			"asset_id":   asset.ID,
			"asset_type": asset.Type,
		}).Info("Skipping smart search for non-image asset")
		return nil
	}

	imageBytes, err := h.loadAssetImageBytes(ctx, asset)
	if err != nil {
		return err
	}

	model := h.config.MachineLearning.Clip.ModelName
	embedding, err := h.mlClient.EncodeImage(ctx, imageBytes, model)
	if err != nil {
		if errors.Is(err, ml.ErrDisabled) {
			return nil
		}
		return fmt.Errorf("CLIP encode image: %w", err)
	}

	if _, err := h.db.UpsertSmartSearch(ctx, sqlc.UpsertSmartSearchParams{
		AssetId:   assetUUID,
		Embedding: ml.FormatVector(embedding),
	}); err != nil {
		return fmt.Errorf("upsert smart search embedding: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"asset_id":       assetID,
		"embedding_dims": len(embedding),
	}).Info("Smart search indexing complete")
	return nil
}

// DuplicateDetectionPayload contains data for duplicate detection
type DuplicateDetectionPayload struct {
	UserID string `json:"user_id"`
}

// HandleDuplicateDetection processes duplicate detection jobs.
// Always runs checksum-based pairing; when CLIP is active also groups near-
// duplicate embeddings under MachineLearning.DuplicateDetection.MaxDistance.
func (h *Handlers) HandleDuplicateDetection(ctx context.Context, task *asynq.Task) error {
	var payload DuplicateDetectionPayload
	if err := unmarshalTypedPayload(task, &payload); err != nil {
		return err
	}

	userID, err := uuid.Parse(payload.UserID)
	if err != nil {
		return fmt.Errorf("invalid user UUID: %w", err)
	}

	h.logger.WithFields(logrus.Fields{
		"user_id": userID,
	}).Info("Detecting duplicates")

	userUUID := pgtype.UUID{Bytes: userID, Valid: true}
	duplicates, err := h.db.GetDuplicateAssets(ctx, userUUID)
	if err != nil {
		return fmt.Errorf("failed to query duplicate assets: %w", err)
	}

	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	seen := make(map[pgtype.UUID]struct{}, len(duplicates)*2)
	for _, duplicate := range duplicates {
		// Share a group id (first asset id) across checksum pairs.
		groupID := duplicate.ID
		for _, assetID := range []pgtype.UUID{duplicate.ID, duplicate.DuplicateID} {
			if !assetID.Valid {
				continue
			}
			if _, ok := seen[assetID]; ok {
				continue
			}
			seen[assetID] = struct{}{}
			_ = h.db.SetAssetDuplicateId(ctx, sqlc.SetAssetDuplicateIdParams{
				ID:          assetID,
				DuplicateId: groupID,
			})
			if err := h.markAssetJobStatus(ctx, sqlc.UpdateAssetJobStatusParams{
				AssetId:              assetID,
				DuplicatesDetectedAt: now,
			}); err != nil {
				return fmt.Errorf("failed to update duplicate job status for asset %s: %w", assetID.String(), err)
			}
		}
	}

	embeddingPairs := 0
	if h.config != nil && h.config.CLIPActive() && h.config.MachineLearning.DuplicateDetection.Enabled {
		n, err := h.detectEmbeddingDuplicates(ctx, userUUID)
		if err != nil {
			h.logger.WithError(err).Warn("embedding-based duplicate detection failed; checksum results kept")
		} else {
			embeddingPairs = n
		}
	}

	h.logger.WithFields(logrus.Fields{
		"user_id":          userID,
		"duplicate_pairs":  len(duplicates),
		"duplicate_assets": len(seen),
		"embedding_pairs":  embeddingPairs,
	}).Info("Duplicate detection complete")
	return nil
}

// detectEmbeddingDuplicates groups assets whose CLIP embeddings are closer than
// the configured max distance. Returns the number of pairs linked.
func (h *Handlers) detectEmbeddingDuplicates(ctx context.Context, owner pgtype.UUID) (int, error) {
	const maxAssets = 5000
	rows, err := h.db.ListSmartSearchByOwner(ctx, sqlc.ListSmartSearchByOwnerParams{
		OwnerId: owner,
		Limit:   maxAssets,
	})
	if err != nil {
		return 0, err
	}
	if len(rows) < 2 {
		return 0, nil
	}

	maxDist := h.config.MachineLearning.DuplicateDetection.MaxDistance
	if maxDist <= 0 {
		maxDist = 0.01
	}

	// O(n²) over a capped list — fine for modest libraries; large libraries
	// should use the vector index via SearchAssetsByEmbedding per asset.
	type item struct {
		id  pgtype.UUID
		emb []float32
	}
	items := make([]item, 0, len(rows))
	for _, row := range rows {
		emb, err := embeddingFromDB(row.Embedding)
		if err != nil || len(emb) == 0 {
			continue
		}
		items = append(items, item{id: row.AssetId, emb: emb})
	}

	pairs := 0
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if cosineDistance(items[i].emb, items[j].emb) > maxDist {
				continue
			}
			groupID := items[i].id
			for _, id := range []pgtype.UUID{items[i].id, items[j].id} {
				if err := h.db.SetAssetDuplicateId(ctx, sqlc.SetAssetDuplicateIdParams{
					ID:          id,
					DuplicateId: groupID,
				}); err != nil {
					return pairs, err
				}
				_ = h.markAssetJobStatus(ctx, sqlc.UpdateAssetJobStatusParams{
					AssetId:              id,
					DuplicatesDetectedAt: now,
				})
			}
			pairs++
		}
	}
	return pairs, nil
}

func embeddingFromDB(v any) ([]float32, error) {
	switch t := v.(type) {
	case []float32:
		return t, nil
	case []float64:
		out := make([]float32, len(t))
		for i, x := range t {
			out[i] = float32(x)
		}
		return out, nil
	case string:
		return parseVectorString(t)
	case []byte:
		return parseVectorString(string(t))
	default:
		// pgvector may surface as a typed string via fmt
		return parseVectorString(fmt.Sprint(v))
	}
}

func parseVectorString(s string) ([]float32, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "[]" {
		return nil, fmt.Errorf("empty vector")
	}
	// Accept both "[1,2]" and pgvector's "{1,2}" styles.
	s = strings.ReplaceAll(s, "{", "[")
	s = strings.ReplaceAll(s, "}", "]")
	var floats []float64
	if err := json.Unmarshal([]byte(s), &floats); err != nil {
		return nil, err
	}
	out := make([]float32, len(floats))
	for i, f := range floats {
		out[i] = float32(f)
	}
	return out, nil
}

func cosineDistance(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 1
	}
	var dot, na, nb float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		na += av * av
		nb += bv * bv
	}
	if na == 0 || nb == 0 {
		return 1
	}
	// cosine distance = 1 - cosine similarity
	return 1 - (dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

// loadAssetImageBytes prefers the preview thumbnail, then falls back to original.
func (h *Handlers) loadAssetImageBytes(ctx context.Context, asset sqlc.Asset) ([]byte, error) {
	if h.storageService == nil {
		return nil, fmt.Errorf("storage service not configured")
	}

	// Prefer preview thumbnail when available (matches Immich ML pipeline).
	previews, err := h.db.GetAssetFilesByType(ctx, sqlc.GetAssetFilesByTypeParams{
		AssetId: asset.ID,
		Type:    string(assets.ThumbnailTypePreview),
	})
	if err == nil && len(previews) > 0 {
		reader, err := h.storageService.Download(ctx, previews[0].Path)
		if err == nil {
			defer reader.Close()
			data, readErr := io.ReadAll(reader)
			if readErr == nil && len(data) > 0 {
				return data, nil
			}
		}
	}

	reader, err := h.storageService.Download(ctx, asset.OriginalPath)
	if err != nil {
		return nil, fmt.Errorf("download asset for ML: %w", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read asset for ML: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty asset data for ML")
	}
	return data, nil
}

func (h *Handlers) markAssetJobStatus(ctx context.Context, params sqlc.UpdateAssetJobStatusParams) error {
	if _, err := h.db.UpdateAssetJobStatus(ctx, params); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		_, createErr := h.db.CreateAssetJobStatus(ctx, sqlc.CreateAssetJobStatusParams(params))
		if createErr != nil {
			return createErr
		}
	}
	return nil
}

func assetIDFromTask(task *asynq.Task, assetID string) (uuid.UUID, error) {
	if assetID != "" {
		return parseAssetID(assetID)
	}

	payload, err := unmarshalJobPayload(task)
	if err != nil {
		return uuid.Nil, err
	}
	return extractAssetID(payload)
}

func unmarshalJobPayload(task *asynq.Task) (JobPayload, error) {
	var payload JobPayload
	if err := unmarshalTypedPayload(task, &payload); err != nil {
		return JobPayload{}, err
	}
	return payload, nil
}

func extractAssetID(payload JobPayload) (uuid.UUID, error) {
	assetIDStr, ok := payload.Data["asset_id"].(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid asset_id in payload")
	}
	return parseAssetID(assetIDStr)
}

func parseAssetID(assetID string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(assetID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid asset UUID: %w", err)
	}
	return parsed, nil
}

// StorageMigrationPayload contains data for storage migration
type StorageMigrationPayload struct {
	AssetID     string `json:"asset_id"`
	FromStorage string `json:"from_storage"`
	ToStorage   string `json:"to_storage"`
}

// HandleStorageMigration processes storage migration jobs
func (h *Handlers) HandleStorageMigration(ctx context.Context, task *asynq.Task) error {
	var payload StorageMigrationPayload
	if err := unmarshalTypedPayload(task, &payload); err != nil {
		return err
	}

	h.logger.WithFields(logrus.Fields{
		"asset_id":     payload.AssetID,
		"from_storage": payload.FromStorage,
		"to_storage":   payload.ToStorage,
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
	service.RegisterHandler(JobTypeFaceRecognition, h.HandleFaceRecognition)
	service.RegisterHandler(JobTypeSmartSearch, h.HandleSmartSearchIndex)

	// Library management
	service.RegisterHandler(JobTypeLibraryScan, h.HandleLibraryScan)
	service.RegisterHandler(JobTypeDuplicateDetect, h.HandleDuplicateDetection)

	// Storage
	service.RegisterHandler(JobTypeStorageMigration, h.HandleStorageMigration)

	h.logger.Info("All job handlers registered")
}
