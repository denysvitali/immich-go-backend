package duplicates

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/grpcutil"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server implements the DuplicatesService
type Server struct {
	immichv1.UnimplementedDuplicatesServiceServer
	service *Service
}

// NewServer creates a new duplicates server
func NewServer(service *Service) *Server {
	return &Server{
		service: service,
	}
}

// GetAssetDuplicates retrieves duplicate assets for the authenticated user
func (s *Server) GetAssetDuplicates(ctx context.Context, request *immichv1.GetAssetDuplicatesRequest) (*immichv1.GetAssetDuplicatesResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Call service
	response, err := s.service.GetAssetDuplicates(ctx, claims.UserID)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to get asset duplicates", err)
	}

	// Convert response
	duplicateGroups := make([]*immichv1.DuplicateGroup, len(response.Duplicates))
	for i, group := range response.Duplicates {
		assets := make([]*immichv1.DuplicateAsset, len(group.Assets))
		for j, asset := range group.Assets {
			assets[j] = &immichv1.DuplicateAsset{
				AssetId:        asset.AssetID,
				DeviceAssetId:  asset.DeviceAssetID,
				DeviceId:       asset.DeviceID,
				Checksum:       asset.Checksum,
				Type:           immichv1.AssetType(asset.Type),
				OriginalPath:   asset.OriginalPath,
				FileSizeInByte: asset.FileSizeInByte,
			}
		}

		duplicateGroups[i] = &immichv1.DuplicateGroup{
			DuplicateId: group.DuplicateID,
			Assets:      assets,
		}
	}

	return &immichv1.GetAssetDuplicatesResponse{
		Duplicates: duplicateGroups,
	}, nil
}

// DeleteDuplicates clears the requested duplicate groups for the authenticated user.
func (s *Server) DeleteDuplicates(ctx context.Context, request *immichv1.DeleteDuplicatesRequest) (*emptypb.Empty, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	if err := s.service.DeleteDuplicates(ctx, claims.UserID, request.GetIds()); err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to delete duplicate groups", err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteDuplicate clears a single duplicate group for the authenticated user.
func (s *Server) DeleteDuplicate(ctx context.Context, request *immichv1.DeleteDuplicateRequest) (*emptypb.Empty, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	if request.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "duplicate ID is required")
	}

	if err := s.service.DeleteDuplicate(ctx, claims.UserID, request.GetId()); err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to delete duplicate group", err)
	}

	return &emptypb.Empty{}, nil
}

// ResolveDuplicates trashes selected assets and clears each resolved duplicate group.
func (s *Server) ResolveDuplicates(ctx context.Context, request *immichv1.ResolveDuplicatesRequest) (*immichv1.ResolveDuplicatesResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	groups := make([]*ResolveDuplicateGroup, len(request.GetGroups()))
	for i, group := range request.GetGroups() {
		groups[i] = &ResolveDuplicateGroup{
			DuplicateID:   group.GetDuplicateId(),
			KeepAssetIDs:  group.GetKeepAssetIds(),
			TrashAssetIDs: group.GetTrashAssetIds(),
		}
	}

	results, err := s.service.ResolveDuplicates(ctx, claims.UserID, groups)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to resolve duplicate groups", err)
	}

	response := &immichv1.ResolveDuplicatesResponse{
		Results: make([]*immichv1.DuplicateBulkIdResponse, len(results)),
	}
	for i, result := range results {
		response.Results[i] = &immichv1.DuplicateBulkIdResponse{
			Id:      result.ID,
			Success: result.Success,
		}
		if result.Error != "" {
			response.Results[i].Error = &result.Error
		}
		if result.ErrorMessage != "" {
			response.Results[i].ErrorMessage = &result.ErrorMessage
		}
	}

	return response, nil
}
