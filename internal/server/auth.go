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

// GetAuthStatus returns the current auth status including session info
func (s *Server) GetAuthStatus(ctx context.Context, req *emptypb.Empty) (*immichv1.AuthStatusResponse, error) {
	// Get user from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "not authenticated")
	}

	// Get user details to check if they have a password set
	_, err = s.userService.GetUser(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}

	// Check if user has a PIN code set
	hasPinCode, err := s.authService.HasPinCode(ctx, userID.String())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check PIN code status: %v", err)
	}

	// Check if session is elevated
	sessionID, _ := s.getSessionIDFromContext(ctx)
	isElevated := false
	if sessionID != "" {
		isElevated, _ = s.authService.IsSessionElevated(ctx, sessionID)
	}

	// Assume users have passwords unless they only use OAuth
	// (OAuth-only users would have an empty password hash)
	hasPassword := true

	return &immichv1.AuthStatusResponse{
		HasPassword:    hasPassword,
		IsElevated:     isElevated,
		PinCodeEnabled: hasPinCode,
	}, nil
}

// SetupPinCode sets up a new PIN code for the current user
func (s *Server) SetupPinCode(ctx context.Context, req *immichv1.PinCodeSetupRequest) (*emptypb.Empty, error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "not authenticated")
	}

	err = s.authService.SetupPinCode(ctx, userID.String(), req.PinCode)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to setup PIN code: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ChangePinCode changes the PIN code for the current user
func (s *Server) ChangePinCode(ctx context.Context, req *immichv1.PinCodeChangeRequest) (*emptypb.Empty, error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "not authenticated")
	}

	err = s.authService.ChangePinCode(ctx, userID.String(), req.CurrentPinCode, req.NewPinCode)
	if err != nil {
		if authErr, ok := err.(*auth.AuthError); ok && authErr.Type == auth.ErrInvalidCredentials {
			return nil, status.Errorf(codes.InvalidArgument, "current PIN code is incorrect")
		}
		return nil, status.Errorf(codes.Internal, "failed to change PIN code: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// ResetPinCode resets the PIN code by verifying account password
func (s *Server) ResetPinCode(ctx context.Context, req *immichv1.PinCodeResetRequest) (*emptypb.Empty, error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "not authenticated")
	}

	err = s.authService.ResetPinCode(ctx, userID.String(), req.Password)
	if err != nil {
		if authErr, ok := err.(*auth.AuthError); ok && authErr.Type == auth.ErrInvalidCredentials {
			return nil, status.Errorf(codes.InvalidArgument, "password is incorrect")
		}
		return nil, status.Errorf(codes.Internal, "failed to reset PIN code: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// UnlockSession unlocks the session with a PIN code for elevated access
func (s *Server) UnlockSession(ctx context.Context, req *immichv1.SessionUnlockRequest) (*emptypb.Empty, error) {
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "not authenticated")
	}

	sessionID, err := s.getSessionIDFromContext(ctx)
	if err != nil || sessionID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "no session found")
	}

	err = s.authService.UnlockSession(ctx, userID.String(), sessionID, req.PinCode)
	if err != nil {
		if authErr, ok := err.(*auth.AuthError); ok && authErr.Type == auth.ErrInvalidCredentials {
			return nil, status.Errorf(codes.InvalidArgument, "PIN code is incorrect")
		}
		return nil, status.Errorf(codes.Internal, "failed to unlock session: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// LockSession locks the session to revoke elevated access
func (s *Server) LockSession(ctx context.Context, req *emptypb.Empty) (*emptypb.Empty, error) {
	_, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "not authenticated")
	}

	sessionID, err := s.getSessionIDFromContext(ctx)
	if err != nil || sessionID == "" {
		return nil, status.Errorf(codes.InvalidArgument, "no session found")
	}

	err = s.authService.LockSession(ctx, sessionID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to lock session: %v", err)
	}

	return &emptypb.Empty{}, nil
}
