package assets

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/storage"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Service handles asset management operations
type Service struct {
	db                *sqlc.Queries
	storage           *storage.Service
	metadataExtractor *MetadataExtractor
	thumbnailGen      *ThumbnailGenerator
	config            *config.Config

	// Metrics
	uploadCounter   metric.Int64Counter
	downloadCounter metric.Int64Counter
	processingTime  metric.Float64Histogram
	storageSize     metric.Int64UpDownCounter
}

// NewService creates a new asset service
func NewService(queries *sqlc.Queries, storageService *storage.Service, cfg *config.Config) (*Service, error) {
	meter := telemetry.GetMeter()

	uploadCounter, err := meter.Int64Counter(
		"assets_uploads_total",
		metric.WithDescription("Total number of asset uploads"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create upload counter: %w", err)
	}

	downloadCounter, err := meter.Int64Counter(
		"assets_downloads_total",
		metric.WithDescription("Total number of asset downloads"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create download counter: %w", err)
	}

	processingTime, err := meter.Float64Histogram(
		"assets_processing_duration_seconds",
		metric.WithDescription("Time spent processing assets"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create processing time histogram: %w", err)
	}

	storageSize, err := meter.Int64UpDownCounter(
		"assets_storage_bytes",
		metric.WithDescription("Total storage used by assets"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage size counter: %w", err)
	}

	return &Service{
		db:                queries,
		storage:           storageService,
		metadataExtractor: NewMetadataExtractor(),
		thumbnailGen:      NewThumbnailGenerator(),
		config:            cfg,
		uploadCounter:     uploadCounter,
		downloadCounter:   downloadCounter,
		processingTime:    processingTime,
		storageSize:       storageSize,
	}, nil
}

// InitiateUpload initiates an asset upload and returns upload instructions
func (s *Service) InitiateUpload(ctx context.Context, req UploadRequest) (*UploadResponse, error) {
	ctx, span := tracer.Start(ctx, "assets.initiate_upload",
		trace.WithAttributes(
			attribute.String("user_id", req.UserID.String()),
			attribute.String("filename", req.Filename),
			attribute.String("content_type", req.ContentType),
			attribute.Int64("size", req.Size),
		))
	defer span.End()

	// Generate asset ID
	assetID := uuid.New()
	span.SetAttributes(attribute.String("asset_id", assetID.String()))

	// Generate storage path
	assetType := s.getAssetTypeFromContentType(req.ContentType)
	storagePath := s.generateStoragePath(req.UserID, assetID, req.Filename, assetType)

	// Create asset record in database with uploading status
	userUUID, err := stringToUUID(req.UserID.String())
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	asset, err := s.db.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    req.Filename, // Use filename as device asset ID for now
		OwnerId:          userUUID,
		DeviceId:         "go-backend", // Default device ID
		Type:             string(assetType),
		OriginalPath:     storagePath,
		FileCreatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		FileModifiedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
		LocalDateTime:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
		OriginalFileName: req.Filename,
		Checksum:         []byte(req.Checksum),
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline, // Default to timeline
		Status:           sqlc.AssetsStatusEnumActive,
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to create asset record: %w", err)
	}

	// Check if we should use direct S3 upload
	if s.config.Storage.S3.Enabled && s.config.Storage.S3.DirectUpload {
		// Generate pre-signed upload URL
		uploadURL, uploadFields, err := s.storage.GeneratePresignedUploadURL(ctx, storagePath, req.ContentType, time.Hour)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to generate pre-signed URL: %w", err)
		}

		return &UploadResponse{
			AssetID:      asset.ID.Bytes,
			UploadURL:    uploadURL,
			UploadFields: uploadFields,
			DirectUpload: true,
		}, nil
	}

	// For non-S3 or non-direct upload, client uploads to our server
	return &UploadResponse{
		AssetID:      asset.ID.Bytes,
		DirectUpload: false,
	}, nil
}

// CompleteUpload completes the upload process and starts background processing
func (s *Service) CompleteUpload(ctx context.Context, assetID uuid.UUID, reader io.Reader) error {
	ctx, span := tracer.Start(ctx, "assets.complete_upload",
		trace.WithAttributes(
			attribute.String("asset_id", assetID.String()),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.processingTime.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "upload")))
	}()

	// Get asset record
	assetUUID, err := stringToUUID(assetID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid asset ID: %w", err)
	}

	asset, err := s.db.GetAssetByID(ctx, assetUUID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get asset: %w", err)
	}

	// Upload file to storage if not using direct upload
	if !s.config.Storage.S3.DirectUpload {
		contentType := s.getMimeTypeFromAssetType(asset.Type)
		err = s.storage.Upload(ctx, asset.OriginalPath, reader, contentType)
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to upload file: %w", err)
		}
	}

	// Update asset status to active
	_, err = s.db.UpdateAssetStatus(ctx, sqlc.UpdateAssetStatusParams{
		ID:     asset.ID,
		Status: sqlc.AssetsStatusEnumActive,
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update asset status: %w", err)
	}

	// Start background processing
	go s.processAsset(context.Background(), assetID)

	s.uploadCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("user_id", uuidToString(asset.OwnerId)),
			attribute.String("type", asset.Type),
		))

	// TODO: Add storage size metric when size is available
	// s.storageSize.Add(ctx, size, metric.WithAttributes(attribute.String("operation", "upload")))

	return nil
}

