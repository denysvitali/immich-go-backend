package server

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) GetAboutInfo(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerAboutResponse, error) {
	return &immichv1.ServerAboutResponse{
		Build:                      "",
		BuildImage:                 "",
		BuildImageUrl:              "",
		BuildUrl:                   "",
		Exiftool:                   "",
		Ffmpeg:                     "",
		Imagemagick:                "",
		Libvips:                    "",
		Licensed:                   false,
		Nodejs:                     "",
		Repository:                 "denysvitali/immich-go-backend",
		RepositoryUrl:              "https://github.com/denysvitali/immich-go-backend",
		SourceCommit:               SourceCommit,
		SourceRef:                  SourceRef,
		SourceUrl:                  SourceUrl,
		ThirdPartyBugFeatureUrl:    "",
		ThirdPartyDocumentationUrl: "",
		ThirdPartySourceUrl:        "",
		ThirdPartySupportUrl:       "",
		Version:                    Version,
		VersionUrl:                 "https://github.com/denysvitali/immich-go-backend/releases/tag/" + Version,
	}, nil
}

func (s *Server) GetServerConfig(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerConfigResponse, error) {
	// The frontend only offers the "create admin account" registration
	// screen when isInitialized is false; a hardcoded true here means a
	// freshly-provisioned (zero-user) instance has no way to bootstrap an
	// admin account from the UI.
	isInitialized, err := s.authService.IsInitialized(ctx)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to check initialization state", err)
	}

	return &immichv1.ServerConfigResponse{
		LoginPageMessage: "Welcome to Immich",
		TrashDays:        30,
		UserDeleteDelay:  7,
		OauthButtonText:  "Login with OAuth",
		IsInitialized:    isInitialized,
		// IsOnboarded here tracks the post-signup admin setup wizard, a
		// separate concept from IsInitialized — see
		// SystemMetadataService.GetAdminOnboarding for the real flag.
		IsOnboarded:      true,
		ExternalDomain:   "",
		PublicUsers:      true,
		MapDarkStyleUrl:  "https://tiles.immich.cloud/v1/style/dark.json",
		MapLightStyleUrl: "https://tiles.immich.cloud/v1/style/light.json",
		MaintenanceMode:  false,
	}, nil
}

func (s *Server) GetServerFeatures(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerFeaturesResponse, error) {
	return &immichv1.ServerFeaturesResponse{
		SmartSearch:        true,
		FacialRecognition:  true,
		DuplicateDetection: true,
		Map:                true,
		ReverseGeocoding:   true,
		ImportFaces:        false,
		Sidecar:            true,
		Search:             true,
		Trash:              true,
		Oauth:              false,
		OauthAutoLaunch:    false,
		PasswordLogin:      true,
		ConfigFile:         false,
		Email:              false,
	}, nil
}

func (s *Server) GetServerLicense(ctx context.Context, empty *emptypb.Empty) (*immichv1.LicenseResponse, error) {
	// Return an open-source license response
	return &immichv1.LicenseResponse{
		ActivatedAt:   timestamppb.Now(),
		LicenseKey:    "OPEN-SOURCE",
		ActivationKey: "AGPL-3.0",
	}, nil
}

func (s *Server) SetServerLicense(ctx context.Context, request *immichv1.LicenseKeyRequest) (*immichv1.LicenseResponse, error) {
	// For open-source implementation, accept any license key but always return open-source
	return &immichv1.LicenseResponse{
		ActivatedAt:   timestamppb.Now(),
		LicenseKey:    request.LicenseKey,
		ActivationKey: "AGPL-3.0",
	}, nil
}

func (s *Server) DeleteServerLicense(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	// License deletion is a no-op for open-source implementation
	return &emptypb.Empty{}, nil
}

