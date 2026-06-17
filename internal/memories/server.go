package memories

import (
	"context"
	"fmt"
	"time"

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

// assetsForMemoryIDs loads the full asset rows linked to the given memory
// IDs in a single batch query, then groups them by memory ID. Used to
// populate the Memory.assets proto field, which previously was always empty.
func (s *Server) assetsForMemoryIDs(ctx context.Context, memoryIDs []string) (map[string][]*immichv1.Asset, error) {
	if len(memoryIDs) == 0 {
		return map[string][]*immichv1.Asset{}, nil
	}

	// Resolve memory -> asset IDs via the memories_assets_assets join table,
	// then batch-load the full assets in a single query.
	links := make(map[string][]pgtype.UUID, len(memoryIDs))
	allAssetIDs := make([]pgtype.UUID, 0)
	for _, mid := range memoryIDs {
		memUUID, err := uuid.Parse(mid)
		if err != nil {
			continue
		}
		assetUUIDs, err := s.service.queries.GetMemoryAssets(ctx, pgtype.UUID{Bytes: memUUID, Valid: true})
		if err != nil {
			return nil, fmt.Errorf("failed to load asset links for memory %s: %w", mid, err)
		}
		links[mid] = assetUUIDs
		allAssetIDs = append(allAssetIDs, assetUUIDs...)
	}

	if len(allAssetIDs) == 0 {
		return map[string][]*immichv1.Asset{}, nil
	}

	assets, err := s.service.queries.GetAssetsByIDs(ctx, allAssetIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to batch-load assets: %w", err)
	}

	// Build id -> asset lookup once, then expand per memory.
	byID := make(map[string]sqlc.Asset, len(assets))
	for _, a := range assets {
		byID[uuid.UUID(a.ID.Bytes).String()] = a
	}

	result := make(map[string][]*immichv1.Asset, len(memoryIDs))
	for mid, ids := range links {
		protoAssets := make([]*immichv1.Asset, 0, len(ids))
		for _, id := range ids {
			if asset, ok := byID[uuid.UUID(id.Bytes).String()]; ok {
				protoAssets = append(protoAssets, s.convertAssetToProto(asset))
			}
		}
		result[mid] = protoAssets
	}
	return result, nil
}

// convertAssetToProto mirrors internal/server/asset.go's converter but
// lives in this package to avoid an import cycle. Memories are
// intentionally a lighter conversion than the full asset viewer.
func (s *Server) convertAssetToProto(asset sqlc.Asset) *immichv1.Asset {
	assetType := immichv1.AssetType_ASSET_TYPE_IMAGE
	if asset.Type == "VIDEO" {
		assetType = immichv1.AssetType_ASSET_TYPE_VIDEO
	}

	proto := &immichv1.Asset{
		Id:               uuid.UUID(asset.ID.Bytes).String(),
		DeviceAssetId:    asset.DeviceAssetId,
		OwnerId:          uuid.UUID(asset.OwnerId.Bytes).String(),
		DeviceId:         asset.DeviceId,
		Type:             assetType,
		OriginalPath:     asset.OriginalPath,
		OriginalFileName: asset.OriginalFileName,
		CreatedAt:        timestamppb.New(asset.CreatedAt.Time),
		UpdatedAt:        timestamppb.New(asset.UpdatedAt.Time),
		IsFavorite:       asset.IsFavorite,
		IsArchived:       asset.Visibility == sqlc.AssetVisibilityEnumArchive,
		IsTrashed:        asset.Status == sqlc.AssetsStatusEnumTrashed,
		Checksum:         fmt.Sprintf("%x", asset.Checksum),
	}

	if asset.Duration.Valid {
		proto.Duration = &asset.Duration.String
	}

	if asset.LivePhotoVideoId.Valid {
		livePhotoID := asset.LivePhotoVideoId.String()
		proto.LivePhotoVideoId = &livePhotoID
	}

	if asset.StackId.Valid {
		stackParentID := asset.StackId.String()
		proto.StackParentId = &stackParentID
	}

	return proto
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

	// Pre-load assets for all memories in a single batch so the
	// Memory.assets proto field is populated. Without this the web
	// client's MemoryLane UI renders empty cards.
	memoryIDs := make([]string, 0, len(memories))
	for _, m := range memories {
		memoryIDs = append(memoryIDs, m.ID)
	}
	assetsByMem, err := s.assetsForMemoryIDs(ctx, memoryIDs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load memory assets: %v", err)
	}

	// Convert to proto memories
	protoMemories := make([]*immichv1.Memory, 0, len(memories))
	for _, mem := range memories {
		protoMem := &immichv1.Memory{
			Id:        mem.ID,
			OwnerId:   mem.UserID,
			Type:      immichv1.MemoryType_MEMORY_TYPE_ON_THIS_DAY,
			MemoryAt:  timestamppb.New(mem.Date),
			CreatedAt: timestamppb.New(mem.CreatedAt),
			UpdatedAt: timestamppb.New(mem.UpdatedAt),
			IsSaved:   false,
			Assets:    assetsByMem[mem.ID],
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

	assets, err := s.assetsForMemoryIDs(ctx, []string{created.ID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load memory assets: %v", err)
	}

	return &immichv1.Memory{
		Id:        created.ID,
		OwnerId:   created.UserID,
		Type:      req.Type,
		MemoryAt:  req.MemoryAt,
		CreatedAt: timestamppb.New(created.CreatedAt),
		UpdatedAt: timestamppb.New(created.UpdatedAt),
		IsSaved:   req.IsSaved != nil && *req.IsSaved,
		Assets:    assets[created.ID],
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

	assets, err := s.assetsForMemoryIDs(ctx, []string{memory.ID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load memory assets: %v", err)
	}

	return &immichv1.Memory{
		Id:        memory.ID,
		OwnerId:   memory.UserID,
		Type:      immichv1.MemoryType_MEMORY_TYPE_ON_THIS_DAY,
		MemoryAt:  timestamppb.New(memory.Date),
		CreatedAt: timestamppb.New(memory.CreatedAt),
		UpdatedAt: timestamppb.New(memory.UpdatedAt),
		IsSaved:   false,
		Assets:    assets[memory.ID],
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

	assets, err := s.assetsForMemoryIDs(ctx, []string{memory.ID})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load memory assets: %v", err)
	}

	return &immichv1.Memory{
		Id:        memory.ID,
		OwnerId:   memory.UserID,
		Type:      immichv1.MemoryType_MEMORY_TYPE_ON_THIS_DAY,
		MemoryAt:  timestamppb.New(time.Now()),
		CreatedAt: timestamppb.New(time.Now()),
		UpdatedAt: timestamppb.New(memory.UpdatedAt),
		IsSaved:   req.IsSaved != nil && *req.IsSaved,
		Assets:    assets[memory.ID],
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