// processAsset handles background processing of an uploaded asset
func (s *Service) processAsset(ctx context.Context, assetID uuid.UUID) {
	ctx, span := tracer.Start(ctx, "assets.process_asset",
		trace.WithAttributes(
			attribute.String("asset_id", assetID.String()),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.processingTime.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "process")))
	}()

	assetUUID, err := stringToUUID(assetID.String())
	if err != nil {
		span.RecordError(err)
		return
	}

	// Get asset record
	asset, err := s.db.GetAssetByID(ctx, assetUUID)
	if err != nil {
		span.RecordError(err)
		return
	}

	// Download file for processing
	reader, err := s.storage.Download(ctx, asset.OriginalPath)
	if err != nil {
		span.RecordError(err)
		s.markAssetFailed(ctx, assetUUID, fmt.Sprintf("failed to download for processing: %v", err))
		return
	}
	defer reader.Close()

	// Extract metadata
	// TODO: Store MIME type and size in database or derive from file
	mimeType := s.getMimeTypeFromAssetType(asset.Type)
	metadata, err := s.metadataExtractor.ExtractMetadata(ctx, reader, asset.OriginalFileName, mimeType, 0)
	if err != nil {
		span.RecordError(err)
		// Continue processing even if metadata extraction fails
	}

	// Update asset with metadata
	if metadata != nil {
		err = s.updateAssetMetadata(ctx, assetUUID, metadata)
		if err != nil {
			span.RecordError(err)
			// Continue processing
		}
	}

	// Generate thumbnails for images
	if s.thumbnailGen.CanGenerateThumbnail(mimeType) {
		err = s.generateAndStoreThumbnails(ctx, assetUUID, asset.OriginalPath, asset.OriginalFileName)
		if err != nil {
			span.RecordError(err)
			// Continue processing even if thumbnail generation fails
		}
	}

	// Mark asset as active
	_, err = s.db.UpdateAssetStatus(ctx, sqlc.UpdateAssetStatusParams{
		ID:     assetUUID,
		Status: sqlc.AssetsStatusEnumActive,
	})
	if err != nil {
		span.RecordError(err)
		s.markAssetFailed(ctx, assetUUID, fmt.Sprintf("failed to mark as active: %v", err))
		return
	}

	span.SetAttributes(attribute.String("status", "completed"))
}

