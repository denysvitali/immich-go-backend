package tags

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the TagsService with stub responses
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

// GetAllTags returns empty list (stub)
func (s *Server) GetAllTags(ctx context.Context, request *immichv1.GetAllTagsRequest) (*immichv1.GetAllTagsResponse, error) {
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	return &immichv1.GetAllTagsResponse{
		Tags: []*immichv1.TagResponse{},
	}, nil
}

// CreateTag creates a stub tag
func (s *Server) CreateTag(ctx context.Context, request *immichv1.CreateTagRequest) (*immichv1.TagResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	return &immichv1.TagResponse{
		Id:        uuid.New().String(),
		Name:      request.GetName(),
		UserId:    claims.UserID,
		CreatedAt: timestamppb.Now(),
		UpdatedAt: timestamppb.Now(),
	}, nil
}

// UpsertTags returns empty response (stub)
func (s *Server) UpsertTags(ctx context.Context, request *immichv1.UpsertTagsRequest) (*immichv1.UpsertTagsResponse, error) {
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	return &immichv1.UpsertTagsResponse{
		Tags: []*immichv1.TagResponse{},
	}, nil
}

// BulkTagAssets returns success (stub)
func (s *Server) BulkTagAssets(ctx context.Context, request *immichv1.BulkTagAssetsRequest) (*immichv1.BulkTagAssetsResponse, error) {
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	return &immichv1.BulkTagAssetsResponse{
		// Field not in proto, return empty response
	}, nil
}

// DeleteTag returns success (stub)
func (s *Server) DeleteTag(ctx context.Context, request *immichv1.DeleteTagRequest) (*emptypb.Empty, error) {
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	return &emptypb.Empty{}, nil
}

// GetTagById returns not found (stub)
func (s *Server) GetTagById(ctx context.Context, request *immichv1.GetTagByIdRequest) (*immichv1.TagResponse, error) {
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	return nil, status.Error(codes.NotFound, "tag not found")
}

// UpdateTag returns not implemented (stub)
func (s *Server) UpdateTag(ctx context.Context, request *immichv1.UpdateTagRequest) (*immichv1.TagResponse, error) {
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// UntagAssets returns success (stub)
func (s *Server) UntagAssets(ctx context.Context, request *immichv1.UntagAssetsRequest) (*immichv1.UntagAssetsResponse, error) {
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Return empty response
	return &immichv1.UntagAssetsResponse{}, nil
}

// TagAssets returns success (stub)
func (s *Server) TagAssets(ctx context.Context, request *immichv1.TagAssetsRequest) (*immichv1.TagAssetsResponse, error) {
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Return empty response
	return &immichv1.TagAssetsResponse{}, nil
}
