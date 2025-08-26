package view

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the ViewService
type Server struct {
	immichv1.UnimplementedViewServiceServer
	service *Service
}

// NewServer creates a new view server
func NewServer(service *Service) *Server {
	return &Server{
		service: service,
	}
}

// GetAssetsByOriginalPath retrieves assets by their original file path
func (s *Server) GetAssetsByOriginalPath(ctx context.Context, request *immichv1.GetAssetsByOriginalPathRequest) (*immichv1.GetAssetsByOriginalPathResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Convert request
	req := GetAssetsByOriginalPathRequest{
		Path: request.GetPath(),
	}

	if request.IsArchived != nil {
		archived := request.GetIsArchived()
		req.IsArchived = &archived
	}

	if request.IsFavorite != nil {
		favorite := request.GetIsFavorite()
		req.IsFavorite = &favorite
	}

	if request.Skip != nil {
		skip := request.GetSkip()
		req.Skip = &skip
	}

	if request.Take != nil {
		take := request.GetTake()
		req.Take = &take
	}

	// Call service
	response, err := s.service.GetAssetsByOriginalPath(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get assets by path: %v", err)
	}

	// Convert response
	assets := make([]*immichv1.AssetInfo, len(response.Assets))
	for i, asset := range response.Assets {
		assets[i] = &immichv1.AssetInfo{
			Id:               asset.ID,
			DeviceAssetId:    asset.DeviceAssetID,
			DeviceId:         asset.DeviceID,
			Type:             immichv1.AssetType(asset.Type),
			OriginalPath:     asset.OriginalPath,
			OriginalFileName: asset.OriginalFileName,
			IsArchived:       asset.IsArchived,
			IsFavorite:       asset.IsFavorite,
			IsTrashed:        asset.IsTrashed,
		}
	}

	return &immichv1.GetAssetsByOriginalPathResponse{
		Assets: assets,
		Total:  response.Total,
	}, nil
}

// GetUniqueOriginalPaths retrieves all unique original file paths
func (s *Server) GetUniqueOriginalPaths(ctx context.Context, request *immichv1.GetUniqueOriginalPathsRequest) (*immichv1.GetUniqueOriginalPathsResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok || claims == nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Call service
	response, err := s.service.GetUniqueOriginalPaths(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get unique paths: %v", err)
	}

	return &immichv1.GetUniqueOriginalPathsResponse{
		Paths: response.Paths,
	}, nil
}