// generateAndStoreThumbnails generates and stores thumbnails for an asset
func (s *Service) generateAndStoreThumbnails(ctx context.Context, assetID pgtype.UUID, originalPath, filename string) error {
	ctx, span := tracer.Start(ctx, "assets.generate_thumbnails",
		trace.WithAttributes(
			attribute.String("asset_id", uuidToString(assetID)),
		))
	defer span.End()

	// Download original file
	reader, err := s.storage.Download(ctx, originalPath)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to download original: %w", err)
	}
	defer reader.Close()

	// Generate thumbnails
	thumbnails, err := s.thumbnailGen.GenerateThumbnails(ctx, reader, filename)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to generate thumbnails: %w", err)
	}

	// Store each thumbnail
	for thumbType, data := range thumbnails {
		thumbPath := s.thumbnailGen.GetThumbnailPath(originalPath, thumbType)

		err = s.storage.UploadBytes(ctx, thumbPath, data, "image/jpeg")
		if err != nil {
			span.RecordError(err)
			continue // Continue with other thumbnails
		}

		// Store thumbnail record in database
		_, err = s.db.CreateAssetFile(ctx, sqlc.CreateAssetFileParams{
			AssetId: assetID,
			Type:    string(thumbType),
			Path:    thumbPath,
		})
		if err != nil {
			span.RecordError(err)
			// Continue with other thumbnails
		}
	}

	span.SetAttributes(attribute.Int("thumbnails_created", len(thumbnails)))
	return nil
}

// updateAssetMetadata updates asset metadata in the database
func (s *Service) updateAssetMetadata(ctx context.Context, assetID pgtype.UUID, metadata *AssetMetadata) error {
	ctx, span := tracer.Start(ctx, "assets.update_metadata")
	defer span.End()

	params := sqlc.CreateOrUpdateExifParams{
		AssetId: assetID,
	}

	// Convert metadata to database format
	if metadata.DateTaken != nil {
		dateTaken, err := timeToTimestamptz(*metadata.DateTaken)
		if err == nil {
			params.DateTimeOriginal = dateTaken
		}
	}

	if metadata.Width != nil {
		params.ExifImageWidth = pgtype.Int4{Int32: *metadata.Width, Valid: true}
	}

	if metadata.Height != nil {
		params.ExifImageHeight = pgtype.Int4{Int32: *metadata.Height, Valid: true}
	}

	if metadata.Make != nil {
		params.Make = pgtype.Text{String: *metadata.Make, Valid: true}
	}

	if metadata.Model != nil {
		params.Model = pgtype.Text{String: *metadata.Model, Valid: true}
	}

	if metadata.LensModel != nil {
		params.LensModel = pgtype.Text{String: *metadata.LensModel, Valid: true}
	}

	if metadata.FNumber != nil {
		params.FNumber = pgtype.Float8{Float64: *metadata.FNumber, Valid: true}
	}

	if metadata.FocalLength != nil {
		params.FocalLength = pgtype.Float8{Float64: *metadata.FocalLength, Valid: true}
	}

	if metadata.ISO != nil {
		params.Iso = pgtype.Int4{Int32: *metadata.ISO, Valid: true}
	}

	if metadata.Latitude != nil {
		params.Latitude = pgtype.Float8{Float64: *metadata.Latitude, Valid: true}
	}

	if metadata.Longitude != nil {
		params.Longitude = pgtype.Float8{Float64: *metadata.Longitude, Valid: true}
	}

	if metadata.Description != nil {
		params.Description = *metadata.Description
	}

	_, err := s.db.CreateOrUpdateExif(ctx, params)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	return nil
}

// markAssetFailed marks an asset as failed with an error message
func (s *Service) markAssetFailed(ctx context.Context, assetID pgtype.UUID, errorMsg string) {
	// In a production system, you'd want to store the error and possibly retry
	// For now, we'll just log and mark as failed
	_, _ = s.db.UpdateAssetStatus(ctx, sqlc.UpdateAssetStatusParams{
		ID:     assetID,
		Status: sqlc.AssetsStatusEnumDeleted, // Use deleted as failed status for now
	})
}

