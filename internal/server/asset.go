package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/denysvitali/immich-go-backend/internal/assets"
	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/jobs"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/util"
)

func (s *Server) GetAssets(ctx context.Context, request *immichv1.GetAssetsRequest) (*immichv1.GetAssetsResponse, error) {
	// Get user ID from context/auth
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := pgutil.ParseUserID(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	// Calculate offset for pagination
	offset := util.Offset(request.GetPage(), request.GetSize())

	// Build query parameters
	var assetType pgtype.Text
	if request.Type != nil {
		switch *request.Type {
		case immichv1.AssetType_ASSET_TYPE_IMAGE:
			assetType = pgtype.Text{String: "IMAGE", Valid: true}
		case immichv1.AssetType_ASSET_TYPE_VIDEO:
			assetType = pgtype.Text{String: "VIDEO", Valid: true}
		}
	}

	isFavorite := util.OptionalBool(request.IsFavorite)
	isArchived := util.OptionalBool(request.IsArchived)
	isTrashed := util.OptionalBool(request.IsTrashed)

	assets, err := s.db.GetAssets(ctx, sqlc.GetAssetsParams{
		OwnerId:    userID,
		Limit:      request.Size,
		Offset:     offset,
		Type:       assetType,
		IsFavorite: isFavorite,
		IsArchived: isArchived,
		IsTrashed:  isTrashed,
	})
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get assets", err)
	}

	// Get total count for pagination
	totalCount, err := s.db.CountAssets(ctx, sqlc.CountAssetsParams{
		OwnerId:    userID,
		Type:       assetType,
		IsFavorite: isFavorite,
		IsArchived: isArchived,
		IsTrashed:  isTrashed,
	})
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to count assets", err)
	}

	// Convert to proto
	protoAssets := make([]*immichv1.Asset, len(assets))
	for i, asset := range assets {
		protoAssets[i] = s.convertAssetToProto(asset)
	}

	return &immichv1.GetAssetsResponse{
		Assets: protoAssets,
		PageInfo: &immichv1.PageInfo{
			Page:  request.Page,
			Size:  request.Size,
			Total: totalCount,
		},
	}, nil
}

func (s *Server) GetAsset(ctx context.Context, request *immichv1.GetAssetRequest) (*immichv1.Asset, error) {
	asset, err := s.getAuthenticatedAsset(ctx, request.AssetId)
	if err != nil {
		return nil, err
	}

	return s.convertAssetToProto(asset), nil
}

