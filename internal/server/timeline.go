package server

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Server) GetTimeBucket(ctx context.Context, request *immichv1.GetTimeBucketRequest) (*immichv1.TimeBucketAssetResponseDto, error) {
	// Get user from context
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Return empty response
	return &immichv1.TimeBucketAssetResponseDto{
		Assets:     []*immichv1.Asset{},
		TimeBucket: request.GetTimeBucket(),
	}, nil
}

func (s *Server) GetTimeBuckets(ctx context.Context, request *immichv1.GetTimeBucketsRequest) (*immichv1.GetTimeBucketsResponse, error) {
	// Get user from context
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Return stub response with some fake data
	return &immichv1.GetTimeBucketsResponse{
		Buckets: []*immichv1.TimeBucketsResponseDto{
			{
				TimeBucket: "2024-01-01T00:00:00Z",
				Count:      5,
			},
		},
	}, nil
}
