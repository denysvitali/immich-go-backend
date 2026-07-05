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

type tagQueries interface {
	GetTags(ctx context.Context, userid pgtype.UUID) ([]sqlc.Tag, error)
	CreateTag(ctx context.Context, arg sqlc.CreateTagParams) (sqlc.Tag, error)
	GetTag(ctx context.Context, id pgtype.UUID) (sqlc.Tag, error)
	DeleteTag(ctx context.Context, id pgtype.UUID) error
	UpdateTag(ctx context.Context, arg sqlc.UpdateTagParams) (sqlc.Tag, error)
	AddTagToAsset(ctx context.Context, arg sqlc.AddTagToAssetParams) error
	RemoveTagFromAsset(ctx context.Context, arg sqlc.RemoveTagFromAssetParams) error
}

// Server implements the TagsService with real database operations
type Server struct {
	immichv1.UnimplementedTagsServiceServer
	queries tagQueries
}

// NewServer creates a new tags server
func NewServer(queries *sqlc.Queries) *Server {
	return &Server{
		queries: queries,
	}
}

func currentUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	return auth.GetUserIDFromContext(ctx)
}

func currentUserUUIDFromContext(ctx context.Context) (pgtype.UUID, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return pgtype.UUID{}, err
	}

	return pgUUID(userID), nil
}

func parseUUIDParam(value, errMsg string) (pgtype.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return pgtype.UUID{}, status.Error(codes.InvalidArgument, errMsg)
	}

	return pgUUID(id), nil
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func tagResponse(tag sqlc.Tag) *immichv1.TagResponse {
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

	return response
}

func tagResponses(tags []sqlc.Tag) []*immichv1.TagResponse {
	protoTags := make([]*immichv1.TagResponse, len(tags))
	for i, tag := range tags {
		protoTags[i] = tagResponse(tag)
	}

	return protoTags
}

func tagBelongsToUser(tag sqlc.Tag, userID uuid.UUID) bool {
	return tag.UserId.Bytes == userID
}

func (s *Server) getOwnedTag(ctx context.Context, tagUUID pgtype.UUID, userID uuid.UUID) (sqlc.Tag, error) {
	tag, err := s.queries.GetTag(ctx, tagUUID)
	if err != nil {
		return sqlc.Tag{}, status.Error(codes.NotFound, "tag not found")
	}
	if !tagBelongsToUser(tag, userID) {
		return sqlc.Tag{}, status.Error(codes.PermissionDenied, "tag does not belong to user")
	}

	return tag, nil
}

// GetAllTags returns all tags for the authenticated user
func (s *Server) GetAllTags(ctx context.Context, request *immichv1.GetAllTagsRequest) (*immichv1.GetAllTagsResponse, error) {
	userUUID, err := currentUserUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Get all tags for the user from database
	tags, err := s.queries.GetTags(ctx, userUUID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get tags: %v", err)
	}

	return &immichv1.GetAllTagsResponse{
		Tags: tagResponses(tags),
	}, nil
}

// CreateTag creates a new tag in the database
func (s *Server) CreateTag(ctx context.Context, request *immichv1.CreateTagRequest) (*immichv1.TagResponse, error) {
	userUUID, err := currentUserUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

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

	return tagResponse(tag), nil
}

// UpsertTags creates or updates multiple tags
func (s *Server) UpsertTags(ctx context.Context, request *immichv1.UpsertTagsRequest) (*immichv1.UpsertTagsResponse, error) {
	userUUID, err := currentUserUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var tags []*immichv1.TagResponse
	existingByName := make(map[string]sqlc.Tag)
	if userTags, err := s.queries.GetTags(ctx, userUUID); err == nil {
		for _, tag := range userTags {
			existingByName[tag.Value] = tag
		}
	}

	// Process each tag to upsert
	for _, tagUpsert := range request.GetTags() {
		tagName := tagUpsert.GetName()
		if existingTag, ok := existingByName[tagName]; ok {
			tags = append(tags, tagResponse(existingTag))
			continue
		}

		// Tag doesn't exist, create it
		newTag, err := s.queries.CreateTag(ctx, sqlc.CreateTagParams{
			UserId: userUUID,
			Value:  tagName,
		})
		if err == nil {
			existingByName[tagName] = newTag
			tags = append(tags, tagResponse(newTag))
		}
	}

	return &immichv1.UpsertTagsResponse{
		Tags: tags,
	}, nil
}

