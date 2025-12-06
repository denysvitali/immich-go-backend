package server

import (
	"context"
	"strings"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// getUserIDFromContext extracts the user ID from the gRPC context
func (s *Server) getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
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

	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Validate token and get user info
	userInfo, err := s.authService.ValidateToken(token)
	if err != nil {
		return uuid.UUID{}, status.Error(codes.Unauthenticated, "invalid token")
	}

	// Parse user ID string to UUID
	userID, err := uuid.Parse(userInfo.ID)
	if err != nil {
		return uuid.UUID{}, status.Error(codes.Internal, "invalid user ID format")
	}

	return userID, nil
}

// getSessionIDFromContext extracts the session ID from the gRPC context
func (s *Server) getSessionIDFromContext(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Try to get session ID from x-session-id header
	sessionHeaders := md.Get("x-session-id")
	if len(sessionHeaders) > 0 {
		return sessionHeaders[0], nil
	}

	// Otherwise, extract from bearer token if it's a session token
	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}

	authHeader := authHeaders[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", status.Error(codes.Unauthenticated, "invalid authorization header format")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Try to get session from the token (for session-based auth)
	session, err := s.sessionsService.GetSessionByToken(ctx, token)
	if err == nil && session != nil {
		return session.ID, nil
	}

	return "", nil
}

// getUserFromContext extracts the user claims from the gRPC context
func (s *Server) getUserFromContext(ctx context.Context) (*auth.Claims, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Try to get user ID from authorization header
	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	// Extract token from "Bearer <token>" format
	authHeader := authHeaders[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization header format")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Validate token and get claims
	claims, err := s.authService.ValidateToken(token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	return claims, nil
}
