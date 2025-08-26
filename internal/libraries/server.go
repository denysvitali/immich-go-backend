package libraries

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/auth"
)

// Server implements the LibrariesServiceServer interface
type Server struct {
	immichv1.UnimplementedLibrariesServiceServer
	service *Service
}

// NewServer creates a new Libraries server
func NewServer(service *Service) *Server {
	return &Server{
		service: service,
	}
}

// CreateLibrary creates a new library
func (s *Server) CreateLibrary(ctx context.Context, req *immichv1.CreateLibraryRequest) (*immichv1.LibraryResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	createReq := CreateLibraryRequest{
		Name:              req.Name,
		Type:              LibraryTypeExternal,
		ImportPaths:       req.ImportPaths,
		ExclusionPatterns: req.ExclusionPatterns,
		IsWatched:         false,
		IsVisible:         true,
	}
	library, err := s.service.CreateLibrary(ctx, userID, createReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return s.libraryToProto(library), nil
}

// GetAllLibraries gets all libraries for the authenticated user
func (s *Server) GetAllLibraries(ctx context.Context, req *immichv1.GetAllLibrariesRequest) (*immichv1.GetAllLibrariesResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	libraries, err := s.service.GetLibraries(ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	protoLibraries := make([]*immichv1.LibraryResponse, len(libraries))
	for i, lib := range libraries {
		protoLibraries[i] = s.libraryToProto(lib)
	}

	return &immichv1.GetAllLibrariesResponse{
		Libraries: protoLibraries,
	}, nil
}

// GetLibrary gets a specific library by ID
func (s *Server) GetLibrary(ctx context.Context, req *immichv1.GetLibraryRequest) (*immichv1.LibraryResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	libraryID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid library ID")
	}

	library, err := s.service.GetLibrary(ctx, userID, libraryID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return s.libraryToProto(library), nil
}

// UpdateLibrary updates an existing library
func (s *Server) UpdateLibrary(ctx context.Context, req *immichv1.UpdateLibraryRequest) (*immichv1.LibraryResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	libraryID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid library ID")
	}

	updateReq := &UpdateLibraryRequest{
		Name:              req.Name,
		ImportPaths:       req.ImportPaths,
		ExclusionPatterns: req.ExclusionPatterns,
	}
	library, err := s.service.UpdateLibrary(ctx, userID, libraryID, updateReq)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return s.libraryToProto(library), nil
}

// DeleteLibrary deletes a library
func (s *Server) DeleteLibrary(ctx context.Context, req *immichv1.DeleteLibraryRequest) (*emptypb.Empty, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	libraryID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid library ID")
	}

	if err := s.service.DeleteLibrary(ctx, userID, libraryID); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

// GetLibraryStatistics gets statistics for a library
func (s *Server) GetLibraryStatistics(ctx context.Context, req *immichv1.GetLibraryStatisticsRequest) (*immichv1.LibraryStatisticsResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	libraryID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid library ID")
	}

	stats, err := s.service.GetLibraryStatistics(ctx, userID, libraryID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &immichv1.LibraryStatisticsResponse{
		Photos: int32(stats.Photos),
		Videos: int32(stats.Videos),
		Total:  stats.AssetCount,
		Usage:  stats.TotalSize,
	}, nil
}

// ScanLibrary triggers a scan of a library
func (s *Server) ScanLibrary(ctx context.Context, req *immichv1.ScanLibraryRequest) (*emptypb.Empty, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	libraryID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid library ID")
	}

	if _, err := s.service.ScanLibrary(ctx, userID, libraryID, false, false); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

// ValidateLibrary validates a library
func (s *Server) ValidateLibrary(ctx context.Context, req *immichv1.ValidateLibraryRequest) (*immichv1.ValidateLibraryResponse, error) {
	userID, err := auth.GetUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	libraryID, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid library ID")
	}

	// For now, this is a stub implementation
	_ = userID
	_ = libraryID

	return &immichv1.ValidateLibraryResponse{
		ImportPaths: []string{},
	}, nil
}


// Helper function to convert database library to proto
func (s *Server) libraryToProto(lib *Library) *immichv1.LibraryResponse {
	return &immichv1.LibraryResponse{
		Id:                lib.ID.String(),
		OwnerId:           lib.OwnerID.String(),
		Name:              lib.Name,
		ImportPaths:       lib.ImportPaths,
		ExclusionPatterns: lib.ExclusionPatterns,
		CreatedAt:         timestamppb.New(lib.CreatedAt),
		UpdatedAt:         timestamppb.New(lib.UpdatedAt),
		AssetCount:        int32(lib.AssetCount),
	}
}