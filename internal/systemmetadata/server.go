package systemmetadata

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the SystemMetadataService
type Server struct {
	immichv1.UnimplementedSystemMetadataServiceServer
	service *Service
}

// NewServer creates a new system metadata server
func NewServer(service *Service) *Server {
	return &Server{
		service: service,
	}
}

// GetAdminOnboarding retrieves admin onboarding status
func (s *Server) GetAdminOnboarding(ctx context.Context, request *immichv1.GetAdminOnboardingRequest) (*immichv1.GetAdminOnboardingResponse, error) {
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(auth.MapAuthErrorToGRPC(err), "admin privileges required")
	}

	// Call service
	response, err := s.service.GetAdminOnboarding(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get admin onboarding status: %v", err)
	}

	return &immichv1.GetAdminOnboardingResponse{
		IsOnboarded: response.IsOnboarded,
	}, nil
}

// UpdateAdminOnboarding updates admin onboarding status
func (s *Server) UpdateAdminOnboarding(ctx context.Context, request *immichv1.UpdateAdminOnboardingRequest) (*immichv1.UpdateAdminOnboardingResponse, error) {
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(auth.MapAuthErrorToGRPC(err), "admin privileges required")
	}

	// Convert request
	req := UpdateAdminOnboardingRequest{
		IsOnboarded: request.GetIsOnboarded(),
	}

	// Call service
	response, err := s.service.UpdateAdminOnboarding(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update admin onboarding status: %v", err)
	}

	return &immichv1.UpdateAdminOnboardingResponse{
		IsOnboarded: response.IsOnboarded,
	}, nil
}

// GetReverseGeocodingState retrieves reverse geocoding state
func (s *Server) GetReverseGeocodingState(ctx context.Context, request *immichv1.GetReverseGeocodingStateRequest) (*immichv1.GetReverseGeocodingStateResponse, error) {
	// Require admin privileges for accessing system state
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(auth.MapAuthErrorToGRPC(err), "admin privileges required")
	}

	// Call service
	response, err := s.service.GetReverseGeocodingState(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get reverse geocoding state: %v", err)
	}

	return &immichv1.GetReverseGeocodingStateResponse{
		LastUpdate:         response.LastUpdate,
		LastImportFileName: response.LastImportFileName,
	}, nil
}

// GetVersionCheckState retrieves version check state.
func (s *Server) GetVersionCheckState(ctx context.Context, request *immichv1.GetVersionCheckStateRequest) (*immichv1.VersionCheckStateResponse, error) {
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(auth.MapAuthErrorToGRPC(err), "admin privileges required")
	}

	response, err := s.service.GetVersionCheckState(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get version check state: %v", err)
	}

	return &immichv1.VersionCheckStateResponse{
		CheckedAt:      response.CheckedAt,
		ReleaseVersion: response.ReleaseVersion,
	}, nil
}
