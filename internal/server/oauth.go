package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/oauth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const mobileOAuthCallbackURL = "app.immich:///oauth-callback"

// Ensure Server implements OAuthServiceServer
var _ immichv1.OAuthServiceServer = (*Server)(nil)

func mobileOAuthRedirectURL(rawQuery string) string {
	return fmt.Sprintf("%s?%s", mobileOAuthCallbackURL, rawQuery)
}

// AuthorizeOAuth initiates OAuth authorization flow
func (s *Server) AuthorizeOAuth(ctx context.Context, req *immichv1.AuthorizeOAuthRequest) (*immichv1.AuthorizeOAuthResponse, error) {
	// Create OAuth service
	oauthService := oauth.NewService(s.db.Queries, s.config)

	// Generate state for CSRF protection
	state, err := oauthService.GenerateState()
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to generate state", err)
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
		return nil, SanitizedInternal(ctx, "failed to exchange code", err)
	}

	// Get user info from provider
	userInfo, err := oauthService.GetUserInfo(req.Provider, accessToken)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get user info", err)
	}

	// Find or create user
	user, err := oauthService.FindOrCreateUserByOAuth(ctx, userInfo)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to find or create user", err)
	}

	// Prefer a durable session row (with optional OIDC sid) so backchannel
	// logout can invalidate it. Fall back to a bare JWT when sessions are
	// unavailable.
	token := ""
	if s.sessionsService != nil {
		session, sessErr := s.sessionsService.CreateOAuthSession(
			ctx,
			user.ID.String(),
			"oauth",
			req.Provider,
			"", // sid is filled when ExchangeCodeForToken returns an id_token
		)
		if sessErr == nil && session != nil {
			token = session.Token
		}
	}
	if token == "" {
		var tokErr error
		token, tokErr = s.authService.GenerateToken(user.ID.String(), user.Email, 24*time.Hour)
		if tokErr != nil {
			return nil, SanitizedInternal(ctx, "failed to generate token", tokErr)
		}
	}

	// Set cookie in response metadata
	md := metadata.Pairs(
		"set-cookie", fmt.Sprintf("immich_access_token=%s; Path=/; HttpOnly; SameSite=Lax; Max-Age=%d",
			token, int(24*time.Hour.Seconds())),
	)
	if err := grpc.SetHeader(ctx, md); err != nil {
		return nil, SanitizedInternal(ctx, "failed to set cookie", err)
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
		return nil, SanitizedInternal(ctx, "failed to link account", err)
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
		return nil, SanitizedInternal(ctx, "failed to unlink account", err)
	}

	return &immichv1.UnlinkOAuthAccountResponse{
		Unlinked: true,
	}, nil
}

// LogoutOAuth handles OIDC backchannel logout requests (RFC / Immich parity).
// Validates the logout_token JWT, then deletes sessions matching sub and/or sid.
func (s *Server) LogoutOAuth(ctx context.Context, req *immichv1.OAuthBackchannelLogoutRequest) (*emptypb.Empty, error) {
	if strings.TrimSpace(req.GetLogoutToken()) == "" {
		return nil, status.Error(codes.InvalidArgument, "logout_token is required")
	}

	oidcCfg, err := s.oauthOIDCConfig(ctx)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to load OAuth config", err)
	}
	if !oidcCfg.Enabled {
		return nil, status.Error(codes.InvalidArgument, "received backchannel logout request but OAuth is not enabled")
	}

	sub, sid, err := oauth.ValidateLogoutToken(oidcCfg, req.GetLogoutToken())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "error backchannel logout: token validation failed")
	}

	if s.db != nil && s.db.Queries != nil {
		if _, invErr := oauth.InvalidateOAuthSessions(ctx, s.db.Queries, sub, sid); invErr != nil {
			return nil, SanitizedInternal(ctx, "failed to invalidate OAuth sessions", invErr)
		}
	}

	return &emptypb.Empty{}, nil
}

// oauthOIDCConfig resolves Immich-style OIDC settings from system config when
// available, falling back to auth.oauth in the process config.
func (s *Server) oauthOIDCConfig(ctx context.Context) (oauth.OIDCConfig, error) {
	cfg := oauth.OIDCConfig{
		Enabled:          s.config != nil && s.config.Auth.OAuth.Enabled,
		SigningAlgorithm: "RS256",
	}

	if s.systemConfigService != nil {
		dto, err := s.systemConfigService.GetConfigDto(ctx)
		if err != nil {
			return cfg, err
		}
		cfg.Enabled = dto.OAuth.Enabled
		cfg.IssuerURL = dto.OAuth.IssuerURL
		cfg.ClientID = dto.OAuth.ClientID
		cfg.ClientSecret = dto.OAuth.ClientSecret
		if dto.OAuth.SigningAlgorithm != "" {
			cfg.SigningAlgorithm = dto.OAuth.SigningAlgorithm
		}
		return cfg, nil
	}

	// Process-level multi-provider config: map first enabled provider for HS
	// validation when an issuer is not configured via system config.
	if s.config != nil {
		o := s.config.Auth.OAuth
		cfg.Enabled = o.Enabled || o.Google.Enabled || o.GitHub.Enabled || o.Microsoft.Enabled
		switch {
		case o.Google.Enabled:
			cfg.ClientID = o.Google.ClientID
			cfg.ClientSecret = o.Google.ClientSecret
		case o.GitHub.Enabled:
			cfg.ClientID = o.GitHub.ClientID
			cfg.ClientSecret = o.GitHub.ClientSecret
		case o.Microsoft.Enabled:
			cfg.ClientID = o.Microsoft.ClientID
			cfg.ClientSecret = o.Microsoft.ClientSecret
		}
	}
	return cfg, nil
}
