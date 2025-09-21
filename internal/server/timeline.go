package server

import (
	"context"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/timeline"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Server) GetTimeBucket(ctx context.Context, request *immichv1.GetTimeBucketRequest) (*immichv1.TimeBucketAssetResponseDto, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse the time bucket string to get the date
	bucketTime, err := time.Parse(time.RFC3339, request.GetTimeBucket())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid time bucket format")
	}

	// Get day detail from timeline service
	dayDetail, err := s.timelineService.GetDayDetail(ctx, claims.UserID, bucketTime)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get time bucket: %v", err)
	}

	// Get assets for this day
	opts := timeline.TimelineOptions{
		UserID:   claims.UserID,
		StartDate: &dayDetail.StartDate,
		EndDate:   &dayDetail.EndDate,
		Limit:    100,
		Offset:   0,
	}

	assetIDs, err := s.timelineService.GetTimelineAssets(ctx, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get assets: %v", err)
	}

	// Convert asset IDs to Asset objects
	// For now, return empty assets as we need full asset details
	assets := make([]*immichv1.Asset, 0)
	_ = assetIDs

	return &immichv1.TimeBucketAssetResponseDto{
		Assets:     assets,
		TimeBucket: request.GetTimeBucket(),
	}, nil
}

func (s *Server) GetTimeBuckets(ctx context.Context, request *immichv1.GetTimeBucketsRequest) (*immichv1.GetTimeBucketsResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Build timeline options from request
	opts := timeline.TimelineOptions{
		UserID:     claims.UserID,
		TimeBucket: "day", // Default to day buckets
		IsArchived: false, // Use IsTrashed field if needed
		IsFavorite: request.GetIsFavorite(),
	}

	// Handle trashed items
	if request.GetIsTrashed() {
		opts.IsArchived = true
	}

	// Get time buckets from timeline service
	buckets, err := s.timelineService.GetTimeBuckets(ctx, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get time buckets: %v", err)
	}

	// Convert to proto response
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
