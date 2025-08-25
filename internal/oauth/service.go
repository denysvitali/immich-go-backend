package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// OAuthProvider represents an OAuth provider configuration
type OAuthProvider struct {
	Name         string
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
	RedirectURL  string
}

// Service handles OAuth authentication
type Service struct {
	db        *sqlc.Queries
	config    *config.Config
	providers map[string]*OAuthProvider
}

// NewService creates a new OAuth service
func NewService(db *sqlc.Queries, cfg *config.Config) *Service {
	s := &Service{
		db:        db,
		config:    cfg,
		providers: make(map[string]*OAuthProvider),
	}
	
	// Initialize OAuth providers from config
	s.initProviders()
	
	return s
}

// initProviders initializes OAuth providers from configuration
func (s *Service) initProviders() {
	// Google OAuth
	if s.config.Auth.OAuth.Google.Enabled {
		s.providers["google"] = &OAuthProvider{
			Name:         "google",
			ClientID:     s.config.Auth.OAuth.Google.ClientID,
			ClientSecret: s.config.Auth.OAuth.Google.ClientSecret,
			AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:     "https://oauth2.googleapis.com/token",
			UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
			Scopes:       []string{"openid", "email", "profile"},
			RedirectURL:  s.config.Auth.OAuth.Google.RedirectURL,
		}
	}
	
	// GitHub OAuth
	if s.config.Auth.OAuth.GitHub.Enabled {
		s.providers["github"] = &OAuthProvider{
			Name:         "github",
			ClientID:     s.config.Auth.OAuth.GitHub.ClientID,
			ClientSecret: s.config.Auth.OAuth.GitHub.ClientSecret,
			AuthURL:      "https://github.com/login/oauth/authorize",
			TokenURL:     "https://github.com/login/oauth/access_token",
			UserInfoURL:  "https://api.github.com/user",
			Scopes:       []string{"user:email"},
			RedirectURL:  s.config.Auth.OAuth.GitHub.RedirectURL,
		}
	}
	
	// Microsoft OAuth
	if s.config.Auth.OAuth.Microsoft.Enabled {
		s.providers["microsoft"] = &OAuthProvider{
			Name:         "microsoft",
			ClientID:     s.config.Auth.OAuth.Microsoft.ClientID,
			ClientSecret: s.config.Auth.OAuth.Microsoft.ClientSecret,
			AuthURL:      "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/token",
			UserInfoURL:  "https://graph.microsoft.com/v1.0/me",
			Scopes:       []string{"openid", "email", "profile"},
			RedirectURL:  s.config.Auth.OAuth.Microsoft.RedirectURL,
		}
	}
}

// GenerateState generates a random state parameter for OAuth
func (s *Service) GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GetAuthorizationURL returns the OAuth authorization URL for a provider
func (s *Service) GetAuthorizationURL(provider, state string) (string, error) {
	p, exists := s.providers[provider]
	if !exists {
		return "", fmt.Errorf("provider %s not configured", provider)
	}
	
	params := url.Values{}
	params.Set("client_id", p.ClientID)
	params.Set("redirect_uri", p.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(p.Scopes, " "))
	params.Set("state", state)
	
	// Add provider-specific parameters
	if provider == "google" {
		params.Set("access_type", "offline")
		params.Set("prompt", "consent")
	}
	
	return fmt.Sprintf("%s?%s", p.AuthURL, params.Encode()), nil
}

// ExchangeCodeForToken exchanges an authorization code for an access token
func (s *Service) ExchangeCodeForToken(provider, code string) (string, error) {
	p, exists := s.providers[provider]
	if !exists {
		return "", fmt.Errorf("provider %s not configured", provider)
	}
	
	data := url.Values{}
	data.Set("client_id", p.ClientID)
	data.Set("client_secret", p.ClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", p.RedirectURL)
	
	resp, err := http.PostForm(p.TokenURL, data)
	if err != nil {
		return "", fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	
	var tokenResp map[string]interface{}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}
	
	accessToken, ok := tokenResp["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("no access token in response")
	}
	
	return accessToken, nil
}

// GetUserInfo fetches user information from the OAuth provider
func (s *Service) GetUserInfo(provider, accessToken string) (*OAuthUserInfo, error) {
	p, exists := s.providers[provider]
	if !exists {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}
	
	req, err := http.NewRequest("GET", p.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	
	// Set authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	
	// GitHub requires a specific Accept header
	if provider == "github" {
		req.Header.Set("Accept", "application/vnd.github.v3+json")
	}
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}
	
	// Parse user info based on provider
	userInfo := &OAuthUserInfo{
		Provider: provider,
	}
	
	switch provider {
	case "google":
		userInfo.ID = getString(data, "id")
		userInfo.Email = getString(data, "email")
		userInfo.Name = getString(data, "name")
		userInfo.Picture = getString(data, "picture")
		
	case "github":
		userInfo.ID = fmt.Sprintf("%v", data["id"])
		userInfo.Email = getString(data, "email")
		userInfo.Name = getString(data, "name")
		userInfo.Picture = getString(data, "avatar_url")
		
	case "microsoft":
		userInfo.ID = getString(data, "id")
		userInfo.Email = getString(data, "mail")
		if userInfo.Email == "" {
			userInfo.Email = getString(data, "userPrincipalName")
		}
		userInfo.Name = getString(data, "displayName")
	}
	
	return userInfo, nil
}

// LinkOAuthAccount links an OAuth account to an existing user
func (s *Service) LinkOAuthAccount(ctx context.Context, userID uuid.UUID, provider string, providerID string) error {
	// Store OAuth link in database
	// This would require adding an oauth_accounts table to track linked accounts
	// For now, this is a placeholder
	return fmt.Errorf("LinkOAuthAccount not implemented - needs database schema update")
}

// FindOrCreateUserByOAuth finds or creates a user based on OAuth info
func (s *Service) FindOrCreateUserByOAuth(ctx context.Context, userInfo *OAuthUserInfo) (*sqlc.User, error) {
	// First, try to find user by email
	user, err := s.db.GetUserByEmail(ctx, userInfo.Email)
	if err == nil {
		return &user, nil
	}
	
	// If user doesn't exist, create a new one
	// Generate a random password since OAuth users don't need one
	randomPass := make([]byte, 32)
	if _, err := rand.Read(randomPass); err != nil {
		return nil, err
	}
	
	newUser, err := s.db.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Email:    userInfo.Email,
		Name:     userInfo.Name,
		Password: base64.URLEncoding.EncodeToString(randomPass), // Random password for OAuth users
		IsAdmin:  false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	
	return &newUser, nil
}

// OAuthUserInfo represents user information from an OAuth provider
type OAuthUserInfo struct {
	Provider string
	ID       string
	Email    string
	Name     string
	Picture  string
}

// getString safely extracts a string from a map
func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}