package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/denysvitali/immich-go-backend/internal/assets"
	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) GetAssets(ctx context.Context, request *immichv1.GetAssetsRequest) (*immichv1.GetAssetsResponse, error) {
	// Get user ID from context/auth
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	uid, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userID := pgtype.UUID{Bytes: uid, Valid: true}

	// Calculate offset for pagination
	offset := int32(0)
	if request.Page > 0 {
		offset = (request.Page - 1) * request.Size
	}

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

	var isFavorite, isArchived, isTrashed pgtype.Bool
	if request.IsFavorite != nil {
		isFavorite = pgtype.Bool{Bool: *request.IsFavorite, Valid: true}
	}
	if request.IsArchived != nil {
		isArchived = pgtype.Bool{Bool: *request.IsArchived, Valid: true}
	}
	if request.IsTrashed != nil {
		isTrashed = pgtype.Bool{Bool: *request.IsTrashed, Valid: true}
	}

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
		return nil, status.Errorf(codes.Internal, "failed to get assets: %v", err)
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
		return nil, status.Errorf(codes.Internal, "failed to count assets: %v", err)
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
	assetID := pgtype.UUID{}
	if err := assetID.Scan(request.AssetId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid asset ID: %v", err)
	}

	asset, err := s.db.GetAsset(ctx, assetID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}

	return s.convertAssetToProto(asset), nil
}

func (s *Server) UploadAsset(ctx context.Context, request *immichv1.UploadAssetRequest) (*immichv1.Asset, error) {
	// Get user ID from context/auth
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	uid, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userID := pgtype.UUID{Bytes: uid, Valid: true}

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

	asset, err := s.db.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    assetData.DeviceAssetId,
		OwnerId:          userID,
		DeviceId:         assetData.DeviceId,
		Type:             assetType,
		OriginalPath:     assetData.OriginalPath,
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
		return nil, status.Errorf(codes.Internal, "failed to create asset: %v", err)
	}

	return s.convertAssetToProto(asset), nil
}

