package server

import (
	"context"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// getUserIDFromContext extracts the user ID from the gRPC context by first
// checking for claims set by middleware, then falling back to validating the
// Bearer token from metadata.
func (s *Server) getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	claims, err := auth.ClaimsFromContext(ctx, s.authService.ValidateToken)
	if err != nil {
		return uuid.UUID{}, err
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return uuid.UUID{}, status.Error(codes.Internal, "invalid user ID format")
	}

	return userID, nil
}

// getSessionIDFromContext extracts the session ID from the gRPC context.
// It first checks the x-session-id header, then falls back to looking up the
// session by the Bearer token.
func (s *Server) getSessionIDFromContext(ctx context.Context) (string, error) {
	// Try to get session ID from x-session-id header first
	if sessionID, err := auth.SessionIDFromGRPCMetadata(ctx); err == nil && sessionID != "" {
		return sessionID, nil
	}

	// Otherwise, extract from bearer token if it's a session token
	token, err := auth.BearerTokenFromGRPCMetadata(ctx)
	if err != nil {
		return "", err
	}

	// Try to get session from the token (for session-based auth)
	session, err := s.sessionsService.GetSessionByToken(ctx, token)
	if err == nil && session != nil {
		return session.ID, nil
	}

	return "", nil
}

// getUserFromContext extracts the user claims from the gRPC context.
// It is a thin wrapper around auth.ClaimsFromContext.
func (s *Server) getUserFromContext(ctx context.Context) (*auth.Claims, error) {
	return auth.ClaimsFromContext(ctx, s.authService.ValidateToken)
}

func (s *Server) requireAdmin(ctx context.Context) (*auth.Claims, error) {
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	if !claims.IsAdmin {
		return nil, status.Error(codes.PermissionDenied, "admin access required")
	}
	return claims, nil
}
