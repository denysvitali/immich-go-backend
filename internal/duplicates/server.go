package duplicates

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		return nil, status.Errorf(codes.Internal, "failed to get asset duplicates: %v", err)
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
