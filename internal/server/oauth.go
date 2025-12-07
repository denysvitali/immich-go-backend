package server

import (
	"context"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/oauth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Ensure Server implements OAuthServiceServer
var _ immichv1.OAuthServiceServer = (*Server)(nil)

// AuthorizeOAuth initiates OAuth authorization flow
func (s *Server) AuthorizeOAuth(ctx context.Context, req *immichv1.AuthorizeOAuthRequest) (*immichv1.AuthorizeOAuthResponse, error) {
	// Create OAuth service
	oauthService := oauth.NewService(s.db.Queries, s.config)

	// Generate state for CSRF protection
	state, err := oauthService.GenerateState()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate state: %v", err)
	}

	// Store state in session/cache (simplified - should use proper session storage)
	// For now, we'll pass it through and validate on callback

	// Get authorization URL
	authURL, err := oauthService.GetAuthorizationURL(req.Provider, state)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid provider: %v", err)
	}

	return &immichv1.AuthorizeOAuthResponse{
		Url: authURL,
	}, nil
}

// CallbackOAuth handles OAuth callback
func (s *Server) CallbackOAuth(ctx context.Context, req *immichv1.CallbackOAuthRequest) (*immichv1.CallbackOAuthResponse, error) {
	// Validate state parameter (simplified - should check against stored state)
	if req.State == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid state parameter")
	}

	// Create OAuth service
	oauthService := oauth.NewService(s.db.Queries, s.config)

	// Exchange code for token
	accessToken, err := oauthService.ExchangeCodeForToken(req.Provider, req.Code)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to exchange code: %v", err)
	}

	// Get user info from provider
	userInfo, err := oauthService.GetUserInfo(req.Provider, accessToken)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user info: %v", err)
	}

	// Find or create user
	user, err := oauthService.FindOrCreateUserByOAuth(ctx, userInfo)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to find or create user: %v", err)
	}

	// Generate JWT token for the user
	token, err := s.authService.GenerateToken(user.ID.String(), user.Email, 24*time.Hour)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate token: %v", err)
	}

	// Set cookie in response metadata
	md := metadata.Pairs(
		"set-cookie", fmt.Sprintf("immich_access_token=%s; Path=/; HttpOnly; SameSite=Lax; Max-Age=%d",
			token, int(24*time.Hour.Seconds())),
	)
	if err := grpc.SetHeader(ctx, md); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set cookie: %v", err)
	}

	return &immichv1.CallbackOAuthResponse{
		AccessToken: token,
		UserId:      user.ID.String(),
		UserEmail:   user.Email,
		IsAdmin:     user.IsAdmin,
		Name:        user.Name,
	}, nil
}

// GenerateOAuthConfig generates OAuth configuration for the client
func (s *Server) GenerateOAuthConfig(ctx context.Context, req *immichv1.GenerateOAuthConfigRequest) (*immichv1.GenerateOAuthConfigResponse, error) {
	// Create response with available OAuth providers
	response := &immichv1.GenerateOAuthConfigResponse{
		Enabled: false,
		Url:     "",
	}

	// Check if any OAuth provider is enabled
	if s.config.Auth.OAuth.Google.Enabled ||
		s.config.Auth.OAuth.GitHub.Enabled ||
		s.config.Auth.OAuth.Microsoft.Enabled {
		response.Enabled = true
		// Return the authorization URL for the requested provider
		oauthService := oauth.NewService(s.db.Queries, s.config)
		state, _ := oauthService.GenerateState()
		url, err := oauthService.GetAuthorizationURL(req.Provider, state)
		if err == nil {
			response.Url = url
		}
	}

	return response, nil
}

// LinkOAuthAccount links an OAuth account to the current user
func (s *Server) LinkOAuthAccount(ctx context.Context, req *immichv1.LinkOAuthAccountRequest) (*immichv1.LinkOAuthAccountResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Create OAuth service
	oauthService := oauth.NewService(s.db.Queries, s.config)

	// Link the OAuth account
	if err := oauthService.LinkOAuthAccount(ctx, userID, req.Provider, req.ProviderId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to link account: %v", err)
	}

	return &immichv1.LinkOAuthAccountResponse{
		Success: true,
	}, nil
}

// UnlinkOAuthAccount unlinks an OAuth account from the current user
func (s *Server) UnlinkOAuthAccount(ctx context.Context, _ *emptypb.Empty) (*immichv1.UnlinkOAuthAccountResponse, error) {
	// Get user ID from context
	userID, err := s.getUserIDFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	// Create OAuth service
	oauthService := oauth.NewService(s.db.Queries, s.config)

	// Unlink the OAuth account
	if err := oauthService.UnlinkOAuthAccount(ctx, userID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unlink account: %v", err)
	}

	return &immichv1.UnlinkOAuthAccountResponse{
		Unlinked: true,
	}, nil
}