func (s *Server) UploadAsset(ctx context.Context, request *immichv1.UploadAssetRequest) (*immichv1.Asset, error) {
	// Get user ID from context/auth
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := pgutil.ParseUserID(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	assetData := request.AssetData
	if assetData == nil {
		return nil, status.Errorf(codes.InvalidArgument, "asset data is required")
	}

	// Convert asset type
	assetType := "IMAGE"
	if assetData.Type == immichv1.AssetType_ASSET_TYPE_VIDEO {
		assetType = "VIDEO"
	}

	// Checksum is required - either from request or computed from file
	var checksum []byte
	if request.Checksum != nil && *request.Checksum != "" {
		// Assume checksum is a hex string and decode it
		if len(*request.Checksum) >= 2 {
			checksum = []byte(*request.Checksum)
		} else {
			return nil, status.Error(codes.InvalidArgument, "invalid checksum format")
		}
	} else {
		// Checksum is required for asset creation
		return nil, status.Error(codes.InvalidArgument, "checksum is required for asset creation")
	}

	// Set default timestamps if not provided
	fileCreatedAt := timestamppb.Now()
	if assetData.FileCreatedAt != nil {
		fileCreatedAt = assetData.FileCreatedAt
	}

	fileModifiedAt := timestamppb.Now()
	if assetData.FileModifiedAt != nil {
		fileModifiedAt = assetData.FileModifiedAt
	}

	// Determine storage path and optionally store the file.
	// If the request carries raw file bytes (FileContent), upload them to the
	// storage backend and use the server-generated path as OriginalPath.
	// Otherwise fall back to the client-supplied OriginalPath.
	originalPath := assetData.OriginalPath
	fileContent := request.FileContent
	if len(fileContent) > 0 {
		storageService := s.assetService.GetStorageService()
		uploadResult, uploadErr := storageService.UploadAsset(
			ctx,
			claims.UserID,
			assetData.OriginalFileName,
			bytes.NewReader(fileContent),
			int64(len(fileContent)),
		)
		if uploadErr != nil {
			// Log but continue — the asset will be created with the client-supplied
			// path so that the DB record is not lost.
			logrus.WithError(uploadErr).Warn("UploadAsset: failed to store file in storage backend")
		} else {
			originalPath = uploadResult.Path
		}
	}

	asset, err := s.db.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    assetData.DeviceAssetId,
		OwnerId:          userID,
		DeviceId:         assetData.DeviceId,
		Type:             assetType,
		OriginalPath:     originalPath,
		FileCreatedAt:    pgtype.Timestamptz{Time: fileCreatedAt.AsTime(), Valid: true},
		FileModifiedAt:   pgtype.Timestamptz{Time: fileModifiedAt.AsTime(), Valid: true},
		LocalDateTime:    pgtype.Timestamptz{Time: fileCreatedAt.AsTime(), Valid: true},
		OriginalFileName: assetData.OriginalFileName,
		Checksum:         checksum,
		IsFavorite:       assetData.IsFavorite != nil && *assetData.IsFavorite,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to create asset", err)
	}

	// Enqueue background jobs for thumbnail generation and metadata extraction.
	// When Redis / the job service is unavailable, fall back to an in-process goroutine
	// (only when the file was actually stored so that processing can read it).
	assetUUID := uuid.UUID(asset.ID.Bytes)
	assetIDStr := assetUUID.String()

	if s.jobService != nil {
		thumbPayload := &jobs.ThumbnailGenerationPayload{AssetID: assetIDStr}
		if enqErr := s.jobService.EnqueueJobWithPriority(ctx, jobs.JobTypeThumbnailGeneration, thumbPayload, jobs.PriorityHigh); enqErr != nil {
			logrus.WithError(enqErr).Warn("UploadAsset: failed to enqueue thumbnail generation job")
		}

		metaPayload := &jobs.MetadataExtractionPayload{AssetID: assetIDStr}
		if enqErr := s.jobService.EnqueueJobWithPriority(ctx, jobs.JobTypeMetadataExtraction, metaPayload, jobs.PriorityHigh); enqErr != nil {
			logrus.WithError(enqErr).Warn("UploadAsset: failed to enqueue metadata extraction job")
		}

		// For video assets, also enqueue a transcode job
		if assetType == "VIDEO" {
			transcodePayload := &jobs.VideoTranscodePayload{AssetID: assetIDStr, Quality: "medium", Format: "mp4"}
			if enqErr := s.jobService.EnqueueJobWithPriority(ctx, jobs.JobTypeVideoTranscode, transcodePayload, jobs.PriorityNormal); enqErr != nil {
				logrus.WithError(enqErr).Warn("UploadAsset: failed to enqueue video transcode job")
			}
		}
	} else if len(fileContent) > 0 {
		// No job queue configured — trigger in-process background processing.
		// Only runs when the file was actually stored so processAsset can read it.
		s.assetService.TriggerProcessing(assetUUID)
	}

	return s.convertAssetToProto(asset), nil
}

func (s *Server) UpdateAsset(ctx context.Context, request *immichv1.UpdateAssetRequest) (*immichv1.Asset, error) {
	existingAsset, err := s.getAuthenticatedAsset(ctx, request.AssetId)
	if err != nil {
		return nil, err
	}

	var isFavorite, isArchived pgtype.Bool
	if request.IsFavorite != nil {
		isFavorite = pgtype.Bool{Bool: *request.IsFavorite, Valid: true}
	}
	if request.IsArchived != nil {
		isArchived = pgtype.Bool{Bool: *request.IsArchived, Valid: true}
	}

	asset, err := s.db.UpdateAsset(ctx, sqlc.UpdateAssetParams{
		ID:         existingAsset.ID,
		IsFavorite: isFavorite,
		IsArchived: isArchived,
	})
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to update asset", err)
	}

	return s.convertAssetToProto(asset), nil
}

