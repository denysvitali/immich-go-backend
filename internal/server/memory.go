package server

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *Server) SearchMemories(ctx context.Context, request *immichv1.SearchMemoriesRequest) (*immichv1.SearchMemoriesResponse, error) {
	// Get user from context
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Return empty memories list
	return &immichv1.SearchMemoriesResponse{
		Memories: []*immichv1.Memory{},
	}, nil
}

func (s *Server) CreateMemory(ctx context.Context, request *immichv1.CreateMemoryRequest) (*immichv1.Memory, error) {
	// Get user from context
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Return stub response
	return &immichv1.Memory{
		Assets: []*immichv1.Asset{},
	}, nil
}

func (s *Server) GetMemory(ctx context.Context, request *immichv1.GetMemoryRequest) (*immichv1.Memory, error) {
	// Get user from context
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Return not found
	return nil, status.Error(codes.NotFound, "memory not found")
}

func (s *Server) UpdateMemory(ctx context.Context, request *immichv1.UpdateMemoryRequest) (*immichv1.Memory, error) {
	// Get user from context
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Return not implemented
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (s *Server) DeleteMemory(ctx context.Context, request *immichv1.DeleteMemoryRequest) (*emptypb.Empty, error) {
	// Get user from context
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Return success
	return &emptypb.Empty{}, nil
}

func (s *Server) AddMemoryAssets(ctx context.Context, request *immichv1.AddMemoryAssetsRequest) (*immichv1.BulkIdResponseList, error) {
	// Get user from context
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Return empty response
	return &immichv1.BulkIdResponseList{}, nil
}

func (s *Server) RemoveMemoryAssets(ctx context.Context, request *immichv1.RemoveMemoryAssetsRequest) (*immichv1.BulkIdResponseList, error) {
	// Get user from context
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Return empty response
	return &immichv1.BulkIdResponseList{}, nil
}