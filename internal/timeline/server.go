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

	// Build options from request
	opts := TimelineOptions{
		UserID:     claims.UserID,
		AlbumID:    req.GetAlbumId(),
		IsFavorite: req.GetIsFavorite(),
		TimeBucket: "day", // Default to day
		Limit:      50,    // Default limit
		Offset:     0,
	}

	// Handle pagination
	if req.Page != nil && req.PageSize != nil {
		opts.Limit = int(*req.PageSize)
		opts.Offset = int(*req.Page) * int(*req.PageSize)
	}

	// Handle partner sharing
	if req.WithPartners != nil && *req.WithPartners {
		// Would need to fetch partner IDs here
		opts.PartnerIDs = []string{}
	}

	// Get assets for the time bucket
	assetIDs, err := s.service.GetTimelineAssets(ctx, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get timeline assets: %v", err)
	}

	// Convert to proto assets (simplified - in production would fetch full asset data)
	var assets []*immichv1.Asset
	for _, id := range assetIDs {
		assets = append(assets, &immichv1.Asset{
			Id: id,
			// Other fields would be populated from database
		})
	}

	return &immichv1.TimeBucketAssetResponseDto{
		Assets:     assets,
		TimeBucket: req.TimeBucket,
		Count:      int32(len(assets)),
	}, nil
}

// GetTimeBuckets returns time buckets with asset counts
func (s *Server) GetTimeBuckets(ctx context.Context, req *immichv1.GetTimeBucketsRequest) (*immichv1.GetTimeBucketsResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Build options from request
	opts := TimelineOptions{
		UserID:     claims.UserID,
		AlbumID:    req.GetAlbumId(),
		IsFavorite: req.GetIsFavorite(),
		TimeBucket: "day", // Default to day buckets
	}

	// Handle partner sharing
	if req.WithPartners != nil && *req.WithPartners {
		// Would need to fetch partner IDs here
		opts.PartnerIDs = []string{}
	}

	// Get time buckets
	buckets, err := s.service.GetTimeBuckets(ctx, opts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get time buckets: %v", err)
	}

	// Convert to proto buckets
	var protoBuckets []*immichv1.TimeBucketsResponseDto
	for _, bucket := range buckets {
		protoBuckets = append(protoBuckets, &immichv1.TimeBucketsResponseDto{
			Count:      int32(bucket.Count),
			TimeBucket: bucket.Date,
		})
	}

	return &immichv1.GetTimeBucketsResponse{
		Buckets: protoBuckets,
	}, nil
}
