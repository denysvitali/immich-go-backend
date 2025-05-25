package server

import (
	"context"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) Login(ctx context.Context, req *immichv1.LoginRequest) (*immichv1.LoginResponse, error) {
	return &immichv1.LoginResponse{
		AccessToken:          "foo",
		UserId:               "1",
		UserEmail:            "info@example.com",
		Name:                 "Some User",
		ProfileImagePath:     "",
		IsAdmin:              true,
		ShouldChangePassword: false,
	}, nil
}
