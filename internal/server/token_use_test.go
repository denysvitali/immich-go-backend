package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestRefreshTokenRejectedAtHTTPAndGRPCAuthenticationBoundaries(t *testing.T) {
	const secret = "test-secret-key-for-testing-only-needs-32-chars"
	now := time.Now()
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &auth.Claims{
		UserID:   uuid.NewString(),
		Email:    "refresh-boundary@test.com",
		TokenUse: auth.TokenUseRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}).SignedString([]byte(secret))
	require.NoError(t, err)

	srv := &Server{authService: auth.NewService(config.AuthConfig{JWTSecret: secret}, nil)}
	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+refreshToken)

	authenticatedRequest, err := srv.authenticateHTTPRequest(req)
	require.Error(t, err)
	assert.Nil(t, authenticatedRequest)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	grpcCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer "+refreshToken,
	))
	claims, err := srv.getUserFromContext(grpcCtx)
	require.Error(t, err)
	assert.Nil(t, claims)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}