// GetAsset retrieves an asset by ID
func (s *Service) GetAsset(ctx context.Context, assetID uuid.UUID, userID uuid.UUID) (*AssetInfo, error) {
	ctx, span := tracer.Start(ctx, "assets.get_asset",
		trace.WithAttributes(
			attribute.String("asset_id", assetID.String()),
			attribute.String("user_id", userID.String()),
		))
	defer span.End()

	assetUUID, err := stringToUUID(assetID.String())
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("invalid asset ID: %w", err)
	}

	userUUID, err := stringToUUID(userID.String())
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Get asset with user verification
	asset, err := s.db.GetAssetByIDAndUser(ctx, sqlc.GetAssetByIDAndUserParams{
		ID:      assetUUID,
		OwnerId: userUUID,
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}

	// Get thumbnails from asset_files table
	assetFiles, err := s.db.GetAssetFiles(ctx, assetUUID)
	if err != nil {
		span.RecordError(err)
		// Continue without thumbnails
	}

	// Convert asset files to thumbnails
	var thumbnails []AssetThumbnail
	for _, file := range assetFiles {
		// Only include thumbnail files
		if file.Type == string(ThumbnailTypePreview) || file.Type == string(ThumbnailTypeWebp) || file.Type == string(ThumbnailTypeThumb) {
			thumbnails = append(thumbnails, AssetThumbnail{
				AssetID: uuid.MustParse(uuidToString(file.AssetId)),
				Type:    file.Type,
				Path:    file.Path,
				Width:   0, // TODO: Store dimensions in asset_files or calculate
				Height:  0,
				Size:    0, // TODO: Store size in asset_files or calculate
			})
		}
	}

	return s.convertToAssetInfo(asset, thumbnails), nil
}

// DownloadAsset generates a download URL for an asset
func (s *Service) DownloadAsset(ctx context.Context, req DownloadRequest) (*DownloadResponse, error) {
	ctx, span := tracer.Start(ctx, "assets.download_asset",
		trace.WithAttributes(
			attribute.String("asset_id", req.AssetID.String()),
			attribute.String("user_id", req.UserID.String()),
		))
	defer span.End()

	// Verify asset ownership
	asset, err := s.GetAsset(ctx, req.AssetID, req.UserID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}

	var downloadPath string
	if req.ThumbnailType != nil {
		// Find thumbnail path
		for _, thumb := range asset.Thumbnails {
			if thumb.Type == *req.ThumbnailType {
				downloadPath = thumb.Path
				break
			}
		}
		if downloadPath == "" {
			return nil, fmt.Errorf("thumbnail not found: %s", *req.ThumbnailType)
		}
	} else {
		downloadPath = asset.OriginalPath
	}

	// Determine expiry time based on file type
	var expiry time.Duration
	if req.ThumbnailType != nil {
		// Thumbnails can have longer expiry since they're smaller and cached
		expiry = 24 * time.Hour
	} else {
		// Original files get shorter expiry
		expiry = time.Hour
	}

	// Generate download URL with appropriate expiry
	url, err := s.storage.GeneratePresignedDownloadURL(ctx, downloadPath, expiry)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to generate download URL: %w", err)
	}

	// Add cache control headers for thumbnails
	headers := make(map[string]string)
	if req.ThumbnailType != nil {
		headers["Cache-Control"] = "public, max-age=86400" // 24 hours
		headers["Content-Type"] = "image/jpeg"
	} else {
		headers["Cache-Control"] = "private, max-age=3600" // 1 hour
		// Set content type based on asset type
		switch asset.Type {
		case AssetTypeImage:
			headers["Content-Type"] = "image/jpeg"
		case AssetTypeVideo:
			headers["Content-Type"] = "video/mp4"
		default:
			headers["Content-Type"] = "application/octet-stream"
		}
	}

	s.downloadCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("user_id", req.UserID.String()),
			attribute.String("type", string(asset.Type)),
			attribute.Bool("is_thumbnail", req.ThumbnailType != nil),
		))

	expiresAt := time.Now().Add(expiry)
	return &DownloadResponse{
		URL:       url,
		Headers:   headers,
		ExpiresAt: &expiresAt,
	}, nil
}