func (s *Server) UpdateAssets(ctx context.Context, request *immichv1.UpdateAssetsRequest) (*emptypb.Empty, error) {
	userID, err := s.userUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	assetIDs := make([]pgtype.UUID, 0, len(request.AssetIds))
	for _, assetID := range request.AssetIds {
		asset, err := s.getAssetForUser(ctx, userID, assetID)
		if err != nil {
			return nil, err
		}
		assetIDs = append(assetIDs, asset.ID)
	}

	var isFavorite, isArchived pgtype.Bool
	if request.IsFavorite != nil {
		isFavorite = pgtype.Bool{Bool: *request.IsFavorite, Valid: true}
	}
	if request.IsArchived != nil {
		isArchived = pgtype.Bool{Bool: *request.IsArchived, Valid: true}
	}

	for _, assetID := range assetIDs {
		_, err := s.db.UpdateAsset(ctx, sqlc.UpdateAssetParams{
			ID:         assetID,
			IsFavorite: isFavorite,
			IsArchived: isArchived,
		})
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to update assets", err)
		}
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) DeleteAssets(ctx context.Context, request *immichv1.DeleteAssetsRequest) (*emptypb.Empty, error) {
	userID, err := s.userUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	for _, assetID := range request.Ids {
		asset, err := s.getAssetForUser(ctx, userID, assetID)
		if err != nil {
			continue
		}

		if request.Force {
			if _, err := s.db.UpdateAssetStatus(ctx, sqlc.UpdateAssetStatusParams{
				ID:     asset.ID,
				Status: sqlc.AssetsStatusEnumDeleted,
			}); err != nil {
				return nil, SanitizedInternal(ctx, "failed to delete assets", err)
			}
			if err := s.db.PermanentlyDeleteAsset(ctx, asset.ID); err != nil {
				return nil, SanitizedInternal(ctx, "failed to delete assets", err)
			}
			continue
		}

		if _, err := s.db.UpdateAssetStatus(ctx, sqlc.UpdateAssetStatusParams{
			ID:     asset.ID,
			Status: sqlc.AssetsStatusEnumTrashed,
		}); err != nil {
			return nil, SanitizedInternal(ctx, "failed to delete assets", err)
		}
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) CheckExistingAssets(ctx context.Context, request *immichv1.CheckExistingAssetsRequest) (*immichv1.CheckExistingAssetsResponse, error) {
	// Get user ID from context/auth
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := pgutil.ParseUserID(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	existingAssets, err := s.db.CheckExistingAssets(ctx, sqlc.CheckExistingAssetsParams{
		OwnerId:  userID,
		DeviceId: request.DeviceId,
		Column3:  request.DeviceAssetIds,
	})
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to check existing assets", err)
	}

	// Create map of existing IDs
	existingMap := make(map[string]bool)
	for _, deviceAssetID := range existingAssets {
		existingMap[deviceAssetID] = true
	}

	// Fill in missing IDs as false
	for _, deviceAssetID := range request.DeviceAssetIds {
		if _, exists := existingMap[deviceAssetID]; !exists {
			existingMap[deviceAssetID] = false
		}
	}

	return &immichv1.CheckExistingAssetsResponse{
		ExistingIds: existingMap,
	}, nil
}

func (s *Server) CheckBulkUpload(ctx context.Context, request *immichv1.CheckBulkUploadRequest) (*immichv1.CheckBulkUploadResponse, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := pgutil.ParseUserID(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	type assetKey struct {
		deviceID      string
		deviceAssetID string
	}

	requested := make([]assetKey, 0, len(request.GetAssets()))
	idsByDevice := make(map[string][]string)
	seenByDevice := make(map[string]map[string]struct{})
	for _, asset := range request.GetAssets() {
		if asset == nil || asset.GetDeviceId() == "" || asset.GetDeviceAssetId() == "" {
			continue
		}
		key := assetKey{
			deviceID:      asset.GetDeviceId(),
			deviceAssetID: asset.GetDeviceAssetId(),
		}
		requested = append(requested, key)

		deviceSeen := seenByDevice[key.deviceID]
		if deviceSeen == nil {
			deviceSeen = make(map[string]struct{})
			seenByDevice[key.deviceID] = deviceSeen
		}
		if _, ok := deviceSeen[key.deviceAssetID]; ok {
			continue
		}
		deviceSeen[key.deviceAssetID] = struct{}{}
		idsByDevice[key.deviceID] = append(idsByDevice[key.deviceID], key.deviceAssetID)
	}

	if len(requested) == 0 {
		return &immichv1.CheckBulkUploadResponse{Results: []*immichv1.Asset{}}, nil
	}

	existingByKey := make(map[assetKey]sqlc.Asset)
	for deviceID, deviceAssetIDs := range idsByDevice {
		foundAssets, err := s.db.GetAssetsByDeviceAssetIDs(ctx, sqlc.GetAssetsByDeviceAssetIDsParams{
			OwnerID:        userID,
			DeviceID:       deviceID,
			DeviceAssetIds: deviceAssetIDs,
		})
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to check bulk upload assets", err)
		}
		for _, asset := range foundAssets {
			existingByKey[assetKey{
				deviceID:      asset.DeviceId,
				deviceAssetID: asset.DeviceAssetId,
			}] = asset
		}
	}

	results := make([]*immichv1.Asset, 0, len(existingByKey))
	emitted := make(map[assetKey]struct{}, len(existingByKey))
	for _, key := range requested {
		asset, ok := existingByKey[key]
		if !ok {
			continue
		}
		if _, ok := emitted[key]; ok {
			continue
		}
		emitted[key] = struct{}{}
		results = append(results, s.convertAssetToProto(asset))
	}

	return &immichv1.CheckBulkUploadResponse{
		Results: results,
	}, nil
}

func (s *Server) GetAssetStatistics(ctx context.Context, request *immichv1.GetAssetStatisticsRequest) (*immichv1.AssetStatisticsResponse, error) {
	// Get user ID from context/auth
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := pgutil.ParseUserID(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	stats, err := s.db.GetAssetStatistics(ctx, userID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get asset statistics", err)
	}

	return &immichv1.AssetStatisticsResponse{
		Images: int32(stats.Images),
		Videos: int32(stats.Videos),
		Total:  int32(stats.Total),
	}, nil
}

func (s *Server) GetAllUserAssetsByDeviceId(ctx context.Context, request *immichv1.GetAllUserAssetsByDeviceIdRequest) (*immichv1.GetAllUserAssetsByDeviceIdResponse, error) {
	// Get user ID from context/auth
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := pgutil.ParseUserID(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	assetIDs, err := s.db.GetAssetsByDeviceId(ctx, sqlc.GetAssetsByDeviceIdParams{
		OwnerId:  userID,
		DeviceId: request.DeviceId,
	})
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get assets by device ID", err)
	}

	// Convert UUIDs to strings
	assetIDStrings := make([]string, len(assetIDs))
	for i, id := range assetIDs {
		assetIDStrings[i] = id.String()
	}

	return &immichv1.GetAllUserAssetsByDeviceIdResponse{
		AssetIds: assetIDStrings,
	}, nil
}

func (s *Server) GetRandom(ctx context.Context, request *immichv1.GetRandomRequest) (*immichv1.GetRandomResponse, error) {
	// Get user ID from context/auth
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := pgutil.ParseUserID(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	count := int32(10) // Default count
	if request.Count != nil {
		count = *request.Count
	}

	assets, err := s.db.GetRandomAssets(ctx, sqlc.GetRandomAssetsParams{
		OwnerId: userID,
		Limit:   count,
	})
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get random assets", err)
	}

	protoAssets := make([]*immichv1.Asset, len(assets))
	for i, asset := range assets {
		protoAssets[i] = s.convertAssetToProto(asset)
	}

	return &immichv1.GetRandomResponse{
		Assets: protoAssets,
	}, nil
}

func (s *Server) GetRecentlyAddedAssets(ctx context.Context, request *immichv1.GetRecentlyAddedAssetsRequest) (*immichv1.GetRecentlyAddedAssetsResponse, error) {
	// Get user ID from context/auth
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := pgutil.ParseUserID(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	// Default to 12 when limit is 0, cap at 100 to prevent abuse
	limit := request.GetLimit()
	if limit == 0 {
		limit = 12
	}
	if limit > 100 {
		limit = 100
	}

	assets, err := s.db.GetRecentlyAddedAssets(ctx, sqlc.GetRecentlyAddedAssetsParams{
		OwnerId: userID,
		Limit:   int32(limit),
	})
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get recently added assets", err)
	}

	protoAssets := make([]*immichv1.Asset, len(assets))
	for i, asset := range assets {
		protoAssets[i] = s.convertAssetToProto(asset)
	}

	return &immichv1.GetRecentlyAddedAssetsResponse{
		Assets: protoAssets,
	}, nil
}

func (s *Server) RunAssetJobs(ctx context.Context, request *immichv1.RunAssetJobsRequest) (*emptypb.Empty, error) {
	userID, err := s.userUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if s.jobService == nil {
		return nil, status.Error(codes.Unavailable, "job service not configured")
	}

	switch request.GetName() {
	case immichv1.AssetJobName_ASSET_JOB_NAME_THUMBNAIL_GENERATION:
		if err := s.enqueueAssetJobsForAssets(ctx, userID, request.GetAssetIds(), jobs.JobTypeThumbnailGeneration, jobs.PriorityHigh); err != nil {
			return nil, err
		}
	case immichv1.AssetJobName_ASSET_JOB_NAME_METADATA_EXTRACTION:
		if err := s.enqueueAssetJobsForAssets(ctx, userID, request.GetAssetIds(), jobs.JobTypeMetadataExtraction, jobs.PriorityHigh); err != nil {
			return nil, err
		}
	case immichv1.AssetJobName_ASSET_JOB_NAME_DUPLICATE_DETECTION:
		payload := jobs.DuplicateDetectionPayload{UserID: userID.String()}
		if err := s.jobService.EnqueueJobWithPriority(ctx, jobs.JobTypeDuplicateDetect, payload, jobs.PriorityNormal); err != nil {
			return nil, SanitizedInternal(ctx, "failed to enqueue duplicate detection job", err)
		}
	case immichv1.AssetJobName_ASSET_JOB_NAME_VIDEO_CONVERSION:
		if err := s.enqueueAssetJobsForAssets(ctx, userID, request.GetAssetIds(), jobs.JobTypeVideoTranscode, jobs.PriorityHigh); err != nil {
			return nil, err
		}
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown asset job name: %v", request.GetName())
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) enqueueAssetJobsForAssets(
	ctx context.Context,
	userID pgtype.UUID,
	assetIDs []string,
	jobType jobs.JobType,
	priority jobs.JobPriority,
) error {
	if len(assetIDs) == 0 {
		return status.Error(codes.InvalidArgument, "asset_ids are required for this asset job")
	}

	for _, assetID := range assetIDs {
		asset, err := s.getAssetForUser(ctx, userID, assetID)
		if err != nil {
			return err
		}

		var payload any
		switch jobType {
		case jobs.JobTypeThumbnailGeneration:
			payload = jobs.ThumbnailGenerationPayload{AssetID: asset.ID.String()}
		case jobs.JobTypeMetadataExtraction:
			payload = jobs.MetadataExtractionPayload{AssetID: asset.ID.String()}
		case jobs.JobTypeVideoTranscode:
			payload = jobs.VideoTranscodePayload{AssetID: asset.ID.String(), Quality: "medium", Format: "mp4"}
		default:
			return status.Errorf(codes.InvalidArgument, "unsupported asset job type: %s", jobType)
		}

		if err := s.jobService.EnqueueJobWithPriority(ctx, jobType, payload, priority); err != nil {
			return SanitizedInternal(ctx, "failed to enqueue asset job", err)
		}
	}

	return nil
}

func (s *Server) DownloadAsset(ctx context.Context, request *immichv1.DownloadAssetRequest) (*immichv1.DownloadAssetResponse, error) {
	asset, err := s.getAuthenticatedAsset(ctx, request.AssetId)
	if err != nil {
		return nil, err
	}

	// Get storage service
	storageService := s.assetService.GetStorageService()

	// Download the asset
	assetData, err := storageService.Download(ctx, asset.OriginalPath)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to retrieve asset", err)
	}
	defer assetData.Close()

	// Read asset data
	data, err := io.ReadAll(assetData)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to read asset data", err)
	}

	return &immichv1.DownloadAssetResponse{
		Data:        data,
		ContentType: assetDownloadContentType(asset.OriginalFileName),
		Filename:    asset.OriginalFileName,
	}, nil
}

func (s *Server) ReplaceAsset(ctx context.Context, request *immichv1.ReplaceAssetRequest) (*immichv1.Asset, error) {
	existingAsset, err := s.getAuthenticatedAsset(ctx, request.AssetId)
	if err != nil {
		return nil, err
	}

	// For now, return the existing asset as if it was replaced
	// In a full implementation, this would:
	// 1. Process the new asset data from request.AssetData
	// 2. Store the new file using storage service
	// 3. Update the database record
	// 4. Handle thumbnails and metadata extraction

	// Convert to proto asset
	return s.convertAssetToProto(existingAsset), nil
}

func (s *Server) GetAssetThumbnail(ctx context.Context, request *immichv1.GetAssetThumbnailRequest) (*immichv1.GetAssetThumbnailResponse, error) {
	asset, err := s.getAuthenticatedAsset(ctx, request.AssetId)
	if err != nil {
		return nil, err
	}

	// Determine thumbnail type based on format parameter
	thumbnailType := assets.ThumbnailTypeWebp
	if request.Format != nil {
		switch *request.Format {
		case immichv1.ImageFormat_IMAGE_FORMAT_JPEG:
			thumbnailType = assets.ThumbnailTypeThumb
		case immichv1.ImageFormat_IMAGE_FORMAT_WEBP:
			thumbnailType = assets.ThumbnailTypeWebp
		default:
			thumbnailType = assets.ThumbnailTypePreview
		}
	}

	// Generate thumbnail path
	generator := assets.NewThumbnailGenerator()
	thumbnailPath := generator.GetThumbnailPath(asset.OriginalPath, thumbnailType)

	// Try to retrieve existing thumbnail from storage
	storageService := s.assetService.GetStorageService()
	thumbnailData, err := storageService.Download(ctx, thumbnailPath)
	if err != nil {
		// If thumbnail doesn't exist, try to generate it
		originalData, err := storageService.Download(ctx, asset.OriginalPath)
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to retrieve original asset", err)
		}

		// Generate thumbnails
		thumbnails, err := generator.GenerateThumbnails(ctx, originalData, asset.OriginalFileName)
		if err != nil {
			return nil, SanitizedInternal(ctx, "failed to generate thumbnail", err)
		}

		// Get the requested thumbnail type
		thumbData, ok := thumbnails[thumbnailType]
		if !ok {
			return nil, status.Errorf(codes.Internal, "thumbnail type not generated")
		}

		// Store the generated thumbnail for future use
		if err := storageService.Upload(ctx, thumbnailPath, bytes.NewReader(thumbData), s.getThumbnailContentType(thumbnailType)); err != nil {
			// Log error but don't fail the request
			logrus.WithError(err).Warn("Failed to store generated thumbnail")
		}

		// Return the generated thumbnail data
		return &immichv1.GetAssetThumbnailResponse{
			Data:        thumbData,
			ContentType: s.getThumbnailContentType(thumbnailType),
		}, nil
	}

	// Read thumbnail data
	thumbData, err := io.ReadAll(thumbnailData)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to read thumbnail data", err)
	}

	return &immichv1.GetAssetThumbnailResponse{
		Data:        thumbData,
		ContentType: s.getThumbnailContentType(thumbnailType),
	}, nil
}

