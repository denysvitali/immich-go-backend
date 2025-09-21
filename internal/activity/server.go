package activity

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse album ID
	if request.AlbumId == "" {
		return nil, status.Error(codes.InvalidArgument, "album_id is required")
	}

	albumID, err := uuid.Parse(request.AlbumId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid album ID")
	}

	albumUUID := pgtype.UUID{Bytes: albumID, Valid: true}

	// Set default limit and offset
	limit := int32(20)
	offset := int32(0)

	// Get activities from database
	activities, err := s.queries.GetAlbumActivity(ctx, sqlc.GetAlbumActivityParams{
		AlbumId: albumUUID,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get activities: %v", err)
	}

	// Convert to proto response
	responseActivities := make([]*immichv1.ActivityResponseDto, 0, len(activities))
	for _, activity := range activities {
		var assetId *string
		if activity.AssetId.Valid {
			aid := uuid.UUID(activity.AssetId.Bytes).String()
			assetId = &aid
		}

		// Determine activity type based on whether it's a comment or like
		activityType := immichv1.ReactionType_REACTION_TYPE_COMMENT
		if activity.IsLiked {
			activityType = immichv1.ReactionType_REACTION_TYPE_LIKE
		}

		var assetIdStr string
		if assetId != nil {
			assetIdStr = *assetId
		}

		responseActivities = append(responseActivities, &immichv1.ActivityResponseDto{
			Id:        uuid.UUID(activity.ID.Bytes).String(),
			CreatedAt: timestamppb.New(activity.CreatedAt.Time),
			Type:      activityType,
			Comment:   activity.Comment.String, // May be empty for likes
			AssetId:   assetIdStr,
			User: &immichv1.User{
				Id:    uuid.UUID(activity.UserId.Bytes).String(),
				Email: activity.UserEmail,
				Name:  activity.UserName,
			},
		})
	}

	return &immichv1.GetActivitiesResponse{
		Activities: responseActivities,
	}, nil
}

// CreateActivity creates a new activity (comment/like)
func (s *Server) CreateActivity(ctx context.Context, request *immichv1.CreateActivityRequest) (*immichv1.ActivityResponseDto, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid user ID")
	}

	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Parse album ID
	if request.AlbumId == "" {
		return nil, status.Error(codes.InvalidArgument, "album_id is required")
	}

	albumID, err := uuid.Parse(request.AlbumId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid album ID")
	}

	albumUUID := pgtype.UUID{Bytes: albumID, Valid: true}

	// Parse asset ID if provided
	var assetUUID pgtype.UUID
	if request.AssetId != nil && *request.AssetId != "" {
		assetID, err := uuid.Parse(*request.AssetId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid asset ID")
		}
		assetUUID = pgtype.UUID{Bytes: assetID, Valid: true}
	}

	// Determine if this is a like or comment
	isLiked := request.GetType() == immichv1.ReactionType_REACTION_TYPE_LIKE
	var comment pgtype.Text
	if request.Comment != "" {
		comment = pgtype.Text{String: request.Comment, Valid: true}
	}

	// Create activity in database
	createdActivity, err := s.queries.CreateActivity(ctx, sqlc.CreateActivityParams{
		UserId:  userUUID,
		AlbumId: albumUUID,
		AssetId: assetUUID,
		Comment: comment,
		IsLiked: isLiked,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create activity: %v", err)
	}

	// Get user info for response
	user, err := s.queries.GetUserByID(ctx, userUUID)
	if err != nil {
		// Use claims if user lookup fails
		user.Email = claims.Email
		user.Name = claims.Email
	}

	var assetIdStr string
	if createdActivity.AssetId.Valid {
		assetIdStr = uuid.UUID(createdActivity.AssetId.Bytes).String()
	}

	return &immichv1.ActivityResponseDto{
		Id:        uuid.UUID(createdActivity.ID.Bytes).String(),
		CreatedAt: timestamppb.New(createdActivity.CreatedAt.Time),
		Type:      request.GetType(),
		Comment:   comment.String,
		AssetId:   assetIdStr,
		User: &immichv1.User{
			Id:    claims.UserID,
			Email: user.Email,
			Name:  user.Name,
		},
	}, nil
}

// GetActivityStatistics gets statistics for activities
func (s *Server) GetActivityStatistics(ctx context.Context, request *immichv1.GetActivityStatisticsRequest) (*immichv1.ActivityStatisticsResponseDto, error) {
	// Get user from context
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse album ID
	if request.AlbumId == "" {
		return nil, status.Error(codes.InvalidArgument, "album_id is required")
	}

	albumID, err := uuid.Parse(request.AlbumId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid album ID")
	}

	albumUUID := pgtype.UUID{Bytes: albumID, Valid: true}

	// Get all activities for the album to count them
	// In a real implementation, we would have a dedicated count query
	activities, err := s.queries.GetAlbumActivity(ctx, sqlc.GetAlbumActivityParams{
		AlbumId: albumUUID,
		Limit:   1000, // Get up to 1000 activities
		Offset:  0,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get activity statistics: %v", err)
	}

	// Count comments (activities that are not likes)
	commentCount := int32(0)
	for _, activity := range activities {
		if !activity.IsLiked && activity.Comment.Valid {
			commentCount++
		}
	}

	return &immichv1.ActivityStatisticsResponseDto{
		Comments: commentCount,
	}, nil
}

// DeleteActivity deletes an activity
func (s *Server) DeleteActivity(ctx context.Context, request *immichv1.DeleteActivityRequest) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse activity ID
	activityID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid activity ID")
	}

	activityUUID := pgtype.UUID{Bytes: activityID, Valid: true}

	// Get the activity to verify ownership
	activity, err := s.queries.GetActivity(ctx, activityUUID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "activity not found")
	}

	// Verify the user owns this activity
	userID, _ := uuid.Parse(claims.UserID)
	if activity.UserId.Bytes != userID {
		return nil, status.Error(codes.PermissionDenied, "not authorized to delete this activity")
	}

	// Delete the activity
	err = s.queries.DeleteActivity(ctx, activityUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete activity: %v", err)
	}

	return &emptypb.Empty{}, nil
}
