package server

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPublicAuthErrorOverridesInvalidCredentialsCode(t *testing.T) {
	err := auth.NewInvalidCredentialsError("Current password is incorrect")

	got, ok := publicAuthError(context.Background(), err, codes.InvalidArgument)

	require.True(t, ok)
	st, ok := status.FromError(got)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Equal(t, "Current password is incorrect", st.Message())
}

func TestPublicAuthErrorUsesInvalidPasswordCause(t *testing.T) {
	err := auth.NewAuthError(
		auth.ErrInvalidPassword,
		"Password does not meet requirements",
		errors.New("password must be at least 8 characters long"),
	)

	got, ok := publicAuthError(context.Background(), err, codes.OK)

	require.True(t, ok)
	st, ok := status.FromError(got)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Equal(t, "password must be at least 8 characters long", st.Message())
}

func TestPublicAuthErrorHandlesWrappedAuthError(t *testing.T) {
	err := fmt.Errorf("handler failed: %w", auth.NewAuthError(auth.ErrRateLimited, "Too many failed login attempts", nil))

	got, ok := publicAuthError(context.Background(), err, codes.OK)

	require.True(t, ok)
	st, ok := status.FromError(got)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, st.Code())
	assert.Equal(t, "Too many failed login attempts", st.Message())
}

func TestPublicAuthErrorRejectsOperationalAuthError(t *testing.T) {
	got, ok := publicAuthError(context.Background(), auth.NewAuthError(auth.ErrTokenStorage, "store token", errors.New("db down")), codes.OK)

	assert.False(t, ok)
	assert.NoError(t, got)
}

func TestPublicAuthErrorRejectsNonAuthError(t *testing.T) {
	got, ok := publicAuthError(context.Background(), errors.New("boom"), codes.InvalidArgument)

	assert.False(t, ok)
	assert.NoError(t, got)
}
