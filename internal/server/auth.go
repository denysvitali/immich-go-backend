package server

import (
	"context"
	"fmt"
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

	loginResponse, err := s.authService.Login(ctx, *loginRequest)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "login failed: %v", err)
	}

	// Set HTTP headers for web compatibility
	_ = grpc.SetHeader(ctx, metadata.Pairs("x-http-code", "201"))

	// Set secure cookies
	cookieMaxAge := fmt.Sprintf("Max-Age=%d", 86400) // 24 hours
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
		UserId:               loginResponse.User.ID,
		UserEmail:            loginResponse.User.Email,
		Name:                 loginResponse.User.Name,
		ProfileImagePath:     "", // Not available in current UserInfo
		IsAdmin:              loginResponse.User.IsAdmin,
		ShouldChangePassword: false, // Not available in current UserInfo
	}, nil
}

func (s *Server) Logout(ctx context.Context, req *emptypb.Empty) (*immichv1.LogoutResponse, error) {
	// Get user from context (if authenticated)
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "not authenticated")
	}

	// Invalidate the session
	err = s.authService.Logout(ctx, userID.String())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "logout failed: %v", err)
	}

	// Clear cookies
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", "immich_access_token=; Max-Age=0; Path=/; HttpOnly; Secure; SameSite=Lax"))
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", "immich_auth_type=; Max-Age=0; Path=/; HttpOnly; Secure; SameSite=Lax"))
	_ = grpc.SetHeader(ctx, metadata.Pairs("Set-Cookie", "immich_is_authenticated=false; Max-Age=0; Path=/; Secure; SameSite=Lax"))

	return &immichv1.LogoutResponse{
		Successful:  true,
		RedirectUri: "/auth/login",
	}, nil
}

func (s *Server) AdminSignUp(ctx context.Context, req *immichv1.AdminSignUpRequest) (*immichv1.LoginResponse, error) {
	// Use the auth service to register an admin user
	registerRequest := auth.RegisterRequest{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
	}

	// Register the user (they'll be admin if this is the first user)
	response, err := s.authService.Register(ctx, registerRequest)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "admin registration failed: %v", err)
	}

	// The Register method already returns the token in AuthResponse
	return &immichv1.LoginResponse{
		AccessToken:          response.AccessToken,
		UserId:               response.User.ID,
		UserEmail:            response.User.Email,
		Name:                 response.User.Name,
		ProfileImagePath:     "", // Not available in current implementation
		IsAdmin:              response.User.IsAdmin,
		ShouldChangePassword: false, // Not available in current implementation
	}, nil
}

// ValidateToken validates the current authentication token
func (s *Server) ValidateToken(ctx context.Context, req *emptypb.Empty) (*immichv1.ValidateTokenResponse, error) {
	// Get user from context (should be set by auth middleware)
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid or expired token")
	}

	// Get user details - just validate that the user exists
	_, err = s.userService.GetUser(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}

	return &immichv1.ValidateTokenResponse{
		AuthStatus: true,
	}, nil
}

// ChangePassword changes the user's password
func (s *Server) ChangePassword(ctx context.Context, req *immichv1.ChangePasswordRequest) (*emptypb.Empty, error) {
	// Get user from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "not authenticated")
	}

	// Change the password using auth service
	err = s.authService.ChangePassword(ctx, userID.String(), auth.ChangePasswordRequest{
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	})
	if err != nil {
		if authErr, ok := err.(*auth.AuthError); ok && authErr.Type == auth.ErrInvalidCredentials {
			return nil, status.Errorf(codes.InvalidArgument, "current password is incorrect")
		}
		return nil, status.Errorf(codes.Internal, "failed to change password: %v", err)
	}

	return &emptypb.Empty{}, nil
}