func (s *Server) UpdateAsset(ctx context.Context, request *immichv1.UpdateAssetRequest) (*immichv1.Asset, error) {
	assetID := pgtype.UUID{}
	if err := assetID.Scan(request.AssetId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid asset ID: %v", err)
	}

	var isFavorite, isArchived pgtype.Bool
	if request.IsFavorite != nil {
		isFavorite = pgtype.Bool{Bool: *request.IsFavorite, Valid: true}
	}
	if request.IsArchived != nil {
		isArchived = pgtype.Bool{Bool: *request.IsArchived, Valid: true}
	}

	asset, err := s.db.UpdateAsset(ctx, sqlc.UpdateAssetParams{
		ID:         assetID,
		IsFavorite: isFavorite,
		IsArchived: isArchived,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update asset: %v", err)
	}

	return s.convertAssetToProto(asset), nil
}

func (s *Server) UpdateAssets(ctx context.Context, request *immichv1.UpdateAssetsRequest) (*emptypb.Empty, error) {
	// For bulk updates, we would need to implement a bulk update query
	// For now, update each asset individually
	var isFavorite, isArchived pgtype.Bool
	if request.IsFavorite != nil {
		isFavorite = pgtype.Bool{Bool: *request.IsFavorite, Valid: true}
	}
	if request.IsArchived != nil {
		isArchived = pgtype.Bool{Bool: *request.IsArchived, Valid: true}
	}

	for _, assetID := range request.AssetIds {
		assetUUID := pgtype.UUID{}
		if err := assetUUID.Scan(assetID); err != nil {
			continue // Skip invalid UUIDs
		}

		_, err := s.db.UpdateAsset(ctx, sqlc.UpdateAssetParams{
			ID:         assetUUID,
			IsFavorite: isFavorite,
			IsArchived: isArchived,
		})
		if err != nil {
			// Log error but continue with other assets
			continue
		}
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) DeleteAssets(ctx context.Context, request *immichv1.DeleteAssetsRequest) (*emptypb.Empty, error) {
	// Convert string IDs to UUIDs
	assetUUIDs := make([]pgtype.UUID, 0, len(request.AssetIds))
	for _, assetID := range request.AssetIds {
		assetUUID := pgtype.UUID{}
		if err := assetUUID.Scan(assetID); err != nil {
			continue // Skip invalid UUIDs
		}
		assetUUIDs = append(assetUUIDs, assetUUID)
	}

	if len(assetUUIDs) == 0 {
		return &emptypb.Empty{}, nil
	}

	if err := s.db.DeleteAssets(ctx, sqlc.DeleteAssetsParams{
		Column1: assetUUIDs,
		Column2: request.Force,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete assets: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) CheckExistingAssets(ctx context.Context, request *immichv1.CheckExistingAssetsRequest) (*immichv1.CheckExistingAssetsResponse, error) {
	// Get user ID from context/auth
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	uid, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userID := pgtype.UUID{Bytes: uid, Valid: true}

	existingAssets, err := s.db.CheckExistingAssets(ctx, sqlc.CheckExistingAssetsParams{
		OwnerId:  userID,
		DeviceId: request.DeviceId,
		Column3:  request.DeviceAssetIds,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check existing assets: %v", err)
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
	// This would typically check which assets already exist and return only new ones
	// For now, return empty results
	return &immichv1.CheckBulkUploadResponse{
		Results: []*immichv1.Asset{},
	}, nil
}

func (s *Server) GetAssetStatistics(ctx context.Context, request *immichv1.GetAssetStatisticsRequest) (*immichv1.AssetStatisticsResponse, error) {
	// Get user ID from context/auth
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	uid, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userID := pgtype.UUID{Bytes: uid, Valid: true}

	stats, err := s.db.GetAssetStatistics(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get asset statistics: %v", err)
	}

	return &immichv1.AssetStatisticsResponse{
		Images: int32(stats.Images),
		Videos: int32(stats.Videos),
		Total:  int32(stats.Total),
	}, nil
}

func (s *Server) GetAllUserAssetsByDeviceId(ctx context.Context, request *immichv1.GetAllUserAssetsByDeviceIdRequest) (*immichv1.GetAllUserAssetsByDeviceIdResponse, error) {
	// Get user ID from context/auth
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	uid, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userID := pgtype.UUID{Bytes: uid, Valid: true}

	assetIDs, err := s.db.GetAssetsByDeviceId(ctx, sqlc.GetAssetsByDeviceIdParams{
		OwnerId:  userID,
		DeviceId: request.DeviceId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get assets by device ID: %v", err)
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
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	uid, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userID := pgtype.UUID{Bytes: uid, Valid: true}

	count := int32(10) // Default count
	if request.Count != nil {
		count = *request.Count
	}

	assets, err := s.db.GetRandomAssets(ctx, sqlc.GetRandomAssetsParams{
		OwnerId: userID,
		Limit:   count,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get random assets: %v", err)
	}

	protoAssets := make([]*immichv1.Asset, len(assets))
	for i, asset := range assets {
		protoAssets[i] = s.convertAssetToProto(asset)
	}

	return &immichv1.GetRandomResponse{
		Assets: protoAssets,
	}, nil
}

func (s *Server) RunAssetJobs(ctx context.Context, request *immichv1.RunAssetJobsRequest) (*emptypb.Empty, error) {
	// Asset job processing would be implemented here
	// For now, just return success
	return &emptypb.Empty{}, nil
}

func (s *Server) DownloadAsset(ctx context.Context, request *immichv1.DownloadAssetRequest) (*immichv1.DownloadAssetResponse, error) {
	// Parse asset ID
	assetID, err := uuid.Parse(request.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid asset ID: %v", err)
	}

	// Convert to pgtype.UUID
	assetUUID := pgtype.UUID{Bytes: assetID, Valid: true}

	// Get asset from database
	asset, err := s.db.GetAssetByID(ctx, assetUUID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}

	// Get storage service
	storageService := s.assetService.GetStorageService()

	// Download the asset
	assetData, err := storageService.Download(ctx, asset.OriginalPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve asset: %v", err)
	}
	defer assetData.Close()

	// Read asset data
	data, err := io.ReadAll(assetData)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read asset data: %v", err)
	}

	// Determine content type based on file extension
	contentType := "application/octet-stream"
	ext := strings.ToLower(filepath.Ext(asset.OriginalFileName))
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	case ".mp4":
		contentType = "video/mp4"
	case ".webm":
		contentType = "video/webm"
	case ".mov":
		contentType = "video/quicktime"
	case ".avi":
		contentType = "video/x-msvideo"
	case ".pdf":
		contentType = "application/pdf"
	}

	return &immichv1.DownloadAssetResponse{
		Data:        data,
		ContentType: contentType,
		Filename:    asset.OriginalFileName,
	}, nil
}

func (s *Server) ReplaceAsset(ctx context.Context, request *immichv1.ReplaceAssetRequest) (*immichv1.Asset, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse asset ID
	assetID, err := uuid.Parse(request.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid asset ID: %v", err)
	}

	// Convert to pgtype.UUID
	assetUUID := pgtype.UUID{Bytes: assetID, Valid: true}

	// Get existing asset to verify ownership
	existingAsset, err := s.queries.GetAssetByID(ctx, assetUUID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}

	// Verify ownership
	if existingAsset.OwnerId.Bytes != uuid.MustParse(claims.UserID) {
		return nil, status.Error(codes.PermissionDenied, "not authorized to replace this asset")
	}

	// For now, return the existing asset as if it was replaced
	// In a full implementation, this would:
	// 1. Process the new asset data from request.AssetData
	// 2. Store the new file using storage service
	// 3. Update the database record
	// 4. Handle thumbnails and metadata extraction

	// Convert to proto asset
	return &immichv1.Asset{
		Id:               uuid.UUID(existingAsset.ID.Bytes).String(),
		OwnerId:          uuid.UUID(existingAsset.OwnerId.Bytes).String(),
		OriginalFileName: existingAsset.OriginalFileName,
		OriginalPath:     existingAsset.OriginalPath,
		Type:             immichv1.AssetType_ASSET_TYPE_IMAGE,
		CreatedAt:        timestamppb.New(existingAsset.CreatedAt.Time),
		UpdatedAt:        timestamppb.New(time.Now()),
	}, nil
}

func (s *Server) GetAssetThumbnail(ctx context.Context, request *immichv1.GetAssetThumbnailRequest) (*immichv1.GetAssetThumbnailResponse, error) {
	// Parse asset ID
	assetID, err := uuid.Parse(request.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid asset ID: %v", err)
	}

	// Convert to pgtype.UUID
	assetUUID := pgtype.UUID{Bytes: assetID, Valid: true}

	// Get asset from database
	asset, err := s.db.GetAssetByID(ctx, assetUUID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
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
			return nil, status.Errorf(codes.Internal, "failed to retrieve original asset: %v", err)
		}

		// Generate thumbnails
		thumbnails, err := generator.GenerateThumbnails(ctx, originalData, asset.OriginalFileName)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to generate thumbnail: %v", err)
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
		return nil, status.Errorf(codes.Internal, "failed to read thumbnail data: %v", err)
	}

	return &immichv1.GetAssetThumbnailResponse{
		Data:        thumbData,
		ContentType: s.getThumbnailContentType(thumbnailType),
	}, nil
}

// getThumbnailContentType returns the MIME type for a thumbnail type
func (s *Server) getThumbnailContentType(thumbnailType assets.ThumbnailType) string {
	switch thumbnailType {
	case assets.ThumbnailTypeWebp:
		return "image/webp"
	case assets.ThumbnailTypePreview, assets.ThumbnailTypeThumb:
		return "image/jpeg"
	default:
		return "image/jpeg"
	}
}

func (s *Server) PlayAssetVideo(ctx context.Context, request *immichv1.PlayAssetVideoRequest) (*immichv1.PlayAssetVideoResponse, error) {
	// Parse asset ID
	assetID, err := uuid.Parse(request.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid asset ID: %v", err)
	}

	// Convert to pgtype.UUID
	assetUUID := pgtype.UUID{Bytes: assetID, Valid: true}

	// Get asset from database
	asset, err := s.db.GetAssetByID(ctx, assetUUID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}

	// Check if asset is a video
	if asset.Type != "VIDEO" {
		return nil, status.Errorf(codes.InvalidArgument, "asset is not a video")
	}

	// Get storage service
	storageService := s.assetService.GetStorageService()

	// For now, we'll stream the video data directly
	// In production, you'd want to implement proper video streaming with range support

	videoStream, err := storageService.Download(ctx, asset.OriginalPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve video: %v", err)
	}
	defer videoStream.Close()

	// Read video data (this is simplified - in production you'd want streaming)
	videoData, err := io.ReadAll(videoStream)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read video data: %v", err)
	}

	return &immichv1.PlayAssetVideoResponse{
		Data:        videoData,
		ContentType: s.getVideoContentType(asset.OriginalFileName),
	}, nil
}

// getVideoContentType determines the video MIME type from filename
func (s *Server) getVideoContentType(filename string) string {
	ext := filepath.Ext(strings.ToLower(filename))
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	case ".flv":
		return "video/x-flv"
	case ".wmv":
		return "video/x-ms-wmv"
	case ".m4v":
		return "video/x-m4v"
	default:
		return "video/mp4" // Default to mp4
	}
}

// Helper function to convert database asset to proto
func (s *Server) convertAssetToProto(asset sqlc.Asset) *immichv1.Asset {
	assetType := immichv1.AssetType_ASSET_TYPE_IMAGE
	if asset.Type == "VIDEO" {
		assetType = immichv1.AssetType_ASSET_TYPE_VIDEO
	}

	protoAsset := &immichv1.Asset{
		Id:               asset.ID.String(),
		DeviceAssetId:    asset.DeviceAssetId,
		OwnerId:          asset.OwnerId.String(),
		DeviceId:         asset.DeviceId,
		Type:             assetType,
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
