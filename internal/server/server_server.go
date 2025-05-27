package server

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) GetAboutInfo(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerAboutResponse, error) {
	//TODO implement me
	panic("implement me")
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