// getThumbnailContentType returns the MIME type for a thumbnail type
func (s *Server) getThumbnailContentType(thumbnailType assets.ThumbnailType) string {
	if contentType, ok := thumbnailContentTypes[thumbnailType]; ok {
		return contentType
	}
	return "image/jpeg"
}

var thumbnailContentTypes = map[assets.ThumbnailType]string{
	assets.ThumbnailTypePreview: "image/jpeg",
	assets.ThumbnailTypeWebp:    "image/webp",
	assets.ThumbnailTypeThumb:   "image/jpeg",
}

func (s *Server) PlayAssetVideo(ctx context.Context, request *immichv1.PlayAssetVideoRequest) (*immichv1.PlayAssetVideoResponse, error) {
	asset, err := s.getAuthenticatedAsset(ctx, request.AssetId)
	if err != nil {
		return nil, err
	}

	// Check if asset is a video
	if asset.Type != "VIDEO" {
		return nil, status.Errorf(codes.InvalidArgument, "asset is not a video")
	}

	// Get storage service
	storageService := s.assetService.GetStorageService()

	// Prefer encoded H.264 copy if available, otherwise fall back to original
	videoPath := asset.OriginalPath
	if asset.EncodedVideoPath.Valid && asset.EncodedVideoPath.String != "" {
		videoPath = asset.EncodedVideoPath.String
	}

	videoStream, err := storageService.Download(ctx, videoPath)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to retrieve video", err)
	}
	defer videoStream.Close()

	// Read video data (this is simplified - in production you'd want streaming)
	videoData, err := io.ReadAll(videoStream)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to read video data", err)
	}

	return &immichv1.PlayAssetVideoResponse{
		Data:        videoData,
		ContentType: s.getVideoContentType(asset.OriginalFileName),
	}, nil
}

