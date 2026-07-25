package auth

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
)

// ContextKey represents a context key for authentication
type ContextKey string

const (
	// UserContextKey is the context key for the authenticated user
	UserContextKey ContextKey = "user"
	// ClaimsContextKey is the context key for JWT claims
	ClaimsContextKey ContextKey = "claims"
)

// LoadUserInfo returns the UserInfo for the user named by claims, as consumed
// by RequireUser/RequireAdmin. It is the only place that turns a validated
// token into a user record, so the gateway's authContextMiddleware and any
// future entry point populate UserContextKey identically.
//
// It returns an *AuthError: ErrInvalidToken for an unparseable user ID,
// ErrUserNotFound when no such row exists, and ErrUserDeleted for a
// soft-deleted account.
func (s *Service) LoadUserInfo(ctx context.Context, claims *Claims) (*UserInfo, error) {
	userID, err := pgutil.StringToUUID(claims.UserID)
	if err != nil {
		return nil, NewInvalidTokenError("invalid user ID", err)
	}

	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, NewUserNotFoundError()
	}

	if user.DeletedAt.Valid {
		return nil, NewAuthError(ErrUserDeleted, "user account has been deleted", nil)
	}

	return &UserInfo{
		ID:        pgutil.UUIDToString(user.ID),
		Email:     user.Email,
		Name:      user.Name,
		IsAdmin:   user.IsAdmin,
		CreatedAt: pgutil.TimestamptzToTime(user.CreatedAt),
		UpdatedAt: pgutil.TimestamptzToTime(user.UpdatedAt),
	}, nil
}

// GetUserFromStdContext extracts user information from standard context
func GetUserFromStdContext(ctx context.Context) (*UserInfo, bool) {
	userInterface := ctx.Value(UserContextKey)
	if userInterface == nil {
		return nil, false
	}

	user, ok := userInterface.(UserInfo)
	if !ok {
		return nil, false
	}

	return &user, true
}

// GetClaimsFromStdContext extracts JWT claims from standard context
func GetClaimsFromStdContext(ctx context.Context) (*Claims, bool) {
	claimsInterface := ctx.Value(ClaimsContextKey)
	if claimsInterface == nil {
		return nil, false
	}

	claims, ok := claimsInterface.(*Claims)
	if !ok {
		return nil, false
	}

	return claims, true
}

// WithUser adds user information to context
func WithUser(ctx context.Context, user UserInfo) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

// WithClaims adds JWT claims to context
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, ClaimsContextKey, claims)
}

// RequireUser ensures a user is present in the context
func RequireUser(ctx context.Context) (*UserInfo, error) {
	user, ok := GetUserFromStdContext(ctx)
	if !ok {
		return nil, NewUnauthorizedError("User authentication required")
	}
	return user, nil
}

// RequireAdmin ensures an admin user is present in the context
func RequireAdmin(ctx context.Context) (*UserInfo, error) {
	user, err := RequireUser(ctx)
	if err != nil {
		return nil, err
	}

	if !user.IsAdmin {
		return nil, NewInsufficientPermissionsError("Admin privileges required")
	}

	return user, nil
}
