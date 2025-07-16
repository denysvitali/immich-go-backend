package server

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) Login(ctx context.Context, req *immichv1.LoginRequest) (*immichv1.LoginResponse, error) {
	// Use the actual auth service
	loginRequest := &auth.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	}

	loginResponse, err := s.authService.Login(ctx, loginRequest)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "login failed: %v", err)
	}

	// Set HTTP headers for web compatibility
	_ = grpc.SetHeader(ctx, metadata.Pairs("x-http-code", "201"))
	
	// Set secure cookies
	cookieMaxAge := fmt.Sprintf("Max-Age=%d", int(loginResponse.ExpiresIn.Seconds()))
	accessTokenCookie := fmt.Sprintf("immich_access_token=%s; %s; Path=/; HttpOnly; Secure; SameSite=Lax", 
		loginResponse.AccessToken, cookieMaxAge)
	authTypeCookie := fmt.Sprintf("immich_auth_type=password; %s; Path=/; HttpOnly; Secure; SameSite=Lax", 
		cookieMaxAge)
	isAuthenticatedCookie := fmt.Sprintf("immich_is_authenticated=true; %s; Path=/; Secure; SameSite=Lax", 
		cookieMaxAge)
	
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", accessTokenCookie))
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", authTypeCookie))
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", isAuthenticatedCookie))

	return &immichv1.LoginResponse{
		AccessToken:          loginResponse.AccessToken,
		UserId:               loginResponse.User.ID.String(),
		UserEmail:            loginResponse.User.Email,
		Name:                 loginResponse.User.Name,
		ProfileImagePath:     loginResponse.User.ProfileImagePath,
		IsAdmin:              loginResponse.User.IsAdmin,
		ShouldChangePassword: loginResponse.User.ShouldChangePassword,
	}, nil
}

func (s *Server) Logout(ctx context.Context, req *emptypb.Empty) (*immichv1.LogoutResponse, error) {
	// Get user from context (if authenticated)
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "not authenticated")
	}

	// Invalidate the session
	err = s.authService.Logout(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "logout failed: %v", err)
	}

	// Clear cookies
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", "immich_access_token=; Max-Age=0; Path=/; HttpOnly; Secure; SameSite=Lax"))
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", "immich_auth_type=; Max-Age=0; Path=/; HttpOnly; Secure; SameSite=Lax"))
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", "immich_is_authenticated=false; Max-Age=0; Path=/; Secure; SameSite=Lax"))

	return &immichv1.LogoutResponse{
		Successful: true,
		RedirectUri: "/auth/login",
	}, nil
}

func (s *Server) SignUpAdmin(ctx context.Context, req *immichv1.SignUpAdminRequest) (*immichv1.UserResponse, error) {
	// Use the auth service to register an admin user
	registerRequest := &auth.RegisterRequest{
		Email:           req.Email,
		Password:        req.Password,
		Name:            req.Name,
		IsAdmin:         true,
	}

	user, err := s.authService.Register(ctx, registerRequest)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "admin registration failed: %v", err)
	}

	return &immichv1.UserResponse{
		Id:                   user.ID.String(),
		Email:                user.Email,
		Name:                 user.Name,
		ProfileImagePath:     user.ProfileImagePath,
		IsAdmin:              user.IsAdmin,
		ShouldChangePassword: user.ShouldChangePassword,
		CreatedAt:            timestampFromTime(user.CreatedAt),
		UpdatedAt:            timestampFromTime(user.UpdatedAt),
	}, nil
}

func (s *Server) ValidateAccessToken(ctx context.Context, req *immichv1.ValidateAccessTokenRequest) (*immichv1.ValidateAccessTokenResponse, error) {
	// Validate the provided access token
	userInfo, err := s.authService.ValidateToken(ctx, req.AccessToken)
	if err != nil {
		return &immichv1.ValidateAccessTokenResponse{
			AuthStatus: false,
		}, nil
	}

	return &immichv1.ValidateAccessTokenResponse{
		AuthStatus: true,
		User: &immichv1.UserResponse{
			Id:                   userInfo.ID.String(),
			Email:                userInfo.Email,
			Name:                 userInfo.Name,
			ProfileImagePath:     userInfo.ProfileImagePath,
			IsAdmin:              userInfo.IsAdmin,
			ShouldChangePassword: userInfo.ShouldChangePassword,
			CreatedAt:            timestampFromTime(userInfo.CreatedAt),
			UpdatedAt:            timestampFromTime(userInfo.UpdatedAt),
		},
	}, nil
}
