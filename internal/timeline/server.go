package timeline

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the TimelineService
type Server struct {
	immichv1.UnimplementedTimelineServiceServer
	service *Service
}

// NewServer creates a new timeline server
func NewServer(service *Service) *Server {
	return &Server{
		service: service,
	}
}

// GetTimeBucket returns assets for a specific time bucket
func (s *Server) GetTimeBucket(ctx context.Context, req *immichv1.GetTimeBucketRequest) (*immichv1.TimeBucketAssetResponseDto, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	opts := ListOptions{
		UserID:     claims.UserID,
		Bucket:     "day",
		Date:       req.GetTimeBucket(),
		IsFavorite: req.GetIsFavorite(),
		IsTrashed:  req.GetIsTrashed(),
		IsArchived: req.GetIsTrashed(),
		Limit:      500,
	}

	if req.PageSize != nil {
		opts.Limit = *req.PageSize
	}

	assets, err := s.service.GetBucketAssets(ctx, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get timeline assets: %v", err)
	}

	protoAssets := make([]*immichv1.Asset, len(assets))
	for i, asset := range assets {
		protoAssets[i] = &immichv1.Asset{
			Id: asset.ID.String(),
		}
	}

	return &immichv1.TimeBucketAssetResponseDto{
		Assets:     protoAssets,
		TimeBucket: req.GetTimeBucket(),
		Count:      int32(len(protoAssets)),
	}, nil
}

// GetTimeBuckets returns time buckets with asset counts
func (s *Server) GetTimeBuckets(ctx context.Context, req *immichv1.GetTimeBucketsRequest) (*immichv1.GetTimeBucketsResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	opts := ListOptions{
		UserID:     claims.UserID,
		Bucket:     "day",
		IsFavorite: req.GetIsFavorite(),
		IsTrashed:  req.GetIsTrashed(),
		IsArchived: req.GetIsTrashed(),
	}

	buckets, err := s.service.GetTimeBuckets(ctx, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get time buckets: %v", err)
	}

	protoBuckets := make([]*immichv1.TimeBucketsResponseDto, len(buckets))
	for i, bucket := range buckets {
		protoBuckets[i] = &immichv1.TimeBucketsResponseDto{
			Count:      int32(bucket.Count),
			TimeBucket: bucket.Date,
		}
	}

	return &immichv1.GetTimeBucketsResponse{
		Buckets: protoBuckets,
	}, nil
}
