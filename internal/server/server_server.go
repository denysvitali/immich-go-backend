package server

import (
	"context"
	"fmt"
	"time"

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
	return &immichv1.ServerConfigResponse{
		LoginPageMessage: "Welcome to Immich",
		TrashDays:        30,
		UserDeleteDelay:  7,
		OauthButtonText:  "Login with OAuth",
		IsInitialized:    true,
		IsOnboarded:      true,
		ExternalDomain:   "",
		PublicUsers:      true,
		MapDarkStyleUrl:  "https://tiles.immich.cloud/v1/style/dark.json",
		MapLightStyleUrl: "https://tiles.immich.cloud/v1/style/light.json",
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
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) SetServerLicense(ctx context.Context, request *immichv1.LicenseKeyRequest) (*immichv1.LicenseResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) DeleteServerLicense(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetSupportedMediaTypes(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerMediaTypesResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) PingServer(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerPingResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetServerStatistics(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerStatsResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetStorage(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerStorageResponse, error) {
	return &immichv1.ServerStorageResponse{
		DiskAvailable:       "500GB",
		DiskAvailableRaw:    500000000000,
		DiskSize:            "1TB",
		DiskSizeRaw:         1000000000000,
		DiskUsagePercentage: 50,
		DiskUse:             "500GB",
		DiskUseRaw:          500000000000,
	}, nil
}

func (s *Server) GetTheme(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerThemeResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetServerVersion(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerVersionResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetVersionHistory(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerVersionHistoryResponse, error) {
	return &immichv1.ServerVersionHistoryResponse{
		Items: []*immichv1.ServerVersionHistoryItem{
			{
				CreatedAt: timestamppb.New(time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC)),
				Id:        "foo-1",
				Version:   "v1.0.0",
			},
			{
				CreatedAt: timestamppb.New(time.Date(2025, 1, 2, 1, 0, 0, 0, time.UTC)),
				Id:        "foo-2",
				Version:   "v1.1.0",
			},
		},
	}, nil
}
