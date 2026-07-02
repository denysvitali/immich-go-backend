package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestRequestAuthorizationUsesImmichAccessTokenCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.AddCookie(&http.Cookie{Name: immichAccessTokenCookie, Value: "cookie-token"})

	assert.Equal(t, "Bearer cookie-token", requestAuthorization(req))
}

func TestRequestAuthorizationPrefersExplicitAuthorizationHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	req.AddCookie(&http.Cookie{Name: immichAccessTokenCookie, Value: "cookie-token"})

	assert.Equal(t, "Bearer header-token", requestAuthorization(req))
}

func TestGatewayIncomingContextUsesImmichAccessTokenCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.AddCookie(&http.Cookie{Name: immichAccessTokenCookie, Value: "cookie-token"})

	md, ok := metadata.FromIncomingContext(gatewayIncomingContext(req))
	require.True(t, ok)
	assert.Equal(t, []string{"Bearer cookie-token"}, md.Get("authorization"))
}

func TestAuthContextMiddlewareForwardsCookieAsAuthorizationHeader(t *testing.T) {
	srv := &Server{
		authService: auth.NewService(config.AuthConfig{
			JWTSecret: "test-secret-key-for-testing-only-needs-32-chars",
		}, nil),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.AddCookie(&http.Cookie{Name: immichAccessTokenCookie, Value: "invalid-token"})
	rec := httptest.NewRecorder()

	handler := srv.authContextMiddleware(runtime.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
			assert.Equal(t, "Bearer invalid-token", r.Header.Get("Authorization"))
		},
	))

	handler(rec, req, nil)
}
