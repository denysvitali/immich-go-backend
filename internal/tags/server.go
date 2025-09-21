package tags

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

// Server implements the TagsService with real database operations
type Server struct {
	immichv1.UnimplementedTagsServiceServer
	queries *sqlc.Queries
}

// NewServer creates a new tags server
func NewServer(queries *sqlc.Queries) *Server {
	return &Server{
		queries: queries,
	}
}

// GetAllTags returns all tags for the authenticated user
func (s *Server) GetAllTags(ctx context.Context, request *immichv1.GetAllTagsRequest) (*immichv1.GetAllTagsResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Get all tags for the user from database
	tags, err := s.queries.GetTags(ctx, userUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get tags: %v", err)
	}

	// Convert to proto response
	protoTags := make([]*immichv1.TagResponse, len(tags))
	for i, tag := range tags {
		protoTags[i] = &immichv1.TagResponse{
			Id:        uuid.UUID(tag.ID.Bytes).String(),
			Name:      tag.Value,
			UserId:    uuid.UUID(tag.UserId.Bytes).String(),
			CreatedAt: timestamppb.New(tag.CreatedAt.Time),
			UpdatedAt: timestamppb.New(tag.UpdatedAt.Time),
		}
		if tag.Color.Valid {
			color := tag.Color.String
			protoTags[i].Color = &color
		}
	}

	return &immichv1.GetAllTagsResponse{
		Tags: protoTags,
	}, nil
}

// CreateTag creates a new tag in the database
func (s *Server) CreateTag(ctx context.Context, request *immichv1.CreateTagRequest) (*immichv1.TagResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Prepare tag creation params
	params := sqlc.CreateTagParams{
		UserId: userUUID,
		Value:  request.GetName(),
	}

	// Set color if provided
	if request.Color != nil && *request.Color != "" {
		params.Color = pgtype.Text{String: *request.Color, Valid: true}
	}

	// Create tag in database
	tag, err := s.queries.CreateTag(ctx, params)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create tag: %v", err)
	}

	// Convert to proto response
	response := &immichv1.TagResponse{
		Id:        uuid.UUID(tag.ID.Bytes).String(),
		Name:      tag.Value,
		UserId:    uuid.UUID(tag.UserId.Bytes).String(),
		CreatedAt: timestamppb.New(tag.CreatedAt.Time),
		UpdatedAt: timestamppb.New(tag.UpdatedAt.Time),
	}
	if tag.Color.Valid {
		color := tag.Color.String
		response.Color = &color
	}

	return response, nil
}

// UpsertTags creates or updates multiple tags
func (s *Server) UpsertTags(ctx context.Context, request *immichv1.UpsertTagsRequest) (*immichv1.UpsertTagsResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	var tags []*immichv1.TagResponse

	// Process each tag to upsert
	for _, tagName := range request.GetTags() {
		// First try to find existing tag
		userTags, err := s.queries.GetTags(ctx, userUUID)
		if err == nil {
			for _, existingTag := range userTags {
				if existingTag.Value == tagName.GetName() {
					// Tag exists, add to response
					response := &immichv1.TagResponse{
						Id:        uuid.UUID(existingTag.ID.Bytes).String(),
						Name:      existingTag.Value,
						UserId:    uuid.UUID(existingTag.UserId.Bytes).String(),
						CreatedAt: timestamppb.New(existingTag.CreatedAt.Time),
						UpdatedAt: timestamppb.New(existingTag.UpdatedAt.Time),
					}
					if existingTag.Color.Valid {
						color := existingTag.Color.String
						response.Color = &color
					}
					tags = append(tags, response)
					continue
				}
			}
		}

		// Tag doesn't exist, create it
		newTag, err := s.queries.CreateTag(ctx, sqlc.CreateTagParams{
			UserId: userUUID,
			Value:  tagName.GetName(),
		})
		if err == nil {
			response := &immichv1.TagResponse{
				Id:        uuid.UUID(newTag.ID.Bytes).String(),
				Name:      newTag.Value,
				UserId:    uuid.UUID(newTag.UserId.Bytes).String(),
				CreatedAt: timestamppb.New(newTag.CreatedAt.Time),
				UpdatedAt: timestamppb.New(newTag.UpdatedAt.Time),
			}
			if newTag.Color.Valid {
				color := newTag.Color.String
				response.Color = &color
			}
			tags = append(tags, response)
		}
	}

	return &immichv1.UpsertTagsResponse{
		Tags: tags,
	}, nil
}