// SearchAssets searches for assets based on criteria
func (s *Service) SearchAssets(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	ctx, span := tracer.Start(ctx, "assets.search_assets",
		trace.WithAttributes(
			attribute.String("user_id", req.UserID.String()),
			attribute.String("query", req.Query),
		))
	defer span.End()

	userUUID, err := stringToUUID(req.UserID.String())
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Set up pagination
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	var assets []sqlc.Asset

	// Choose search strategy based on request parameters
	switch {
	case req.Query != "":
		// Text search across metadata
		span.SetAttributes(attribute.String("search_type", "text"))
		textAssets, err := s.db.SearchAssetsByText(ctx, sqlc.SearchAssetsByTextParams{
			OwnerId: userUUID,
			Query:   req.Query,
			Limit:   pgtype.Int4{Int32: int32(limit), Valid: true},
			Offset:  pgtype.Int4{Int32: int32(offset), Valid: true},
		})
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to search assets by text: %w", err)
		}
		assets = textAssets

	case req.StartDate != nil && req.EndDate != nil:
		// Date range search
		span.SetAttributes(attribute.String("search_type", "date_range"))
		startTime, err := timeToTimestamptz(*req.StartDate)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("invalid start date: %w", err)
		}
		endTime, err := timeToTimestamptz(*req.EndDate)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("invalid end date: %w", err)
		}

		dateAssets, err := s.db.GetAssetsByDateRange(ctx, sqlc.GetAssetsByDateRangeParams{
			OwnerId:   userUUID,
			StartDate: startTime,
			EndDate:   endTime,
			Limit:     pgtype.Int4{Int32: int32(limit), Valid: true},
			Offset:    pgtype.Int4{Int32: int32(offset), Valid: true},
		})
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to search assets by date range: %w", err)
		}
		assets = dateAssets

	case req.City != nil || req.State != nil || req.Country != nil:
		// Location-based search (using text search for now)
		span.SetAttributes(attribute.String("search_type", "location"))
		var locationQuery string
		if req.City != nil {
			locationQuery = *req.City
		} else if req.State != nil {
			locationQuery = *req.State
		} else if req.Country != nil {
			locationQuery = *req.Country
		}

		locationAssets, err := s.db.SearchAssetsByText(ctx, sqlc.SearchAssetsByTextParams{
			OwnerId: userUUID,
			Query:   locationQuery,
			Limit:   pgtype.Int4{Int32: int32(limit), Valid: true},
			Offset:  pgtype.Int4{Int32: int32(offset), Valid: true},
		})
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to search assets by location: %w", err)
		}
		assets = locationAssets

	case req.Type != nil:
		// Filter by asset type
		span.SetAttributes(attribute.String("search_type", "type"))
		typeAssets, err := s.db.GetAssets(ctx, sqlc.GetAssetsParams{
			OwnerId:     userUUID,
			Type:        pgtype.Text{String: string(*req.Type), Valid: true},
			IsFavorite:  pgtype.Bool{Bool: false, Valid: false},
			IsArchived:  pgtype.Bool{Bool: false, Valid: false},
			IsTrashed:   pgtype.Bool{Bool: false, Valid: false},
			Limit:       pgtype.Int4{Int32: int32(limit), Valid: true},
			Offset:      pgtype.Int4{Int32: int32(offset), Valid: true},
		})
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to search assets by type: %w", err)
		}
		assets = typeAssets

	default:
		// Default: get all user assets
		span.SetAttributes(attribute.String("search_type", "default"))
		userAssets, err := s.db.GetUserAssets(ctx, sqlc.GetUserAssetsParams{
			OwnerId: userUUID,
			Limit:   pgtype.Int4{Int32: int32(limit), Valid: true},
			Offset:  pgtype.Int4{Int32: int32(offset), Valid: true},
		})
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to get user assets: %w", err)
		}
		assets = userAssets
	}

	// Convert to response format
	assetInfos := make([]AssetInfo, len(assets))
	for i, asset := range assets {
		// Get thumbnails for each asset
		assetFiles, err := s.db.GetAssetFiles(ctx, asset.ID)
		if err != nil {
			span.RecordError(err)
			// Continue without thumbnails
		}

		// Convert asset files to thumbnails
		var thumbnails []AssetThumbnail
		for _, file := range assetFiles {
			// Only include thumbnail files
			if file.Type == string(ThumbnailTypePreview) || file.Type == string(ThumbnailTypeWebp) || file.Type == string(ThumbnailTypeThumb) {
				thumbnails = append(thumbnails, AssetThumbnail{
					AssetID: uuid.MustParse(uuidToString(file.AssetId)),
					Type:    file.Type,
					Path:    file.Path,
					Width:   0, // TODO: Store dimensions in asset_files or calculate
					Height:  0,
					Size:    0, // TODO: Store size in asset_files or calculate
				})
			}
		}

		assetInfos[i] = *s.convertToAssetInfo(asset, thumbnails)
	}

	// Get total count (simplified)
	total := int64(len(assetInfos))

	return &SearchResponse{
		Assets: assetInfos,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

// DeleteAsset marks an asset as deleted
func (s *Service) DeleteAsset(ctx context.Context, assetID uuid.UUID, userID uuid.UUID) error {
	ctx, span := tracer.Start(ctx, "assets.delete_asset",
		trace.WithAttributes(
			attribute.String("asset_id", assetID.String()),
			attribute.String("user_id", userID.String()),
		))
	defer span.End()

	// Verify ownership
	asset, err := s.GetAsset(ctx, assetID, userID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get asset: %w", err)
	}

	assetUUID, err := stringToUUID(assetID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid asset ID: %w", err)
	}

	// Mark as deleted (soft delete)
	_, err = s.db.UpdateAssetStatus(ctx, sqlc.UpdateAssetStatusParams{
		ID:     assetUUID,
		Status: sqlc.AssetsStatusEnumDeleted,
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to delete asset: %w", err)
	}

	// Update storage metrics
	s.storageSize.Add(ctx, -asset.Metadata.Size,
		metric.WithAttributes(attribute.String("operation", "delete")))

	// Schedule background cleanup if this is a hard delete
	// For now, just do immediate cleanup
	// In production, you'd want to use a job queue for this
	go s.cleanupAssetFiles(context.Background(), assetUUID, asset.OriginalPath)

	return nil
}

// HardDeleteAsset permanently deletes an asset and its files
func (s *Service) HardDeleteAsset(ctx context.Context, assetID uuid.UUID, userID uuid.UUID) error {
	ctx, span := tracer.Start(ctx, "assets.hard_delete_asset",
		trace.WithAttributes(
			attribute.String("asset_id", assetID.String()),
			attribute.String("user_id", userID.String()),
		))
	defer span.End()

	// Verify ownership
	asset, err := s.GetAsset(ctx, assetID, userID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get asset: %w", err)
	}

	assetUUID, err := stringToUUID(assetID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid asset ID: %w", err)
	}

	// Delete all associated files immediately
	err = s.cleanupAssetFiles(ctx, assetUUID, asset.OriginalPath)
	if err != nil {
		span.RecordError(err)
		// Continue with database deletion even if file cleanup fails
	}

	// Delete EXIF data
	err = s.db.DeleteExif(ctx, assetUUID)
	if err != nil {
		span.RecordError(err)
		// Continue even if EXIF deletion fails
	}

	// Delete all asset files from database
	err = s.db.DeleteAssetFiles(ctx, assetUUID)
	if err != nil {
		span.RecordError(err)
		// Continue even if asset files deletion fails
	}

	// Permanently delete the asset record
	err = s.db.PermanentlyDeleteAssets(ctx, []pgtype.UUID{assetUUID}, stringToUUIDUnsafe(userID.String()))
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to permanently delete asset: %w", err)
	}

	// Update storage metrics
	s.storageSize.Add(ctx, -asset.Metadata.Size,
		metric.WithAttributes(attribute.String("operation", "hard_delete")))

	return nil
}

// cleanupAssetFiles removes the asset files from storage
func (s *Service) cleanupAssetFiles(ctx context.Context, assetID pgtype.UUID, originalPath string) error {
	ctx, span := tracer.Start(ctx, "assets.cleanup_files",
		trace.WithAttributes(
			attribute.String("asset_id", uuidToString(assetID)),
		))
	defer span.End()

	var cleanupErrors []error

	// Delete original file
	err := s.storage.Delete(ctx, originalPath)
	if err != nil {
		span.RecordError(err)
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to delete original file: %w", err))
	}

	// Get all associated files from database
	assetFiles, err := s.db.GetAssetFiles(ctx, assetID)
	if err != nil {
		span.RecordError(err)
		cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to get asset files: %w", err))
	} else {
		// Delete each associated file (thumbnails, etc.)
		for _, file := range assetFiles {
			err := s.storage.Delete(ctx, file.Path)
			if err != nil {
				span.RecordError(err)
				cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to delete file %s: %w", file.Path, err))
			}
		}
	}

	if len(cleanupErrors) > 0 {
		span.SetAttributes(attribute.Int("cleanup_errors", len(cleanupErrors)))
		// Return the first error, but log all of them
		return cleanupErrors[0]
	}

	span.SetAttributes(attribute.String("status", "success"))
	return nil
}

// RestoreAsset restores a soft-deleted asset
func (s *Service) RestoreAsset(ctx context.Context, assetID uuid.UUID, userID uuid.UUID) error {
	ctx, span := tracer.Start(ctx, "assets.restore_asset",
		trace.WithAttributes(
			attribute.String("asset_id", assetID.String()),
			attribute.String("user_id", userID.String()),
		))
	defer span.End()

	assetUUID, err := stringToUUID(assetID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid asset ID: %w", err)
	}

	userUUID, err := stringToUUID(userID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// Restore the asset by changing its status back to active
	err = s.db.RestoreAssets(ctx, []pgtype.UUID{assetUUID}, userUUID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to restore asset: %w", err)
	}

	return nil
}

// Helper functions

func (s *Service) getAssetTypeFromContentType(contentType string) AssetType {
	return s.metadataExtractor.getAssetTypeFromContentType(contentType)
}

func (s *Service) generateStoragePath(userID uuid.UUID, assetID uuid.UUID, filename string, assetType AssetType) string {
	// Generate a hash-based path for better distribution
	// Format: assets/{userID}/{year}/{month}/{assetID}/{filename}
	now := time.Now()
	return filepath.Join(
		"assets",
		userID.String(),
		fmt.Sprintf("%04d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		assetID.String(),
		filename,
	)
}

func (s *Service) convertToAssetInfo(asset sqlc.Asset, thumbnails []AssetThumbnail) *AssetInfo {
	info := &AssetInfo{
		ID:           uuid.MustParse(uuidToString(asset.ID)),
		UserID:       uuid.MustParse(uuidToString(asset.OwnerId)),
		Type:         AssetType(asset.Type),
		Status:       AssetStatus(asset.Status),
		OriginalPath: asset.OriginalPath,
		CreatedAt:    timestamptzToTime(asset.CreatedAt),
		UpdatedAt:    timestamptzToTime(asset.UpdatedAt),
		Metadata: AssetMetadata{
			Filename:    asset.OriginalFileName,
			ContentType: s.getMimeTypeFromAssetType(asset.Type),
			Size:        0, // TODO: Store size in database
		},
	}

	// Add metadata if available
	if asset.LocalDateTime.Valid {
		dateTaken := timestamptzToTime(asset.LocalDateTime)
		info.Metadata.DateTaken = &dateTaken
	}

	if asset.FileCreatedAt.Valid {
		info.Metadata.CreatedAt = timestamptzToTime(asset.FileCreatedAt)
	}

	if asset.FileModifiedAt.Valid {
		info.Metadata.ModifiedAt = timestamptzToTime(asset.FileModifiedAt)
	}

	// TODO: Get EXIF data from separate exif table
	// For now, we'll leave width, height, make, model empty

	// Add thumbnails
	info.Thumbnails = make([]ThumbnailInfo, len(thumbnails))
	for i, thumb := range thumbnails {
		info.Thumbnails[i] = ThumbnailInfo{
			Type:   ThumbnailType(thumb.Type),
			Path:   thumb.Path,
			Width:  thumb.Width,
			Height: thumb.Height,
			Size:   thumb.Size,
		}
	}

	return info
}

// Helper functions for type conversions (reuse from auth package)
func stringToUUID(s string) (pgtype.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

// getMimeTypeFromAssetType derives MIME type from asset type
func (s *Service) getMimeTypeFromAssetType(assetType string) string {
	switch strings.ToLower(assetType) {
	case "image":
		return "image/jpeg" // Default for images
	case "video":
		return "video/mp4" // Default for videos
	default:
		return "application/octet-stream"
	}
}

func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

func timestamptzToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func timeToTimestamptz(t time.Time) (pgtype.Timestamptz, error) {
	return pgtype.Timestamptz{
		Time:  t,
		Valid: true,
	}, nil
}

func stringToUUIDUnsafe(s string) pgtype.UUID {
	id, _ := uuid.Parse(s)
	return pgtype.UUID{Bytes: id, Valid: true}
}
