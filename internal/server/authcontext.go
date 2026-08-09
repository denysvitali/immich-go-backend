package server

import (
	"context"
	"strings"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// getUserIDFromContext extracts the user ID from the gRPC context by first
// checking for claims set by middleware, then falling back to validating the
// Bearer token from metadata.
func (s *Server) getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	claims, err := s.getUserFromContext(ctx)
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

// getUserFromContext extracts the authenticated user claims from the gRPC
// context. API keys are validated here as well as in the HTTP gateway so the
// RPCs implemented directly by Server accept either credential type.
func (s *Server) getUserFromContext(ctx context.Context) (*auth.Claims, error) {
	if err := requestAuthenticationError(ctx); err != nil {
		return nil, err
	}
	// Claims already stored in the context were resolved by the HTTP wrapper or
	// gRPC interceptor (and are also used by focused in-process tests).
	if claims, ok := auth.GetClaimsFromStdContext(ctx); ok && claims != nil {
		return claims, nil
	}
	if claims, err := auth.ClaimsFromContext(ctx, s.authService.ValidateToken); err == nil {
		// A directly exposed gRPC listener can be reached without the HTTP
		// wrapper. Re-read the user here so deleted/removing accounts and stale
		// administrator claims cannot continue authenticating until JWT expiry.
		userInfo, loadErr := s.authService.LoadUserInfo(ctx, claims)
		if loadErr != nil {
			return nil, status.Error(codes.Unauthenticated, "authenticated user is unavailable")
		}
		claims.Email = userInfo.Email
		claims.IsAdmin = userInfo.IsAdmin
		return claims, nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	values := md.Get("x-api-key")
	if len(values) == 0 || values[0] == "" {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	if s.apiKeyService == nil {
		return nil, status.Error(codes.Unauthenticated, "invalid API key")
	}

	apiKey, err := s.apiKeyService.ValidateAPIKey(ctx, strings.TrimPrefix(values[0], "immich_"))
	if err != nil || !apiKey.UserId.Valid {
		return nil, status.Error(codes.Unauthenticated, "invalid API key")
	}

	claims := &auth.Claims{UserID: uuid.UUID(apiKey.UserId.Bytes).String()}
	userInfo, err := s.authService.LoadUserInfo(ctx, claims)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid API key")
	}
	claims.Email = userInfo.Email
	claims.IsAdmin = userInfo.IsAdmin
	return claims, nil
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

func (s *Server) authenticatedContext(ctx context.Context) context.Context {
	claims, err := s.getUserFromContext(ctx)
	if err != nil {
		return ctx
	}
	return s.contextWithAuthenticatedUser(ctx, claims)
}

func (s *Server) authenticationUnaryInterceptor(
	ctx context.Context,
	req any,
	_ *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	return handler(s.authenticatedContext(ctx), req)
}

func (s *Server) authenticationStreamInterceptor(
	srv any,
	stream grpc.ServerStream,
	_ *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	return handler(srv, &authenticatedServerStream{
		ServerStream: stream,
		ctx:          s.authenticatedContext(stream.Context()),
	})
}

type authenticatedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authenticatedServerStream) Context() context.Context {
	return s.ctx
}
