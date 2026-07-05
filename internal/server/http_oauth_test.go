package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/config"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestOAuthBackchannelLogoutRequestFromHTTP(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/oauth/backchannel-logout",
		strings.NewReader("logout_token=logout-token-123"),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	got, err := oauthBackchannelLogoutRequestFromHTTP(req)
	require.NoError(t, err)
	assert.Equal(t, "logout-token-123", got.GetLogoutToken())
}

func TestLogoutOAuthRejectsMissingLogoutToken(t *testing.T) {
	srv := &Server{config: &config.Config{}}

	_, err := srv.LogoutOAuth(context.Background(), &immichv1.OAuthBackchannelLogoutRequest{})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "logout_token is required")
}

func TestLogoutOAuthRejectsWhenOAuthDisabled(t *testing.T) {
	srv := &Server{config: &config.Config{}}

	_, err := srv.LogoutOAuth(context.Background(), &immichv1.OAuthBackchannelLogoutRequest{
		LogoutToken: "logout-token-123",
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "OAuth is not enabled")
}

func TestLogoutOAuthRejectsUnverifiableLogoutToken(t *testing.T) {
	srv := &Server{config: &config.Config{
		Auth: config.AuthConfig{
			OAuth: config.OAuthConfig{Enabled: true},
		},
	}}

	_, err := srv.LogoutOAuth(context.Background(), &immichv1.OAuthBackchannelLogoutRequest{
		LogoutToken: "logout-token-123",
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "token validation failed")
}

func TestHandleOAuthBackchannelLogoutDoesNotRequireAuth(t *testing.T) {
	srv := &Server{config: &config.Config{}}
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/oauth/backchannel-logout",
		strings.NewReader("logout_token=logout-token-123"),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	srv.handleOAuthBackchannelLogout(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "OAuth is not enabled")
}