// BulkTagAssets performs bulk tagging operations
func (s *Server) BulkTagAssets(ctx context.Context, request *immichv1.BulkTagAssetsRequest) (*immichv1.BulkTagAssetsResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID for ownership checks
	userID, _ := uuid.Parse(claims.UserID)

	// Process bulk operations
	for _, tagID := range request.GetTagIds() {
		// Parse tag ID
		tagUUID, err := uuid.Parse(tagID)
		if err != nil {
			continue
		}
		tagPgUUID := pgtype.UUID{Bytes: tagUUID, Valid: true}

		// Verify tag ownership
		tag, err := s.queries.GetTag(ctx, tagPgUUID)
		if err != nil || tag.UserId.Bytes != userID {
			continue
		}

		// Add tag to each asset
		for _, assetID := range request.GetAssetIds() {
			// Parse asset ID
			assetUUID, err := uuid.Parse(assetID)
			if err != nil {
				continue
			}
			assetPgUUID := pgtype.UUID{Bytes: assetUUID, Valid: true}

			// Add tag to asset (ignore errors for bulk operation)
			_ = s.queries.AddTagToAsset(ctx, sqlc.AddTagToAssetParams{
				TagsId:   tagPgUUID,
				AssetsId: assetPgUUID,
			})
		}
	}

	return &immichv1.BulkTagAssetsResponse{}, nil
}

// DeleteTag deletes a tag from the database
func (s *Server) DeleteTag(ctx context.Context, request *immichv1.DeleteTagRequest) (*emptypb.Empty, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse tag ID
	tagID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tag ID")
	}
	tagUUID := pgtype.UUID{Bytes: tagID, Valid: true}

	// First verify the tag belongs to the user
	tag, err := s.queries.GetTag(ctx, tagUUID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "tag not found")
	}

	// Check ownership
	userID, _ := uuid.Parse(claims.UserID)
	if tag.UserId.Bytes != userID {
		return nil, status.Error(codes.PermissionDenied, "tag does not belong to user")
	}

	// Delete the tag
	err = s.queries.DeleteTag(ctx, tagUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete tag: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// GetTagById retrieves a specific tag by ID
func (s *Server) GetTagById(ctx context.Context, request *immichv1.GetTagByIdRequest) (*immichv1.TagResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse tag ID
	tagID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tag ID")
	}
	tagUUID := pgtype.UUID{Bytes: tagID, Valid: true}

	// Get tag from database
	tag, err := s.queries.GetTag(ctx, tagUUID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "tag not found")
	}

	// Verify the tag belongs to the user
	userID, _ := uuid.Parse(claims.UserID)
	if tag.UserId.Bytes != userID {
		return nil, status.Error(codes.PermissionDenied, "tag does not belong to user")
	}

	// Convert to proto response
	response := &immichv1.TagResponse{
		Id:        uuid.UUID(tag.ID.Bytes).String(),
		Name:      tag.Value,
		UserId:    uuid.UUID(tag.UserId.Bytes).String(),
		CreatedAt: timestamppb.New(tag.CreatedAt.Time),
		UpdatedAt: timestamppb.New(tag.UpdatedAt.Time),
	}
	if tag.Color.Valid {
		color := tag.Color.String
		response.Color = &color
	}

	return response, nil
}

