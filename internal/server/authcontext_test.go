package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRequireAdminRejectsUnauthenticatedContext(t *testing.T) {
	claims, err := (&Server{}).requireAdmin(context.Background())

	require.Error(t, err)
	assert.Nil(t, claims)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Equal(t, "authentication required", status.Convert(err).Message())
}

func TestRequireAdminRejectsNonAdmin(t *testing.T) {
	ctx := auth.WithClaims(context.Background(), &auth.Claims{
		UserID:  "user-1",
		Email:   "user@example.com",
		IsAdmin: false,
	})

	claims, err := (&Server{}).requireAdmin(ctx)

	require.Error(t, err)
	assert.Nil(t, claims)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Equal(t, "admin access required", status.Convert(err).Message())
}

func TestRequireAdminReturnsAdminClaims(t *testing.T) {
	want := &auth.Claims{
		UserID:  "admin-1",
		Email:   "admin@example.com",
		IsAdmin: true,
	}

	got, err := (&Server{}).requireAdmin(auth.WithClaims(context.Background(), want))

	require.NoError(t, err)
	assert.Same(t, want, got)
}

func TestAuthenticationUnaryInterceptorEnrichesValidContext(t *testing.T) {
	wantClaims := &auth.Claims{UserID: "7ebecbc0-9128-4c3e-941c-e50999d98942"}
	wantUser := auth.UserInfo{ID: wantClaims.UserID, Email: "user@example.com"}
	ctx := auth.WithClaims(context.Background(), wantClaims)
	ctx = auth.WithUser(ctx, wantUser)

	response, err := (&Server{}).authenticationUnaryInterceptor(ctx, "request", nil,
		func(ctx context.Context, req any) (any, error) {
			claims, ok := auth.GetClaimsFromStdContext(ctx)
			require.True(t, ok)
			assert.Same(t, wantClaims, claims)
			user, ok := auth.GetUserFromStdContext(ctx)
			require.True(t, ok)
			assert.Equal(t, wantUser, *user)
			return req, nil
		})

	require.NoError(t, err)
	assert.Equal(t, "request", response)
}

// mustHTTPHandler builds the REST handler and fails the test if any gateway
// registration is rejected.
func mustHTTPHandler(t *testing.T, srv *Server) http.Handler {
	t.Helper()

	handler, err := srv.HTTPHandler()
	if err != nil {
		t.Fatalf("HTTPHandler: %v", err)
	}
	return handler
}