// BulkTagAssets performs bulk tagging operations
func (s *Server) BulkTagAssets(ctx context.Context, request *immichv1.BulkTagAssetsRequest) (*immichv1.BulkTagAssetsResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	count := int32(0)

	// Process bulk operations
	for _, tagID := range request.GetTagIds() {
		// Parse tag ID
		tagPgUUID, err := parseUUIDParam(tagID, "invalid tag ID")
		if err != nil {
			continue
		}

		// Verify tag ownership
		tag, err := s.queries.GetTag(ctx, tagPgUUID)
		if err != nil || !tagBelongsToUser(tag, userID) {
			continue
		}

		// Add tag to each asset
		for _, assetID := range request.GetAssetIds() {
			// Parse asset ID
			assetUUID, err := uuid.Parse(assetID)
			if err != nil {
				continue
			}
			assetPgUUID := pgUUID(assetUUID)

			// Add tag to asset (ignore errors for bulk operation)
			err = s.queries.AddTagToAsset(ctx, sqlc.AddTagToAssetParams{
				TagsId:   tagPgUUID,
				AssetsId: assetPgUUID,
			})
			if err == nil {
				count++
			}
		}
	}

	return &immichv1.BulkTagAssetsResponse{Count: count}, nil
}

// DeleteTag deletes a tag from the database
func (s *Server) DeleteTag(ctx context.Context, request *immichv1.DeleteTagRequest) (*emptypb.Empty, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Parse tag ID
	tagUUID, err := parseUUIDParam(request.GetId(), "invalid tag ID")
	if err != nil {
		return nil, err
	}

	// First verify the tag belongs to the user
	if _, err := s.getOwnedTag(ctx, tagUUID, userID); err != nil {
		return nil, err
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
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Parse tag ID
	tagUUID, err := parseUUIDParam(request.GetId(), "invalid tag ID")
	if err != nil {
		return nil, err
	}

	// Get tag from database
	tag, err := s.getOwnedTag(ctx, tagUUID, userID)
	if err != nil {
		return nil, err
	}

	return tagResponse(tag), nil
}

// UpdateTag updates an existing tag
func (s *Server) UpdateTag(ctx context.Context, request *immichv1.UpdateTagRequest) (*immichv1.TagResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Parse tag ID
	tagUUID, err := parseUUIDParam(request.GetId(), "invalid tag ID")
	if err != nil {
		return nil, err
	}

	// First verify the tag belongs to the user
	existingTag, err := s.getOwnedTag(ctx, tagUUID, userID)
	if err != nil {
		return nil, err
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

	return tagResponse(tag), nil
}

// UntagAssets removes tags from multiple assets
func (s *Server) UntagAssets(ctx context.Context, request *immichv1.UntagAssetsRequest) (*immichv1.UntagAssetsResponse, error) {
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	count := int32(0)

	// Parse tag ID
	tagPgUUID, err := parseUUIDParam(request.GetId(), "invalid tag ID")
	if err != nil {
		return nil, err
	}

	// Verify tag ownership
	if _, err := s.getOwnedTag(ctx, tagPgUUID, userID); err != nil {
		return nil, err
	}

	// Remove tag from each asset
	for _, assetID := range request.GetAssetIds() {
		// Parse asset ID
		assetUUID, err := uuid.Parse(assetID)
		if err != nil {
			continue // Skip invalid IDs
		}
		assetPgUUID := pgUUID(assetUUID)

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
	userID, err := currentUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	count := int32(0)

	// Parse tag ID
	tagPgUUID, err := parseUUIDParam(request.GetId(), "invalid tag ID")
	if err != nil {
		return nil, err
	}

	// Verify tag ownership
	if _, err := s.getOwnedTag(ctx, tagPgUUID, userID); err != nil {
		return nil, err
	}

	// Add tag to each asset
	for _, assetID := range request.GetAssetIds() {
		// Parse asset ID
		assetUUID, err := uuid.Parse(assetID)
		if err != nil {
			continue // Skip invalid IDs
		}
		assetPgUUID := pgUUID(assetUUID)

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
