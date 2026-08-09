package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type requestAuthenticationErrorKey struct{}

// httpAuthenticationHandler resolves credentials before a request reaches
// either grpc-gateway or one of the hand-written compatibility handlers. This
// keeps both transport paths on the same authentication policy and makes the
// resolved claims/user available through the request context.
func (s *Server) httpAuthenticationHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticatedRequest, err := s.authenticateHTTPRequest(r)
		if err != nil {
			ctx := context.WithValue(r.Context(), requestAuthenticationErrorKey{}, err)
			authenticatedRequest = r.WithContext(ctx)
		}
		next.ServeHTTP(w, authenticatedRequest)
	})
}

// authenticateHTTPRequest canonicalizes cookie authentication, validates all
// credentials supplied by the caller and enriches the request context. When a
// bearer token and API key are both supplied, they must identify the same user.
func (s *Server) authenticateHTTPRequest(r *http.Request) (*http.Request, error) {
	authHeader := requestAuthorization(r)
	if authHeader != "" && r.Header.Get("Authorization") == "" {
		r = requestWithAuthorization(r, authHeader)
	}
	if claims, hasClaims := auth.GetClaimsFromStdContext(r.Context()); hasClaims && claims != nil {
		if _, hasUser := auth.GetUserFromStdContext(r.Context()); hasUser {
			return r, nil
		}
	}

	apiKeyHeader := strings.TrimSpace(r.Header.Get("x-api-key"))
	if authHeader == "" && apiKeyHeader == "" {
		return r, nil
	}

	var bearerClaims *auth.Claims
	if authHeader != "" {
		if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")) == "" || s.authService == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid bearer token")
		}
		claims, err := s.authService.ValidateToken(strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")))
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid bearer token")
		}
		bearerClaims = claims
	}

	var apiKeyClaims *auth.Claims
	if apiKeyHeader != "" {
		if s.apiKeyService == nil {
			return nil, status.Error(codes.Unauthenticated, "invalid API key")
		}
		rawKey := strings.TrimSpace(strings.TrimPrefix(apiKeyHeader, "immich_"))
		if rawKey == "" {
			return nil, status.Error(codes.Unauthenticated, "invalid API key")
		}
		apiKey, err := s.apiKeyService.ValidateAPIKey(r.Context(), rawKey)
		if err != nil || !apiKey.UserId.Valid {
			return nil, status.Error(codes.Unauthenticated, "invalid API key")
		}
		apiKeyClaims = &auth.Claims{UserID: uuid.UUID(apiKey.UserId.Bytes).String()}
	}

	if bearerClaims != nil && apiKeyClaims != nil {
		bearerUserID, err := uuid.Parse(bearerClaims.UserID)
		if err != nil || bearerUserID.String() != apiKeyClaims.UserID {
			return nil, status.Error(codes.PermissionDenied, "credentials identify different users")
		}
	}

	claims := bearerClaims
	if claims == nil {
		claims = apiKeyClaims
	}
	return s.enrichHTTPRequest(r, claims)
}

func (s *Server) enrichHTTPRequest(r *http.Request, claims *auth.Claims) (*http.Request, error) {
	if claims == nil || s.authService == nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}

	userInfo, err := s.authService.LoadUserInfo(r.Context(), claims)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "authenticated user is unavailable")
	}
	claims.Email = userInfo.Email
	claims.IsAdmin = userInfo.IsAdmin
	ctx := auth.WithClaims(r.Context(), claims)
	ctx = auth.WithUser(ctx, *userInfo)
	return r.WithContext(ctx), nil
}

// authenticatedHTTPRequestContext is used by grpc-gateway's middleware when
// it is invoked without the full HTTPHandler wrapper (primarily focused tests).
// Invalid credentials are left for the endpoint's normal authorization path.
func (s *Server) authenticatedHTTPRequestContext(r *http.Request) context.Context {
	authenticatedRequest, err := s.authenticateHTTPRequest(r)
	if err != nil {
		return context.WithValue(r.Context(), requestAuthenticationErrorKey{}, err)
	}
	return authenticatedRequest.Context()
}

func requestAuthenticationError(ctx context.Context) error {
	err, _ := ctx.Value(requestAuthenticationErrorKey{}).(error)
	return err
}
