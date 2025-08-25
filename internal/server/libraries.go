package server

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/libraries"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Ensure Server implements LibrariesServiceServer
var _ immichv1.LibrariesServiceServer = (*Server)(nil)

// CreateLibrary creates a new library
func (s *Server) CreateLibrary(ctx context.Context, req *immichv1.CreateLibraryRequest) (*immichv1.CreateLibraryResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Create library
	library, err := s.libraryService.CreateLibrary(ctx, userID, &libraries.CreateLibraryRequest{
		Name:            req.Name,
		ImportPaths:     req.ImportPaths,
		ExclusionPatterns: req.ExclusionPatterns,
		IsVisible:       req.IsVisible,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create library: %v", err)
	}

	return &immichv1.CreateLibraryResponse{
		Id:                library.ID.String(),
		Name:              library.Name,
		OwnerId:           library.OwnerID.String(),
		ImportPaths:       library.ImportPaths,
		ExclusionPatterns: library.ExclusionPatterns,
		IsVisible:         library.IsVisible,
		CreatedAt:         library.CreatedAt.Unix(),
		UpdatedAt:         library.UpdatedAt.Unix(),
	}, nil
}

// GetLibraries returns all libraries for the user
func (s *Server) GetLibraries(ctx context.Context, _ *emptypb.Empty) (*immichv1.GetLibrariesResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Get libraries
	libs, err := s.libraryService.GetLibraries(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get libraries: %v", err)
	}

	// Convert to proto format
	var libraries []*immichv1.LibraryResponse
	for _, lib := range libs {
		libraries = append(libraries, &immichv1.LibraryResponse{
			Id:                lib.ID.String(),
			Name:              lib.Name,
			OwnerId:           lib.OwnerID.String(),
			ImportPaths:       lib.ImportPaths,
			ExclusionPatterns: lib.ExclusionPatterns,
			IsVisible:         lib.IsVisible,
			CreatedAt:         lib.CreatedAt.Unix(),
			UpdatedAt:         lib.UpdatedAt.Unix(),
			AssetCount:        int32(lib.AssetCount),
		})
	}

	return &immichv1.GetLibrariesResponse{
		Libraries: libraries,
	}, nil
}

// GetLibrary returns a specific library
func (s *Server) GetLibrary(ctx context.Context, req *immichv1.GetLibraryRequest) (*immichv1.GetLibraryResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Parse library ID
	libraryID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid library ID")
	}

	// Get library
	lib, err := s.libraryService.GetLibrary(ctx, userID, libraryID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get library: %v", err)
	}

	return &immichv1.GetLibraryResponse{
		Id:                lib.ID.String(),
		Name:              lib.Name,
		OwnerId:           lib.OwnerID.String(),
		ImportPaths:       lib.ImportPaths,
		ExclusionPatterns: lib.ExclusionPatterns,
		IsVisible:         lib.IsVisible,
		CreatedAt:         lib.CreatedAt.Unix(),
		UpdatedAt:         lib.UpdatedAt.Unix(),
		AssetCount:        int32(lib.AssetCount),
	}, nil
}

// UpdateLibrary updates a library
func (s *Server) UpdateLibrary(ctx context.Context, req *immichv1.UpdateLibraryRequest) (*immichv1.UpdateLibraryResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Parse library ID
	libraryID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid library ID")
	}

	// Update library
	lib, err := s.libraryService.UpdateLibrary(ctx, userID, libraryID, &libraries.UpdateLibraryRequest{
		Name:              req.Name,
		ImportPaths:       req.ImportPaths,
		ExclusionPatterns: req.ExclusionPatterns,
		IsVisible:         &req.IsVisible,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update library: %v", err)
	}

	return &immichv1.UpdateLibraryResponse{
		Id:                lib.ID.String(),
		Name:              lib.Name,
		OwnerId:           lib.OwnerID.String(),
		ImportPaths:       lib.ImportPaths,
		ExclusionPatterns: lib.ExclusionPatterns,
		IsVisible:         lib.IsVisible,
		CreatedAt:         lib.CreatedAt.Unix(),
		UpdatedAt:         lib.UpdatedAt.Unix(),
	}, nil
}

// DeleteLibrary deletes a library
func (s *Server) DeleteLibrary(ctx context.Context, req *immichv1.DeleteLibraryRequest) (*emptypb.Empty, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Parse library ID
	libraryID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid library ID")
	}

	// Delete library
	if err := s.libraryService.DeleteLibrary(ctx, userID, libraryID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete library: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ScanLibrary starts a library scan
func (s *Server) ScanLibrary(ctx context.Context, req *immichv1.ScanLibraryRequest) (*immichv1.ScanLibraryResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Parse library ID
	libraryID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid library ID")
	}

	// Start scan
	jobID, err := s.libraryService.ScanLibrary(ctx, userID, libraryID, req.RefreshModifiedFiles, req.RefreshAllFiles)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to start library scan: %v", err)
	}

	return &immichv1.ScanLibraryResponse{
		JobId: jobID.String(),
	}, nil
}

// GetLibraryStatistics returns library statistics
func (s *Server) GetLibraryStatistics(ctx context.Context, req *immichv1.GetLibraryStatisticsRequest) (*immichv1.GetLibraryStatisticsResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Parse library ID
	libraryID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid library ID")
	}

	// Get statistics
	stats, err := s.libraryService.GetLibraryStatistics(ctx, userID, libraryID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get library statistics: %v", err)
	}

	return &immichv1.GetLibraryStatisticsResponse{
		Photos:     int32(stats.Photos),
		Videos:     int32(stats.Videos),
		TotalSize:  stats.TotalSize,
		Usage:      float32(stats.Usage),
	}, nil
}

// ValidateImportPath validates an import path
func (s *Server) ValidateImportPath(ctx context.Context, req *immichv1.ValidateImportPathRequest) (*immichv1.ValidateImportPathResponse, error) {
	// Validate the import path
	valid, reason := s.libraryService.ValidateImportPath(req.ImportPath)
	
	return &immichv1.ValidateImportPathResponse{
		IsValid: valid,
		Message: reason,
	}, nil
}