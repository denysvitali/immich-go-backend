//go:build integration

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestIntegrationRefreshTokenCannotAuthenticateRESTOrGRPC(t *testing.T) {
	testdb.SkipIfNoDocker(t)
	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	authService := auth.NewService(config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           time.Hour,
		JWTRefreshExpiry:    24 * time.Hour,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}, tdb.Queries)
	registered, err := authService.Register(ctx, auth.RegisterRequest{
		Email:    "refresh-boundary@test.com",
		Password: "SecurePass123!",
		Name:     "Refresh Boundary User",
	})
	require.NoError(t, err)

	srv := &Server{authService: authService}

	// The REST authentication middleware must leave a request carrying a
	// refresh token anonymous, so protected handlers reject it.
	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+registered.RefreshToken)
	authenticatedRequest, err := srv.authenticateHTTPRequest(req)
	require.Error(t, err)
	assert.Nil(t, authenticatedRequest)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	// The gateway middleware also must not enrich the context when exercised
	// directly without the outer HTTP authentication handler.
	rec := httptest.NewRecorder()
	srv.authContextMiddleware(runtime.HandlerFunc(func(_ http.ResponseWriter, r *http.Request, _ map[string]string) {
		_, hasClaims := auth.GetClaimsFromStdContext(r.Context())
		_, hasUser := auth.GetUserFromStdContext(r.Context())
		assert.False(t, hasClaims)
		assert.False(t, hasUser)
		_, requireErr := auth.RequireUser(r.Context())
		require.Error(t, requireErr)
	}))(rec, req, nil)

	// The direct gRPC boundary uses the same access validator and returns
	// Unauthenticated before the RPC can execute as the refresh-token subject.
	grpcCtx := metadata.NewIncomingContext(ctx, metadata.Pairs(
		"authorization", "Bearer "+registered.RefreshToken,
	))
	claims, err := srv.getUserFromContext(grpcCtx)
	require.Error(t, err)
	assert.Nil(t, claims)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	response, err := srv.ValidateToken(grpcCtx, &emptypb.Empty{})
	require.Error(t, err)
	assert.Nil(t, response)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}
