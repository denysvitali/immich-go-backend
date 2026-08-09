package auth

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	userIDKey contextKey = "userID"
)

// BearerTokenFromGRPCMetadata extracts a Bearer token from gRPC incoming metadata.
// It returns the raw token string (without the "Bearer " prefix) or an error if
// metadata is missing, the authorization header is absent, or the format is invalid.
func BearerTokenFromGRPCMetadata(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}

	authHeader := authHeaders[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", status.Error(codes.Unauthenticated, "invalid authorization header format")
	}

	return strings.TrimPrefix(authHeader, "Bearer "), nil
}

// ClaimsFromContext extracts *auth.Claims from a gRPC context.
// It first checks for claims stored in the context by middleware (ClaimsContextKey),
// then falls back to parsing a Bearer token from gRPC metadata and validating it
// with the provided validator function.
func ClaimsFromContext(ctx context.Context, validator func(string) (*Claims, error)) (*Claims, error) {
	// Try context value first (set by auth middleware)
	if claims, ok := GetClaimsFromStdContext(ctx); ok && claims != nil {
		return claims, nil
	}

	// Fall back to validating Bearer token from metadata
	token, err := BearerTokenFromGRPCMetadata(ctx)
	if err != nil {
		return nil, err
	}

	claims, err := validator(token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	return claims, nil
}

// SessionIDFromGRPCMetadata extracts the x-session-id header from gRPC incoming metadata.
// It returns the session ID string or an empty string if the header is absent.
func SessionIDFromGRPCMetadata(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	sessionHeaders := md.Get("x-session-id")
	if len(sessionHeaders) > 0 {
		return sessionHeaders[0], nil
	}

	return "", nil
}

// GetUserIDFromContext extracts the user ID from the gRPC context
func GetUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	// Check if user ID is already in context (set by middleware)
	if userID, ok := ctx.Value(userIDKey).(uuid.UUID); ok {
		return userID, nil
	}

	// Try to get claims from standard context (set by auth middleware)
	claims, ok := GetClaimsFromStdContext(ctx)
	if ok && claims != nil {
		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			return uuid.UUID{}, status.Error(codes.Internal, "invalid user ID")
		}
		return userID, nil
	}

	// Try to extract from gRPC metadata
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return uuid.UUID{}, status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Try to get user ID from authorization header
	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return uuid.UUID{}, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	// Extract token from "Bearer <token>" format
	authHeader := authHeaders[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return uuid.UUID{}, status.Error(codes.Unauthenticated, "invalid authorization header format")
	}

	// Return error - authentication is required
	return uuid.UUID{}, status.Error(codes.Unauthenticated, "authentication required - JWT validation not performed")
}

// SetUserIDInContext sets the user ID in the context
func SetUserIDInContext(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

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
	if user.Status != "active" {
		return nil, NewAuthError(ErrUserDeleted, "user account is not active", nil)
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
