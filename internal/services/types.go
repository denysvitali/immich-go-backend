package services

import (
	"github.com/denysvitali/immich-go-backend/internal/models"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func toAssetProto(asset models.Asset) *immichv1.Asset {
	var duration *string
	if asset.Duration != "" {
		duration = &asset.Duration
	}

	// Convert asset type to protobuf enum
	var assetType immichv1.AssetType
	switch asset.Type {
	case "IMAGE":
		assetType = immichv1.AssetType_ASSET_TYPE_IMAGE
	case "VIDEO":
		assetType = immichv1.AssetType_ASSET_TYPE_VIDEO
	default:
		assetType = immichv1.AssetType_ASSET_TYPE_UNSPECIFIED
	}

	return &immichv1.Asset{
		Id:               asset.ID.String(),
		DeviceAssetId:    asset.DeviceAssetID,
		OwnerId:          asset.OwnerID.String(),
		DeviceId:         asset.DeviceID,
		Type:             assetType,
		OriginalPath:     asset.OriginalPath,
		OriginalFileName: asset.OriginalFileName,
		ResizePath:       nil, // TODO: implement resize path
		WebpPath:         nil, // TODO: implement webp path
		ThumbhashPath:    nil, // TODO: implement thumbhash path
		EncodedVideoPath: nil, // TODO: implement encoded video path
		CreatedAt:        timestamppb.New(asset.CreatedAt),
		UpdatedAt:        timestamppb.New(asset.UpdatedAt),
		IsFavorite:       asset.IsFavorite,
		IsArchived:       asset.IsArchived,
		IsTrashed:        asset.IsTrashed,
		Duration:         duration,
		Checksum:         asset.Checksum,
		StackParentId:    func() *string { if asset.StackID != nil { s := asset.StackID.String(); return &s }; return nil }(),
		// TODO: Add ExifInfo, SmartInfo, Tags, People conversion
	}
}