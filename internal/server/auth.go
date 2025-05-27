package server

import (
	"context"
	"crypto/rand"
	"math/big"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) Login(ctx context.Context, req *immichv1.LoginRequest) (*immichv1.LoginResponse, error) {
	_ = grpc.SetHeader(ctx, metadata.Pairs("x-http-code", "201"))
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", "immich_access_token=foo; Max-Age=3600; Path=/; HttpOnly; Secure; SameSite=Lax"))
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", "immich_auth_type=password; Max-Age=3600; Path=/; HttpOnly; Secure; SameSite=Lax"))
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", "immich_is_authenticated=true; Max-Age=3600; Path=/; Secure; SameSite=Lax"))
	return &immichv1.LoginResponse{
		AccessToken:          randomString(42),
		UserId:               uuid.New().String(),
		UserEmail:            "info@example.com",
		Name:                 "Some User",
		ProfileImagePath:     "upload/profile/foo/1.png",
		IsAdmin:              true,
		ShouldChangePassword: true,
	}, nil
}

func randomString(i int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, i)
	for j := range b {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[j] = charset[num.Int64()]
	}
	return string(b)
}