func (s *Server) GetSupportedMediaTypes(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerMediaTypesResponse, error) {
	return &immichv1.ServerMediaTypesResponse{
		Image: []string{
			"image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp",
			"image/tiff", "image/svg+xml", "image/heic", "image/heif",
			"image/x-adobe-dng", "image/x-canon-cr2", "image/x-canon-crw",
			"image/x-nikon-nef", "image/x-sony-arw",
		},
		Video: []string{
			"video/mp4", "video/webm", "video/quicktime", "video/x-msvideo",
			"video/x-matroska", "video/mpeg", "video/3gpp", "video/MP2T",
			"video/avi", "video/x-flv", "video/x-ms-wmv",
		},
		Sidecar: []string{
			"application/xml", "text/xml", "application/json",
		},
	}, nil
}

func (s *Server) PingServer(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerPingResponse, error) {
	return &immichv1.ServerPingResponse{
		Res: "pong",
	}, nil
}

func (s *Server) GetServerStatistics(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerStatsResponse, error) {
	stats, err := s.queries.GetServerAssetStatistics(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get server statistics: %v", err)
	}

	userRows, err := s.queries.GetServerUsageByUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get per-user usage: %v", err)
	}

	usageByUser := make([]*immichv1.UsageByUser, 0, len(userRows))
	for _, row := range userRows {
		entry := &immichv1.UsageByUser{
			Photos:      int32(row.Photos),
			Videos:      int32(row.Videos),
			Usage:       row.Usage,
			UsagePhotos: row.UsagePhotos,
			UsageVideos: row.UsageVideos,
			UserId:      uuid.UUID(row.UserID.Bytes).String(),
			UserName:    row.UserName,
		}
		if row.QuotaSizeInBytes.Valid {
			entry.QuotaSizeInBytes = &row.QuotaSizeInBytes.Int64
		}
		usageByUser = append(usageByUser, entry)
	}

	return &immichv1.ServerStatsResponse{
		Photos:      int32(stats.Photos),
		Videos:      int32(stats.Videos),
		Usage:       stats.Usage,
		UsagePhotos: stats.UsagePhotos,
		UsageVideos: stats.UsageVideos,
		UsageByUser: usageByUser,
	}, nil
}

func (s *Server) GetStorage(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerStorageResponse, error) {
	path := s.config.Storage.Local.RootPath
	if path == "" {
		path = "."
	}

	var fs unix.Statfs_t
	if err := unix.Statfs(path, &fs); err != nil {
		// Fall back to the working directory (e.g. RootPath not created yet).
		if err := unix.Statfs(".", &fs); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to stat filesystem: %v", err)
		}
	}

	blockSize := uint64(fs.Bsize) //nolint:gosec // Bsize is never negative
	total := fs.Blocks * blockSize
	available := fs.Bavail * blockSize
	used := total - fs.Bfree*blockSize

	var usagePercentage float64
	if total > 0 {
		usagePercentage = float64(used) / float64(total) * 100
	}

	return &immichv1.ServerStorageResponse{
		DiskAvailable:       humanReadableBytes(available),
		DiskAvailableRaw:    int64(available), //nolint:gosec // disk sizes fit in int64
		DiskSize:            humanReadableBytes(total),
		DiskSizeRaw:         int64(total), //nolint:gosec // disk sizes fit in int64
		DiskUsagePercentage: usagePercentage,
		DiskUse:             humanReadableBytes(used),
		DiskUseRaw:          int64(used), //nolint:gosec // disk sizes fit in int64
	}, nil
}

func humanReadableBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (s *Server) GetTheme(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerThemeResponse, error) {
	return &immichv1.ServerThemeResponse{
		CustomCss: "",
	}, nil
}

func (s *Server) GetServerVersion(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerVersionResponse, error) {
	return &immichv1.ServerVersionResponse{
		Major: 1,
		Minor: 95,
		Patch: 0,
	}, nil
}

func (s *Server) GetVersionHistory(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerVersionHistoryResponse, error) {
	rows, err := s.queries.ListVersionHistory(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list version history: %v", err)
	}

	items := make([]*immichv1.ServerVersionHistoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, &immichv1.ServerVersionHistoryItem{
			CreatedAt: timestamppb.New(row.CreatedAt.Time),
			Id:        uuid.UUID(row.ID.Bytes).String(),
			Version:   row.Version,
		})
	}

	return &immichv1.ServerVersionHistoryResponse{Items: items}, nil
}
