package server

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) GetTimeBucket(ctx context.Context, request *immichv1.GetTimeBucketRequest) (*immichv1.TimeBucketAssetResponseDto, error) {
	return &immichv1.TimeBucketAssetResponseDto{
		Assets:     []*immichv1.Asset{},
		TimeBucket: "2023-10-01T00:00:00Z",
	}, nil
}

func (s *Server) GetTimeBuckets(ctx context.Context, request *immichv1.GetTimeBucketsRequest) (*immichv1.GetTimeBucketsResponse, error) {
	return &immichv1.GetTimeBucketsResponse{
		Buckets: []*immichv1.TimeBucketsResponseDto{
			{
				TimeBucket: "2023-10-01T00:00:00Z",
				Count:      10,
			},
		},
	}, nil
}
