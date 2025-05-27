package server

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) SearchMemories(ctx context.Context, request *immichv1.SearchMemoriesRequest) (*immichv1.SearchMemoriesResponse, error) {
	return &immichv1.SearchMemoriesResponse{
		Memories: []*immichv1.Memory{
			{
				Assets: []*immichv1.Asset{},
			},
		},
	}, nil
}

func (s *Server) CreateMemory(ctx context.Context, request *immichv1.CreateMemoryRequest) (*immichv1.Memory, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetMemory(ctx context.Context, request *immichv1.GetMemoryRequest) (*immichv1.Memory, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) UpdateMemory(ctx context.Context, request *immichv1.UpdateMemoryRequest) (*immichv1.Memory, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) DeleteMemory(ctx context.Context, request *immichv1.DeleteMemoryRequest) (*emptypb.Empty, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) AddMemoryAssets(ctx context.Context, request *immichv1.AddMemoryAssetsRequest) (*immichv1.BulkIdResponseList, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) RemoveMemoryAssets(ctx context.Context, request *immichv1.RemoveMemoryAssetsRequest) (*immichv1.BulkIdResponseList, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}
