package auth

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestAsAuthErrorFindsWrappedAuthError(t *testing.T) {
	authErr := NewAuthError(ErrInvalidToken, "invalid token", errors.New("bad signature"))
	err := fmt.Errorf("middleware failed: %w", authErr)

	got, ok := AsAuthError(err)

	require.True(t, ok)
	assert.Same(t, authErr, got)
	assert.True(t, IsAuthError(err))
	assert.Equal(t, ErrInvalidToken, GetAuthErrorType(err))
}

func TestMapAuthErrorToGRPC(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{
			name: "invalid credentials",
			err:  NewAuthError(ErrInvalidCredentials, "bad credentials", nil),
			want: codes.Unauthenticated,
		},
		{
			name: "invalid password",
			err:  NewAuthError(ErrInvalidPassword, "bad password", nil),
			want: codes.InvalidArgument,
		},
		{
			name: "rate limited",
			err:  NewAuthError(ErrRateLimited, "too many attempts", nil),
			want: codes.ResourceExhausted,
		},
		{
			name: "user exists",
			err:  NewAuthError(ErrUserExists, "already exists", nil),
			want: codes.AlreadyExists,
		},
		{
			name: "pin code exists",
			err:  NewAuthError(ErrPinCodeExists, "pin exists", nil),
			want: codes.FailedPrecondition,
		},
		{
			name: "operational auth error",
			err:  NewAuthError(ErrTokenStorage, "store token", nil),
			want: codes.Internal,
		},
		{
			name: "wrapped auth error",
			err:  fmt.Errorf("wrapped: %w", NewAuthError(ErrUnauthorized, "unauthorized", nil)),
			want: codes.Unauthenticated,
		},
		{
			name: "non auth error",
			err:  errors.New("boom"),
			want: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, MapAuthErrorToGRPC(tt.err))
		})
	}
}
