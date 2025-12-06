package server

import (
	"context"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SetMaintenanceMode sets the maintenance mode
func (s *Server) SetMaintenanceMode(ctx context.Context, req *immichv1.SetMaintenanceModeRequest) (*immichv1.SetMaintenanceModeResponse, error) {
	// Verify user is authenticated and is admin
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication required")
	}

	if !claims.IsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "admin access required")
	}

	switch req.Action {
	case immichv1.MaintenanceAction_MAINTENANCE_ACTION_START:
		token, err := s.maintenanceService.StartMaintenance(ctx, claims.Email)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to start maintenance mode: %v", err)
		}
		return &immichv1.SetMaintenanceModeResponse{
			IsMaintenanceMode: true,
			Token:             &token,
		}, nil

	case immichv1.MaintenanceAction_MAINTENANCE_ACTION_STOP:
		err := s.maintenanceService.StopMaintenance(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to stop maintenance mode: %v", err)
		}
		return &immichv1.SetMaintenanceModeResponse{
			IsMaintenanceMode: false,
		}, nil

	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid maintenance action")
	}
}

// MaintenanceLogin logs into maintenance mode
func (s *Server) MaintenanceLogin(ctx context.Context, req *immichv1.MaintenanceLoginRequest) (*immichv1.MaintenanceAuthResponse, error) {
	// Check if we're in maintenance mode
	state, err := s.maintenanceService.GetMaintenanceMode(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get maintenance mode state: %v", err)
	}

	if !state.IsMaintenanceMode {
		return nil, status.Errorf(codes.FailedPrecondition, "not in maintenance mode")
	}

	// Validate the token if provided
	token := ""
	if req.Token != nil {
		token = *req.Token
	}

	if token == "" {
		return nil, status.Errorf(codes.InvalidArgument, "maintenance token required")
	}

	claims, err := s.maintenanceService.ValidateMaintenanceToken(ctx, token)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid maintenance token: %v", err)
	}

	return &immichv1.MaintenanceAuthResponse{
		Username:        claims.Username,
		IsAuthenticated: true,
	}, nil
}

// GetMaintenanceStatus returns the current maintenance mode status
func (s *Server) GetMaintenanceStatus(ctx context.Context, _ *emptypb.Empty) (*immichv1.MaintenanceStatusResponse, error) {
	state, err := s.maintenanceService.GetMaintenanceMode(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get maintenance mode state: %v", err)
	}

	response := &immichv1.MaintenanceStatusResponse{
		IsMaintenanceMode: state.IsMaintenanceMode,
	}

	if state.IsMaintenanceMode {
		response.StartedBy = &state.StartedBy
		startedAt := state.StartedAt.Format("2006-01-02T15:04:05Z07:00")
		response.StartedAt = &startedAt
	}

	return response, nil
}

// GetApkLinks returns APK download links for mobile apps
func (s *Server) GetApkLinks(ctx context.Context, _ *emptypb.Empty) (*immichv1.ServerApkLinksResponse, error) {
	// Return links to official Immich APK releases
	// In production, this would fetch from a configuration or external source
	links := []*immichv1.ApkLink{
		{
			Name:    "Immich Android App",
			Url:     "https://github.com/immich-app/immich/releases/latest/download/app-release.apk",
			Version: Version,
		},
		{
			Name:    "Immich Android App (F-Droid)",
			Url:     "https://f-droid.org/packages/app.alextran.immich/",
			Version: Version,
		},
	}

	return &immichv1.ServerApkLinksResponse{
		Links: links,
	}, nil
}

// CheckVersion checks for available version updates
func (s *Server) CheckVersion(ctx context.Context, _ *emptypb.Empty) (*immichv1.ServerVersionCheckResponse, error) {
	// In production, this would check against an external version API
	// For now, return current version info without checking for updates
	currentVersion := Version
	if currentVersion == "" {
		currentVersion = "1.0.0"
	}

	return &immichv1.ServerVersionCheckResponse{
		CurrentVersion:    currentVersion,
		LatestVersion:     currentVersion, // Would be fetched from GitHub releases
		IsUpdateAvailable: false,
		ReleaseNotesUrl:   "https://github.com/immich-app/immich/releases",
		CheckedAt:         timestamppb.Now(),
	}, nil
}
