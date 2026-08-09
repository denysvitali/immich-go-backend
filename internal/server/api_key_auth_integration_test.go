//go:build integration

package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/apikeys"
	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/denysvitali/immich-go-backend/internal/users"
	immichws "github.com/denysvitali/immich-go-backend/internal/websocket"
	"github.com/google/uuid"
	gorilla "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestIntegrationAPIKeyAuthenticatesFullHTTPHandlerAndDirectRPC(t *testing.T) {
	testdb.SkipIfNoDocker(t)
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()
	conn, err := db.New(ctx, tdb.ConnStr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	cfg := &config.Config{Auth: config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		JWTRefreshExpiry:    24 * time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}}
	authService := auth.NewService(cfg.Auth, tdb.Queries)
	firstUser, err := authService.Register(ctx, auth.RegisterRequest{
		Email:    "api-key-auth@test.com",
		Password: "SecurePass123!",
		Name:     "API Key Auth User",
	})
	require.NoError(t, err)
	secondUser, err := authService.Register(ctx, auth.RegisterRequest{
		Email:    "other-api-key-auth@test.com",
		Password: "SecurePass123!",
		Name:     "Other API Key Auth User",
	})
	require.NoError(t, err)

	apiKeyService := apikeys.NewService(tdb.Queries)
	_, rawKey, err := apiKeyService.CreateAPIKey(ctx, uuid.MustParse(firstUser.User.ID), "integration")
	require.NoError(t, err)
	_, otherRawKey, err := apiKeyService.CreateAPIKey(ctx, uuid.MustParse(secondUser.User.ID), "other integration")
	require.NoError(t, err)

	userService, err := users.NewService(tdb.Queries, cfg)
	require.NoError(t, err)
	hub := immichws.New()
	go hub.Run()
	t.Cleanup(hub.Close)
	srv := &Server{
		config:      cfg,
		db:          conn,
		authService: authService,
		wsHub:       hub,
		services: services{
			apiKeyService: apiKeyService,
			userService:   userService,
		},
	}
	credential := "immich_" + rawKey
	otherCredential := "immich_" + otherRawKey
	httpServer := httptest.NewServer(mustHTTPHandler(t, srv))
	t.Cleanup(httpServer.Close)

	t.Run("hand-written users me route", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, httpServer.URL+"/api/users/me", nil)
		require.NoError(t, err)
		req.Header.Set("x-api-key", credential)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode, string(body))
		assert.Contains(t, string(body), firstUser.User.Email)
	})

	t.Run("generated gateway route", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, httpServer.URL+"/api/api-keys/me", nil)
		require.NoError(t, err)
		req.Header.Set("x-api-key", credential)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode, string(body))
		assert.Contains(t, string(body), `"name":"integration"`)
	})

	t.Run("authenticated user read route", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, httpServer.URL+"/api/users/"+secondUser.User.ID, nil)
		require.NoError(t, err)
		req.Header.Set("x-api-key", credential)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode, string(body))
		assert.Contains(t, string(body), secondUser.User.Email)
	})

	t.Run("authenticated profile image route", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, httpServer.URL+"/api/users/"+firstUser.User.ID+"/profile-image", nil)
		require.NoError(t, err)
		req.Header.Set("x-api-key", credential)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("frontend compatibility route", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, httpServer.URL+"/api/users/me/onboarding", nil)
		require.NoError(t, err)
		req.Header.Set("x-api-key", credential)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	t.Run("invalid API key", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, httpServer.URL+"/api/users/me", nil)
		require.NoError(t, err)
		req.Header.Set("x-api-key", "immich_invalid")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("credentials for different users", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, httpServer.URL+"/api/users/me", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+firstUser.AccessToken)
		req.Header.Set("x-api-key", otherCredential)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("websocket upgrade", func(t *testing.T) {
		header := http.Header{"x-api-key": []string{credential}}
		wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/socket.io/"
		conn, resp, err := gorilla.DefaultDialer.Dial(wsURL, header)
		if resp != nil && resp.Body != nil {
			defer resp.Body.Close()
		}
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
	})

	grpcCtx := metadata.NewIncomingContext(ctx, metadata.Pairs("x-api-key", credential))
	claims, err := srv.getUserFromContext(grpcCtx)
	require.NoError(t, err)
	assert.Equal(t, firstUser.User.ID, claims.UserID)
	assert.Equal(t, firstUser.User.Email, claims.Email)

	enriched := srv.authenticatedContext(grpcCtx)
	_, hasClaims := auth.GetClaimsFromStdContext(enriched)
	_, hasUser := auth.GetUserFromStdContext(enriched)
	assert.True(t, hasClaims)
	assert.True(t, hasUser)
}
