package memories

import (
	"context"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the MemoryService
type Server struct {
	immichv1.UnimplementedMemoryServiceServer
	service *Service
}

// NewServer creates a new memories server
func NewServer(service *Service) *Server {
	return &Server{
		service: service,
	}
}

// SearchMemories returns memories based on search criteria
func (s *Server) SearchMemories(ctx context.Context, req *immichv1.SearchMemoriesRequest) (*immichv1.SearchMemoriesResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Get memories from service
	memories, err := s.service.GetMemories(ctx, claims.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get memories: %v", err)
	}

	// Convert to proto memories
	var protoMemories []*immichv1.Memory
	for _, mem := range memories {
		protoMem := &immichv1.Memory{
			Id:        mem.ID,
			OwnerId:   mem.UserID,
			Type:      immichv1.MemoryType_MEMORY_TYPE_ON_THIS_DAY,
			MemoryAt:  timestamppb.New(mem.Date),
			CreatedAt: timestamppb.New(mem.CreatedAt),
			UpdatedAt: timestamppb.New(mem.UpdatedAt),
			IsSaved:   false,
			Assets:    []*immichv1.Asset{}, // Empty for now
		}

		// Apply filters if provided
		if req.IsSaved != nil && *req.IsSaved != protoMem.IsSaved {
			continue
		}
		if req.Type != nil && *req.Type != protoMem.Type {
			continue
		}

		protoMemories = append(protoMemories, protoMem)
	}

	return &immichv1.SearchMemoriesResponse{
		Memories: protoMemories,
	}, nil
}

// CreateMemory creates a new memory
func (s *Server) CreateMemory(ctx context.Context, req *immichv1.CreateMemoryRequest) (*immichv1.Memory, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	memory := &Memory{
		UserID:   claims.UserID,
		Title:    "New Memory",
		Date:     req.MemoryAt.AsTime(),
		Type:     "on_this_day",
		AssetIDs: req.AssetIds,
	}

	created, err := s.service.CreateMemory(ctx, memory)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create memory: %v", err)
	}

	return &immichv1.Memory{
		Id:        created.ID,
		OwnerId:   created.UserID,
		Type:      req.Type,
		MemoryAt:  req.MemoryAt,
		CreatedAt: timestamppb.New(created.CreatedAt),
		UpdatedAt: timestamppb.New(created.UpdatedAt),
		IsSaved:   req.IsSaved != nil && *req.IsSaved,
		Assets:    []*immichv1.Asset{},
		Data: &immichv1.OnThisDayData{
			Year: int32(created.Date.Year()),
		},
	}, nil
}

// GetMemory gets a memory by ID
func (s *Server) GetMemory(ctx context.Context, req *immichv1.GetMemoryRequest) (*immichv1.Memory, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	memory, err := s.service.GetMemory(ctx, claims.UserID, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "memory not found: %v", err)
	}

	return &immichv1.Memory{
		Id:        memory.ID,
		OwnerId:   memory.UserID,
		Type:      immichv1.MemoryType_MEMORY_TYPE_ON_THIS_DAY,
		MemoryAt:  timestamppb.New(memory.Date),
		CreatedAt: timestamppb.New(memory.CreatedAt),
		UpdatedAt: timestamppb.New(memory.UpdatedAt),
		IsSaved:   false,
		Assets:    []*immichv1.Asset{},
		Data: &immichv1.OnThisDayData{
			Year: int32(memory.Date.Year()),
		},
	}, nil
}

// UpdateMemory updates a memory
func (s *Server) UpdateMemory(ctx context.Context, req *immichv1.UpdateMemoryRequest) (*immichv1.Memory, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	updates := make(map[string]interface{})
	if req.IsSaved != nil {
		updates["is_saved"] = *req.IsSaved
	}
	if req.MemoryAt != nil {
		updates["memory_at"] = req.MemoryAt.AsTime()
	}
	if req.SeenAt != nil {
		updates["seen_at"] = req.SeenAt.AsTime()
	}

	memory, err := s.service.UpdateMemory(ctx, claims.UserID, req.Id, updates)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update memory: %v", err)
	}

	return &immichv1.Memory{
		Id:        memory.ID,
		OwnerId:   memory.UserID,
		Type:      immichv1.MemoryType_MEMORY_TYPE_ON_THIS_DAY,
		MemoryAt:  timestamppb.New(time.Now()),
		CreatedAt: timestamppb.New(time.Now()),
		UpdatedAt: timestamppb.New(memory.UpdatedAt),
		IsSaved:   req.IsSaved != nil && *req.IsSaved,
		Assets:    []*immichv1.Asset{},
	}, nil
}

// DeleteMemory deletes a memory
func (s *Server) DeleteMemory(ctx context.Context, req *immichv1.DeleteMemoryRequest) (*emptypb.Empty, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	err := s.service.DeleteMemory(ctx, claims.UserID, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete memory: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// AddMemoryAssets adds assets to a memory
func (s *Server) AddMemoryAssets(ctx context.Context, req *immichv1.AddMemoryAssetsRequest) (*immichv1.BulkIdResponseList, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	assetIDs := req.BulkIds.Ids
	err := s.service.AddAssetsToMemory(ctx, claims.UserID, req.Id, assetIDs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add assets: %v", err)
	}

	// Return success for all assets
	var responses []*immichv1.BulkIdResponse
	for _, id := range assetIDs {
		responses = append(responses, &immichv1.BulkIdResponse{
			Id:      id,
			Success: true,
		})
	}

	return &immichv1.BulkIdResponseList{
		Responses: responses,
	}, nil
}

// RemoveMemoryAssets removes assets from a memory
func (s *Server) RemoveMemoryAssets(ctx context.Context, req *immichv1.RemoveMemoryAssetsRequest) (*immichv1.BulkIdResponseList, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	assetIDs := req.BulkIds.Ids
	err := s.service.RemoveAssetsFromMemory(ctx, claims.UserID, req.Id, assetIDs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove assets: %v", err)
	}

	// Return success for all assets
	var responses []*immichv1.BulkIdResponse
	for _, id := range assetIDs {
		responses = append(responses, &immichv1.BulkIdResponse{
			Id:      id,
			Success: true,
		})
	}

	return &immichv1.BulkIdResponseList{
		Responses: responses,
	}, nil
}
