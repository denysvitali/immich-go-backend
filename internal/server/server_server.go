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
		ExternalDomain:   "foo.example.com",
		IsInitialized:    true,
		IsOnboarded:      true,
		LoginPageMessage: "Welcome to Immich",
		MapDarkStyleUrl:  "",
		MapLightStyleUrl: "",
		OauthButtonText:  "",
		PublicUsers:      false,
		TrashDays:        0,
		UserDeleteDelay:  0,
	}, nil
}

func (s *Server) GetServerFeatures(ctx context.Context, empty *emptypb.Empty) (*immichv1.ServerFeaturesResponse, error) {
	return &immichv1.ServerFeaturesResponse{
		ConfigFile:         false,
		DuplicateDetection: false,
		Email:              false,
		FacialRecognition:  false,
		ImportFaces:        false,
		Map:                false,
		Oauth:              false,
		OauthAutoLaunch:    false,
		PasswordLogin:      true,
		ReverseGeocoding:   false,
		Search:             false,
		Sidecar:            false,
		SmartSearch:        false,
		Trash:              false,
	}, nil
}