// getVideoContentType determines the video MIME type from filename
func (s *Server) getVideoContentType(filename string) string {
	if contentType, ok := videoContentTypes[fileExtension(filename)]; ok {
		return contentType
	}
	return "video/mp4"
}

var videoContentTypes = map[string]string{
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".avi":  "video/x-msvideo",
	".mov":  "video/quicktime",
	".mkv":  "video/x-matroska",
	".flv":  "video/x-flv",
	".wmv":  "video/x-ms-wmv",
	".m4v":  "video/x-m4v",
}

func assetDownloadContentType(filename string) string {
	if contentType, ok := assetDownloadContentTypes[fileExtension(filename)]; ok {
		return contentType
	}
	return "application/octet-stream"
}

func (s *Server) getAuthenticatedAsset(ctx context.Context, assetID string) (sqlc.Asset, error) {
	userID, err := s.userUUIDFromContext(ctx)
	if err != nil {
		return sqlc.Asset{}, err
	}

	return s.getAssetForUser(ctx, userID, assetID)
}

func (s *Server) getAssetForUser(ctx context.Context, userID pgtype.UUID, assetID string) (sqlc.Asset, error) {
	parsedAssetID, err := uuid.Parse(assetID)
	if err != nil {
		return sqlc.Asset{}, status.Errorf(codes.InvalidArgument, "invalid asset ID: %v", err)
	}

	asset, err := s.db.GetAssetByIDAndUser(ctx, sqlc.GetAssetByIDAndUserParams{
		ID:      pgtype.UUID{Bytes: parsedAssetID, Valid: true},
		OwnerId: userID,
	})
	if err != nil {
		return sqlc.Asset{}, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}

	return asset, nil
}

var assetDownloadContentTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".mov":  "video/quicktime",
	".avi":  "video/x-msvideo",
	".pdf":  "application/pdf",
}

func fileExtension(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}

// Helper function to convert database asset to proto
func (s *Server) convertAssetToProto(asset sqlc.Asset) *immichv1.Asset {
	protoAsset := &immichv1.Asset{
		Id:               asset.ID.String(),
		DeviceAssetId:    asset.DeviceAssetId,
		OwnerId:          asset.OwnerId.String(),
		DeviceId:         asset.DeviceId,
		Type:             assets.AssetTypeFromString(asset.Type),
		OriginalPath:     asset.OriginalPath,
		OriginalFileName: asset.OriginalFileName,
		CreatedAt:        timestamppb.New(asset.CreatedAt.Time),
		UpdatedAt:        timestamppb.New(asset.UpdatedAt.Time),
		IsFavorite:       asset.IsFavorite,
		IsArchived:       asset.Visibility == sqlc.AssetVisibilityEnumArchive,
		IsTrashed:        asset.Status == sqlc.AssetsStatusEnumTrashed,
		Checksum:         fmt.Sprintf("%x", asset.Checksum),
	}

	if asset.Duration.Valid {
		protoAsset.Duration = &asset.Duration.String
	}

	if asset.LivePhotoVideoId.Valid {
		livePhotoID := asset.LivePhotoVideoId.String()
		protoAsset.LivePhotoVideoId = &livePhotoID
	}

	if asset.StackId.Valid {
		stackParentID := asset.StackId.String()
		protoAsset.StackParentId = &stackParentID
	}

	return protoAsset
}
