package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel/attribute"
)

// ContextKey represents a context key for authentication
type ContextKey string

const (
	// UserContextKey is the context key for the authenticated user
	UserContextKey ContextKey = "user"
	// ClaimsContextKey is the context key for JWT claims
	ClaimsContextKey ContextKey = "claims"
)

// Helper functions for type conversion
func stringToUUID(s string) (pgtype.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

func timestamptzToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func timeToTimestamptz(t time.Time) (pgtype.Timestamptz, error) {
	return pgtype.Timestamptz{
		Time:  t,
		Valid: true,
	}, nil
}

// AuthMiddleware creates a middleware for JWT authentication
func (s *Service) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), "auth.middleware")
		defer span.End()

		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			span.SetAttributes(attribute.String("auth.error", "missing_authorization_header"))
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header is required",
				"type":  string(ErrUnauthorized),
			})
			c.Abort()
			return
		}

		// Check if it's a Bearer token
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			span.SetAttributes(attribute.String("auth.error", "invalid_authorization_format"))
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format",
				"type":  string(ErrUnauthorized),
			})
			c.Abort()
			return
		}

		token := tokenParts[1]

		// Validate token
		claims, err := s.ValidateToken(token)
		if err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("auth.error", "invalid_token"))
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
				"type":  string(ErrInvalidToken),
			})
			c.Abort()
			return
		}

		// Get user from database
		userID, err := stringToUUID(claims.UserID)
		if err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("auth.error", "invalid_user_id"))
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid user ID",
				"type":  string(ErrInvalidToken),
			})
			c.Abort()
			return
		}
		
		user, err := s.queries.GetUserByID(ctx, userID)
		if err != nil {
			span.RecordError(err)
			span.SetAttributes(attribute.String("auth.error", "user_not_found"))
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User not found",
				"type":  string(ErrUserNotFound),
			})
			c.Abort()
			return
		}

		// Check if user is deleted
		if user.DeletedAt.Valid {
			span.SetAttributes(attribute.String("auth.error", "user_deleted"))
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User account has been deleted",
				"type":  string(ErrUserDeleted),
			})
			c.Abort()
			return
		}

		// Add user and claims to context
		userInfo := UserInfo{
			ID:        uuidToString(user.ID),
			Email:     user.Email,
			Name:      user.Name,
			IsAdmin:   user.IsAdmin,
			CreatedAt: timestamptzToTime(user.CreatedAt),
			UpdatedAt: timestamptzToTime(user.UpdatedAt),
		}

		c.Set(string(UserContextKey), userInfo)
		c.Set(string(ClaimsContextKey), claims)

		// Add user info to span
		span.SetAttributes(
			attribute.String("auth.user_id", uuidToString(user.ID)),
			attribute.String("auth.user_email", user.Email),
			attribute.Bool("auth.is_admin", user.IsAdmin),
		)

		c.Next()
	}
}

// AdminMiddleware creates a middleware that requires admin privileges
func (s *Service) AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, span := tracer.Start(c.Request.Context(), "auth.admin_middleware")
		defer span.End()

		// Get user from context (should be set by AuthMiddleware)
		userInterface, exists := c.Get(string(UserContextKey))
		if !exists {
			span.SetAttributes(attribute.String("auth.error", "user_not_in_context"))
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
				"type":  string(ErrUnauthorized),
			})
			c.Abort()
			return
		}

		user, ok := userInterface.(UserInfo)
		if !ok {
			span.SetAttributes(attribute.String("auth.error", "invalid_user_context"))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user context",
			})
			c.Abort()
			return
		}

		// Check if user is admin
		if !user.IsAdmin {
			span.SetAttributes(attribute.String("auth.error", "insufficient_permissions"))
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Admin privileges required",
				"type":  string(ErrInsufficientPermissions),
			})
			c.Abort()
			return
		}

		span.SetAttributes(attribute.Bool("auth.admin_access", true))
		c.Next()
	}
}

// OptionalAuthMiddleware creates a middleware for optional authentication
// If a valid token is provided, user info is added to context
// If no token or invalid token, request continues without user info
func (s *Service) OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), "auth.optional_middleware")
		defer span.End()

		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			span.SetAttributes(attribute.String("auth.status", "no_token"))
			c.Next()
			return
		}

		// Check if it's a Bearer token
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			span.SetAttributes(attribute.String("auth.status", "invalid_format"))
			c.Next()
			return
		}

		token := tokenParts[1]

		// Validate token
		claims, err := s.ValidateToken(token)
		if err != nil {
			span.SetAttributes(attribute.String("auth.status", "invalid_token"))
			c.Next()
			return
		}

		// Get user from database
		userID, err := stringToUUID(claims.UserID)
		if err != nil {
			span.SetAttributes(attribute.String("auth.status", "invalid_user_id"))
			c.Next()
			return
		}
		
		user, err := s.queries.GetUserByID(ctx, userID)
		if err != nil {
			span.SetAttributes(attribute.String("auth.status", "user_not_found"))
			c.Next()
			return
		}

		// Check if user is deleted
		if user.DeletedAt.Valid {
			span.SetAttributes(attribute.String("auth.status", "user_deleted"))
			c.Next()
			return
		}

		// Add user and claims to context
		userInfo := UserInfo{
			ID:        uuidToString(user.ID),
			Email:     user.Email,
			Name:      user.Name,
			IsAdmin:   user.IsAdmin,
			CreatedAt: timestamptzToTime(user.CreatedAt),
			UpdatedAt: timestamptzToTime(user.UpdatedAt),
		}

		c.Set(string(UserContextKey), userInfo)
		c.Set(string(ClaimsContextKey), claims)

		span.SetAttributes(
			attribute.String("auth.status", "authenticated"),
			attribute.String("auth.user_id", uuidToString(user.ID)),
			attribute.Bool("auth.is_admin", user.IsAdmin),
		)

		c.Next()
	}
}

// GetUserFromContext extracts user information from Gin context
func GetUserFromContext(c *gin.Context) (*UserInfo, bool) {
	userInterface, exists := c.Get(string(UserContextKey))
	if !exists {
		return nil, false
	}

	user, ok := userInterface.(UserInfo)
	if !ok {
		return nil, false
	}

	return &user, true
}

// GetClaimsFromContext extracts JWT claims from Gin context
func GetClaimsFromContext(c *gin.Context) (*Claims, bool) {
	claimsInterface, exists := c.Get(string(ClaimsContextKey))
	if !exists {
		return nil, false
	}

	claims, ok := claimsInterface.(*Claims)
	if !ok {
		return nil, false
	}

	return claims, true
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