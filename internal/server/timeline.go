package server

import (
	"context"
	"time"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/timeline"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Server) GetTimeBucket(ctx context.Context, request *immichv1.GetTimeBucketRequest) (*immichv1.TimeBucketAssetResponseDto, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	bucket, date, err := parseTimeBucket(request.GetTimeBucket())
	if err != nil {
		return nil, err
	}

	opts := timeline.ListOptions{
		UserID:     claims.UserID,
		Bucket:     bucketSizeForLayout(date),
		Date:       bucket.Format("2006-01-02"),
		IsFavorite: request.GetIsFavorite(),
		IsTrashed:  request.GetIsTrashed(),
		IsArchived: request.GetIsTrashed(),
		Limit:      500,
	}

	assets, err := s.timelineService.GetBucketAssets(ctx, opts)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get time bucket assets", err)
	}

	protoAssets := make([]*immichv1.Asset, len(assets))
	for i, asset := range assets {
		protoAssets[i] = s.convertBucketAssetToProto(asset)
	}

	return &immichv1.TimeBucketAssetResponseDto{
		Assets:     protoAssets,
		TimeBucket: request.GetTimeBucket(),
		Count:      int32(len(protoAssets)),
	}, nil
}

func (s *Server) GetTimeBuckets(ctx context.Context, request *immichv1.GetTimeBucketsRequest) (*immichv1.GetTimeBucketsResponse, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	opts := timeline.ListOptions{
		UserID:     claims.UserID,
		Bucket:     "day",
		IsFavorite: request.GetIsFavorite(),
		IsTrashed:  request.GetIsTrashed(),
		IsArchived: request.GetIsTrashed(),
	}

	buckets, err := s.timelineService.GetTimeBuckets(ctx, opts)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get time buckets", err)
	}

	protoBuckets := make([]*immichv1.TimeBucketsResponseDto, len(buckets))
	for i, bucket := range buckets {
		protoBuckets[i] = &immichv1.TimeBucketsResponseDto{
			TimeBucket: bucket.Date,
			Count:      int32(bucket.Count),
		}
	}

	return &immichv1.GetTimeBucketsResponse{
		Buckets: protoBuckets,
	}, nil
}

func (s *Server) convertBucketAssetToProto(asset timeline.BucketAsset) *immichv1.Asset {
	protoAsset := &immichv1.Asset{
		Id:               asset.ID.String(),
		DeviceAssetId:    asset.DeviceAssetId,
		OwnerId:          asset.OwnerId.String(),
		DeviceId:         asset.DeviceId,
		Type:             convertAssetTypeString(asset.Type),
		OriginalPath:     asset.OriginalPath,
		OriginalFileName: asset.OriginalFileName,
		CreatedAt:        timestamppb.New(asset.FileCreatedAt),
		UpdatedAt:        timestamppb.New(asset.FileModifiedAt),
		IsFavorite:       asset.IsFavorite,
		IsArchived:       asset.Visibility == "archive",
		IsTrashed:        asset.Status == "trashed",
		Checksum:         "",
	}

	if asset.Duration != nil {
		protoAsset.Duration = asset.Duration
	}
	if asset.LivePhotoVideoId != nil {
		id := asset.LivePhotoVideoId.String()
		protoAsset.LivePhotoVideoId = &id
	}
	if asset.StackId != nil {
		id := asset.StackId.String()
		protoAsset.StackParentId = &id
	}
	if asset.EncodedVideoPath != nil {
		protoAsset.EncodedVideoPath = asset.EncodedVideoPath
	}

	return protoAsset
}

func convertAssetTypeString(assetType string) immichv1.AssetType {
	switch assetType {
	case "IMAGE":
		return immichv1.AssetType_ASSET_TYPE_IMAGE
	case "VIDEO":
		return immichv1.AssetType_ASSET_TYPE_VIDEO
	case "AUDIO":
		return immichv1.AssetType_ASSET_TYPE_AUDIO
	default:
		return immichv1.AssetType_ASSET_TYPE_OTHER
	}
}

// parseTimeBucket tries to parse a time-bucket identifier as a date. It is
// permissive so that month/year values from the upstream SDK are accepted.
func parseTimeBucket(value string) (time.Time, string, error) {
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05Z07:00", "2006-01", "2006"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, layout, nil
		}
	}
	return time.Time{}, "", status.Error(codes.InvalidArgument, "invalid time bucket format")
}

func bucketSizeForLayout(layout string) string {
	switch layout {
	case "2006":
		return "year"
	case "2006-01":
		return "month"
	default:
		return "day"
	}
}
