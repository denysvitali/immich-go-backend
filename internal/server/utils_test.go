package server

import (
	"context"
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
