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
		if err == nil {
			return userID, nil
		}
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
