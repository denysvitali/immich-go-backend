package auth

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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
