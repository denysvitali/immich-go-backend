package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	immichws "github.com/denysvitali/immich-go-backend/internal/websocket"
	gorilla "github.com/gorilla/websocket"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestRequestAuthorizationUsesImmichAccessTokenCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.AddCookie(testAccessTokenCookie("cookie-token"))

	assert.Equal(t, "Bearer cookie-token", requestAuthorization(req))
}

func TestRequestAuthorizationPrefersExplicitAuthorizationHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	req.AddCookie(testAccessTokenCookie("cookie-token"))

	assert.Equal(t, "Bearer header-token", requestAuthorization(req))
}

func TestGatewayIncomingContextUsesImmichAccessTokenCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.AddCookie(testAccessTokenCookie("cookie-token"))

	md, ok := metadata.FromIncomingContext(gatewayIncomingContext(req))
	require.True(t, ok)
	assert.Equal(t, []string{"Bearer cookie-token"}, md.Get("authorization"))
}

func TestGatewayIncomingContextIncludesAPIKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("x-api-key", "immich_test-key")

	md, ok := metadata.FromIncomingContext(gatewayIncomingContext(req))
	require.True(t, ok)
	assert.Equal(t, []string{"immich_test-key"}, md.Get("x-api-key"))
}

func TestAuthContextMiddlewareForwardsCookieAsAuthorizationHeader(t *testing.T) {
	srv := &Server{
		authService: auth.NewService(config.AuthConfig{
			JWTSecret: "test-secret-key-for-testing-only-needs-32-chars",
		}, nil),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.AddCookie(testAccessTokenCookie("invalid-token"))
	rec := httptest.NewRecorder()

	handler := srv.authContextMiddleware(runtime.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
			assert.Equal(t, "Bearer invalid-token", r.Header.Get("Authorization"))
		},
	))

	handler(rec, req, nil)
}

func TestWebSocketRouteRequiresAuthentication(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/socket.io/", nil)
	rec := httptest.NewRecorder()

	srv.handleWs(nil).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestWebSocketRouteAcceptsResolvedAuthenticationContext(t *testing.T) {
	hub := immichws.New()
	go hub.Run()
	t.Cleanup(hub.Close)
	srv := &Server{wsHub: hub}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := &auth.Claims{UserID: "7ebecbc0-9128-4c3e-941c-e50999d98942", Email: "ws@test.com"}
		ctx := auth.WithClaims(r.Context(), claims)
		ctx = auth.WithUser(ctx, auth.UserInfo{ID: claims.UserID, Email: claims.Email})
		srv.handleWs(nil).ServeHTTP(w, r.WithContext(ctx))
	})
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/api/socket.io/"
	conn, resp, err := gorilla.DefaultDialer.Dial(wsURL, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
}

func testAccessTokenCookie(value string) *http.Cookie {
	return &http.Cookie{
		Name:     immichAccessTokenCookie,
		Value:    value,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}
