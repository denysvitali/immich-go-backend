package activity

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server implements the ActivityService
type Server struct {
	immichv1.UnimplementedActivityServiceServer
	queries *sqlc.Queries
}

// NewServer creates a new activity server
func NewServer(queries *sqlc.Queries) *Server {
	return &Server{
		queries: queries,
	}
}

// GetActivities gets activities for albums/assets
func (s *Server) GetActivities(ctx context.Context, request *immichv1.GetActivitiesRequest) (*immichv1.GetActivitiesResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual activity retrieval based on album_id, asset_id, level, type
	// For now, return empty activities list
	return &immichv1.GetActivitiesResponse{
		Activities: []*immichv1.ActivityResponseDto{},
	}, nil
}

// CreateActivity creates a new activity (comment/like)
func (s *Server) CreateActivity(ctx context.Context, request *immichv1.CreateActivityRequest) (*immichv1.ActivityResponseDto, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual activity creation
	// Validate album/asset exists and user has access
	// Create comment or like activity
	return nil, status.Error(codes.Unimplemented, "method not implemented")
}

// GetActivityStatistics gets statistics for activities
func (s *Server) GetActivityStatistics(ctx context.Context, request *immichv1.GetActivityStatisticsRequest) (*immichv1.ActivityStatisticsResponseDto, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual activity statistics
	// Count comments/likes for given album/asset
	return &immichv1.ActivityStatisticsResponseDto{
		Comments: 0,
	}, nil
}

// DeleteActivity deletes an activity
func (s *Server) DeleteActivity(ctx context.Context, request *immichv1.DeleteActivityRequest) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	_ = claims // TODO: Use claims when implementing actual functionality

	// TODO: Implement actual activity deletion
	// Validate activity exists and user owns it or has permission
	return &emptypb.Empty{}, nil
}
