package server

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) GetAssets(ctx context.Context, request *immichv1.GetAssetsRequest) (*immichv1.GetAssetsResponse, error) {
	// TODO: Get user ID from context/auth
	userID := pgtype.UUID{}
	if err := userID.Scan("00000000-0000-0000-0000-000000000000"); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}

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
		Limit:      int32(request.Size),
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
	// TODO: Get user ID from context/auth
	userID := pgtype.UUID{}
	if err := userID.Scan("00000000-0000-0000-0000-000000000000"); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
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

	// Create checksum from request or generate placeholder
	checksum := []byte("placeholder-checksum")
	if request.Checksum != nil {
		checksum = []byte(*request.Checksum)
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
	// TODO: Get user ID from context/auth
	userID := pgtype.UUID{}
	if err := userID.Scan("00000000-0000-0000-0000-000000000000"); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}

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
	// TODO: Get user ID from context/auth
	userID := pgtype.UUID{}
	if err := userID.Scan("00000000-0000-0000-0000-000000000000"); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}

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
	// TODO: Get user ID from context/auth
	userID := pgtype.UUID{}
	if err := userID.Scan("00000000-0000-0000-0000-000000000000"); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}

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
	// TODO: Get user ID from context/auth
	userID := pgtype.UUID{}
	if err := userID.Scan("00000000-0000-0000-0000-000000000000"); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
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
	// File download would be implemented here
	return nil, status.Errorf(codes.Unimplemented, "download asset not implemented")
}

func (s *Server) ReplaceAsset(ctx context.Context, request *immichv1.ReplaceAssetRequest) (*immichv1.Asset, error) {
	// Asset replacement would be implemented here
	return nil, status.Errorf(codes.Unimplemented, "replace asset not implemented")
}

func (s *Server) GetAssetThumbnail(ctx context.Context, request *immichv1.GetAssetThumbnailRequest) (*immichv1.GetAssetThumbnailResponse, error) {
	// Thumbnail generation/retrieval would be implemented here
	return nil, status.Errorf(codes.Unimplemented, "get asset thumbnail not implemented")
}

func (s *Server) PlayAssetVideo(ctx context.Context, request *immichv1.PlayAssetVideoRequest) (*immichv1.PlayAssetVideoResponse, error) {
	// Video streaming would be implemented here
	return nil, status.Errorf(codes.Unimplemented, "play asset video not implemented")
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