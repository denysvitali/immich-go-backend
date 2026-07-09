package activity

import (
	"context"
	"errors"
	"strings"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/grpcutil"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid user ID")
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

	params := sqlc.SearchActivityParams{AlbumID: albumUUID}
	if request.UserId != nil && *request.UserId != "" {
		filterUserID, err := uuid.Parse(*request.UserId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid user ID")
		}
		params.UserID = pgtype.UUID{Bytes: filterUserID, Valid: true}
	}
	if request.Type != nil {
		switch request.GetType() {
		case immichv1.ReactionType_REACTION_TYPE_COMMENT:
			params.IsLiked = pgtype.Bool{Bool: false, Valid: true}
		case immichv1.ReactionType_REACTION_TYPE_LIKE:
			params.IsLiked = pgtype.Bool{Bool: true, Valid: true}
		default:
			return nil, status.Error(codes.InvalidArgument, "invalid activity type")
		}
	}

	switch request.GetLevel() {
	case immichv1.ReactionLevel_REACTION_LEVEL_UNSPECIFIED:
		if request.AssetId != nil && *request.AssetId != "" {
			assetID, err := uuid.Parse(*request.AssetId)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, "invalid asset ID")
			}
			params.FilterAsset = true
			params.AssetID = pgtype.UUID{Bytes: assetID, Valid: true}
		}
	case immichv1.ReactionLevel_REACTION_LEVEL_ALBUM:
		params.FilterAsset = true
	case immichv1.ReactionLevel_REACTION_LEVEL_ASSET:
		if request.AssetId != nil && *request.AssetId != "" {
			assetID, err := uuid.Parse(*request.AssetId)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, "invalid asset ID")
			}
			params.FilterAsset = true
			params.AssetID = pgtype.UUID{Bytes: assetID, Valid: true}
		}
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid activity level")
	}
	if err := s.requireAlbumAccess(ctx, userID, albumUUID); err != nil {
		return nil, err
	}

	// Get activities from database
	activities, err := s.queries.SearchActivity(ctx, params)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to get activities", err)
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

	// Determine if this is a like or comment.
	isLiked := false
	switch request.GetType() {
	case immichv1.ReactionType_REACTION_TYPE_COMMENT:
		if strings.TrimSpace(request.Comment) == "" {
			return nil, status.Error(codes.InvalidArgument, "comment is required for comment activities")
		}
	case immichv1.ReactionType_REACTION_TYPE_LIKE:
		isLiked = true
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid activity type")
	}
	if err := s.requireAlbumAccess(ctx, userID, albumUUID); err != nil {
		return nil, err
	}
	var comment pgtype.Text
	if !isLiked {
		comment = pgtype.Text{String: request.Comment, Valid: true}
	}

	if isLiked {
		existing, err := s.queries.GetActivityLike(ctx, sqlc.GetActivityLikeParams{
			UserID:  userUUID,
			AlbumID: albumUUID,
			AssetID: assetUUID,
		})
		if err == nil {
			return s.activityResponse(ctx, existing, claims)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, grpcutil.SanitizedInternal(ctx, "failed to check existing activity", err)
		}
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
		return nil, grpcutil.SanitizedInternal(ctx, "failed to create activity", err)
	}

	return s.activityResponse(ctx, createdActivity, claims)
}

// GetActivityStatistics gets statistics for activities
func (s *Server) GetActivityStatistics(ctx context.Context, request *immichv1.GetActivityStatisticsRequest) (*immichv1.ActivityStatisticsResponseDto, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid user ID")
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

	var assetUUID pgtype.UUID
	if request.AssetId != nil && *request.AssetId != "" {
		assetID, err := uuid.Parse(*request.AssetId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid asset ID")
		}
		assetUUID = pgtype.UUID{Bytes: assetID, Valid: true}
	}
	if err := s.requireAlbumAccess(ctx, userID, albumUUID); err != nil {
		return nil, err
	}

	statistics, err := s.queries.GetActivityStatistics(ctx, sqlc.GetActivityStatisticsParams{
		AlbumID: albumUUID,
		AssetID: assetUUID,
	})
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to get activity statistics", err)
	}

	return &immichv1.ActivityStatisticsResponseDto{
		Comments: statistics.Comments,
		Likes:    statistics.Likes,
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
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid user ID")
	}
	if activity.UserId.Bytes != userID {
		return nil, status.Error(codes.PermissionDenied, "not authorized to delete this activity")
	}

	// Delete the activity
	err = s.queries.DeleteActivity(ctx, activityUUID)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to delete activity", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) requireAlbumAccess(ctx context.Context, userID uuid.UUID, albumID pgtype.UUID) error {
	album, err := s.queries.GetAlbum(ctx, albumID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return status.Error(codes.NotFound, "album not found")
		}
		return grpcutil.SanitizedInternal(ctx, "failed to get album", err)
	}
	if album.OwnerId.Valid && album.OwnerId.Bytes == userID {
		return nil
	}

	sharedUsers, err := s.queries.GetAlbumSharedUsers(ctx, albumID)
	if err != nil {
		return grpcutil.SanitizedInternal(ctx, "failed to check album access", err)
	}
	for _, sharedUser := range sharedUsers {
		if sharedUser.ID.Valid && sharedUser.ID.Bytes == userID {
			return nil
		}
	}
	return status.Error(codes.PermissionDenied, "not authorized to access this album")
}

func (s *Server) activityResponse(ctx context.Context, activity sqlc.Activity, claims *auth.Claims) (*immichv1.ActivityResponseDto, error) {
	user, err := s.queries.GetUserByID(ctx, activity.UserId)
	if err != nil {
		user.Email = claims.Email
		user.Name = claims.Email
	}

	activityType := immichv1.ReactionType_REACTION_TYPE_COMMENT
	if activity.IsLiked {
		activityType = immichv1.ReactionType_REACTION_TYPE_LIKE
	}
	assetID := ""
	if activity.AssetId.Valid {
		assetID = uuid.UUID(activity.AssetId.Bytes).String()
	}

	return &immichv1.ActivityResponseDto{
		Id:        uuid.UUID(activity.ID.Bytes).String(),
		CreatedAt: timestamppb.New(activity.CreatedAt.Time),
		Type:      activityType,
		Comment:   activity.Comment.String,
		AssetId:   assetID,
		User: &immichv1.User{
			Id:    uuid.UUID(activity.UserId.Bytes).String(),
			Email: user.Email,
			Name:  user.Name,
		},
	}, nil
}