// UpdateTag updates an existing tag
func (s *Server) UpdateTag(ctx context.Context, request *immichv1.UpdateTagRequest) (*immichv1.TagResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse tag ID
	tagID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tag ID")
	}
	tagUUID := pgtype.UUID{Bytes: tagID, Valid: true}

	// First verify the tag belongs to the user
	existingTag, err := s.queries.GetTag(ctx, tagUUID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "tag not found")
	}

	// Check ownership
	userID, _ := uuid.Parse(claims.UserID)
	if existingTag.UserId.Bytes != userID {
		return nil, status.Error(codes.PermissionDenied, "tag does not belong to user")
	}

	// Prepare update params
	params := sqlc.UpdateTagParams{
		ID:    tagUUID,
		Value: pgtype.Text{String: request.GetName(), Valid: true},
	}

	// Set color if provided
	if request.Color != nil && *request.Color != "" {
		params.Color = pgtype.Text{String: *request.Color, Valid: true}
	} else {
		params.Color = existingTag.Color
	}

	// Update tag in database
	tag, err := s.queries.UpdateTag(ctx, params)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update tag: %v", err)
	}

	// Convert to proto response
	response := &immichv1.TagResponse{
		Id:        uuid.UUID(tag.ID.Bytes).String(),
		Name:      tag.Value,
		UserId:    uuid.UUID(tag.UserId.Bytes).String(),
		CreatedAt: timestamppb.New(tag.CreatedAt.Time),
		UpdatedAt: timestamppb.New(tag.UpdatedAt.Time),
	}
	if tag.Color.Valid {
		color := tag.Color.String
		response.Color = &color
	}

	return response, nil
}

// UntagAssets removes tags from multiple assets
func (s *Server) UntagAssets(ctx context.Context, request *immichv1.UntagAssetsRequest) (*immichv1.UntagAssetsResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID for ownership checks
	userID, _ := uuid.Parse(claims.UserID)

	count := int32(0)

	// Parse tag ID
	tagUUID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tag ID")
	}
	tagPgUUID := pgtype.UUID{Bytes: tagUUID, Valid: true}

	// Verify tag ownership
	tag, err := s.queries.GetTag(ctx, tagPgUUID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "tag not found")
	}
	if tag.UserId.Bytes != userID {
		return nil, status.Error(codes.PermissionDenied, "tag does not belong to user")
	}

	// Remove tag from each asset
	for _, assetID := range request.GetAssetIds() {
		// Parse asset ID
		assetUUID, err := uuid.Parse(assetID)
		if err != nil {
			continue // Skip invalid IDs
		}
		assetPgUUID := pgtype.UUID{Bytes: assetUUID, Valid: true}

		// Remove tag from asset
		err = s.queries.RemoveTagFromAsset(ctx, sqlc.RemoveTagFromAssetParams{
			TagsId:   tagPgUUID,
			AssetsId: assetPgUUID,
		})
		if err == nil {
			count++
		}
	}

	return &immichv1.UntagAssetsResponse{
		Count: count,
	}, nil
}

// TagAssets adds tags to multiple assets
func (s *Server) TagAssets(ctx context.Context, request *immichv1.TagAssetsRequest) (*immichv1.TagAssetsResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Parse user ID for ownership checks
	userID, _ := uuid.Parse(claims.UserID)

	count := int32(0)

	// Parse tag ID
	tagUUID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tag ID")
	}
	tagPgUUID := pgtype.UUID{Bytes: tagUUID, Valid: true}

	// Verify tag ownership
	tag, err := s.queries.GetTag(ctx, tagPgUUID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "tag not found")
	}
	if tag.UserId.Bytes != userID {
		return nil, status.Error(codes.PermissionDenied, "tag does not belong to user")
	}

	// Add tag to each asset
	for _, assetID := range request.GetAssetIds() {
		// Parse asset ID
		assetUUID, err := uuid.Parse(assetID)
		if err != nil {
			continue // Skip invalid IDs
		}
		assetPgUUID := pgtype.UUID{Bytes: assetUUID, Valid: true}

		// Add tag to asset
		err = s.queries.AddTagToAsset(ctx, sqlc.AddTagToAssetParams{
			TagsId:   tagPgUUID,
			AssetsId: assetPgUUID,
		})
		if err == nil {
			count++
		}
	}

	return &immichv1.TagAssetsResponse{
		Count: count,
	}, nil
}
