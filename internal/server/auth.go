package server

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) Login(ctx context.Context, req *immichv1.LoginRequest) (*immichv1.LoginResponse, error) {
	_ = grpc.SetHeader(ctx, metadata.Pairs("x-http-code", "201"))
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", "immich_access_token=foo; Max-Age=3600; Path=/; HttpOnly; Secure; SameSite=Lax"))
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", "immich_auth_type=password; Max-Age=3600; Path=/; HttpOnly; Secure; SameSite=Lax"))
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", "immich_is_authenticated=true; Max-Age=3600; Path=/; HttpOnly; Secure; SameSite=Lax"))
	return &immichv1.LoginResponse{
		AccessToken:          "foo",
		UserId:               "1",
		UserEmail:            "info@example.com",
		Name:                 "Some User",
		ProfileImagePath:     "upload/profile/foo/1.png",
		IsAdmin:              false,
		ShouldChangePassword: false,
	}, nil
}
