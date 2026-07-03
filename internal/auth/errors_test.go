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

func TestAuthErrorConstructors(t *testing.T) {
	cause := errors.New("bad signature")
	tests := []struct {
		name string
		got  *AuthError
		want AuthError
	}{
		{
			name: "invalid credentials",
			got:  NewInvalidCredentialsError("bad credentials"),
			want: AuthError{Type: ErrInvalidCredentials, Message: "bad credentials"},
		},
		{
			name: "invalid token",
			got:  NewInvalidTokenError("invalid token", cause),
			want: AuthError{Type: ErrInvalidToken, Message: "invalid token", Err: cause},
		},
		{
			name: "token expired",
			got:  NewTokenExpiredError(),
			want: AuthError{Type: ErrTokenExpired, Message: "Token has expired"},
		},
		{
			name: "user not found",
			got:  NewUserNotFoundError(),
			want: AuthError{Type: ErrUserNotFound, Message: "User not found"},
		},
		{
			name: "user exists",
			got:  NewUserExistsError(),
			want: AuthError{Type: ErrUserExists, Message: "User already exists"},
		},
		{
			name: "registration disabled",
			got:  NewRegistrationDisabledError(),
			want: AuthError{Type: ErrRegistrationDisabled, Message: "User registration is disabled"},
		},
		{
			name: "insufficient permissions",
			got:  NewInsufficientPermissionsError("admin required"),
			want: AuthError{Type: ErrInsufficientPermissions, Message: "admin required"},
		},
		{
			name: "unauthorized",
			got:  NewUnauthorizedError("not authenticated"),
			want: AuthError{Type: ErrUnauthorized, Message: "not authenticated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.got)
			assert.Equal(t, tt.want.Type, tt.got.Type)
			assert.Equal(t, tt.want.Message, tt.got.Message)
			assert.ErrorIs(t, tt.got.Err, tt.want.Err)
		})
	}
}
