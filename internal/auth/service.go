package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/bcrypt"
)

var tracer = otel.Tracer("immich-go-backend/auth")

// Service provides authentication functionality
type Service struct {
	config  config.AuthConfig
	queries *sqlc.Queries
}

// NewService creates a new authentication service
func NewService(config config.AuthConfig, queries *sqlc.Queries) *Service {
	return &Service{
		config:  config,
		queries: queries,
	}
}

// Claims represents JWT claims
type Claims struct {
	UserID  string `json:"user_id"`
	Email   string `json:"email"`
	IsAdmin bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=1"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
	Name     string `json:"name" validate:"required,min=1"`
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         UserInfo  `json:"user"`
}

// UserInfo represents user information
type UserInfo struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RefreshRequest represents a token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ChangePasswordRequest represents a password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required"`
}

// Login authenticates a user and returns tokens
func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	ctx, span := tracer.Start(ctx, "auth.Login",
		trace.WithAttributes(attribute.String("auth.email", req.Email)))
	defer span.End()

	// Validate password complexity
	if err := s.validatePassword(req.Password); err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrInvalidCredentials,
			Message: "Invalid password format",
			Err:     err,
		}
	}

	// Get user by email
	user, err := s.queries.GetUserByEmail(ctx, req.Email)
	if err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrInvalidCredentials,
			Message: "Invalid email or password",
			Err:     err,
		}
	}

	// Check if user is deleted
	if user.DeletedAt.Valid {
		return nil, &AuthError{
			Type:    ErrUserDeleted,
			Message: "User account has been deleted",
		}
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrInvalidCredentials,
			Message: "Invalid email or password",
			Err:     err,
		}
	}

	// Generate tokens
	accessToken, refreshToken, expiresAt, err := s.generateTokens(uuidToString(user.ID), user.Email, user.IsAdmin)
	if err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrTokenGeneration,
			Message: "Failed to generate authentication tokens",
			Err:     err,
		}
	}

	// Store refresh token
	if err := s.storeRefreshToken(ctx, uuidToString(user.ID), refreshToken); err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrTokenStorage,
			Message: "Failed to store refresh token",
			Err:     err,
		}
	}

	// Update last login
	if err := s.queries.UpdateUserLastLogin(ctx, user.ID); err != nil {
		// Log error but don't fail the login
		span.RecordError(err)
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User: UserInfo{
			ID:        uuidToString(user.ID),
			Email:     user.Email,
			Name:      user.Name,
			IsAdmin:   user.IsAdmin,
			CreatedAt: timestamptzToTime(user.CreatedAt),
			UpdatedAt: timestamptzToTime(user.UpdatedAt),
		},
	}, nil
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	ctx, span := tracer.Start(ctx, "auth.Register",
		trace.WithAttributes(attribute.String("auth.email", req.Email)))
	defer span.End()

	// Check if registration is enabled
	if !s.config.RegistrationEnabled {
		return nil, &AuthError{
			Type:    ErrRegistrationDisabled,
			Message: "User registration is disabled",
		}
	}

	// Validate password
	if err := s.validatePassword(req.Password); err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrInvalidPassword,
			Message: "Password does not meet requirements",
			Err:     err,
		}
	}

	// Check if user already exists
	existingUser, err := s.queries.GetUserByEmail(ctx, req.Email)
	if err == nil && existingUser.ID.Valid {
		return nil, &AuthError{
			Type:    ErrUserExists,
			Message: "User with this email already exists",
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrPasswordHashing,
			Message: "Failed to hash password",
			Err:     err,
		}
	}

	// Generate user ID
	userID, err := s.generateUserID()
	if err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrUserCreation,
			Message: "Failed to generate user ID",
			Err:     err,
		}
	}

	// Create user
	userUUID, err := stringToUUID(userID)
	if err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrUserCreation,
			Message: "Failed to convert user ID",
			Err:     err,
		}
	}

	createUserParams := sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    req.Email,
		Name:     req.Name,
		Password: string(hashedPassword),
		IsAdmin:  false, // New users are not admin by default
	}

	user, err := s.queries.CreateUser(ctx, createUserParams)
	if err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrUserCreation,
			Message: "Failed to create user account",
			Err:     err,
		}
	}

	// Generate tokens
	accessToken, refreshToken, expiresAt, err := s.generateTokens(uuidToString(user.ID), user.Email, user.IsAdmin)
	if err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrTokenGeneration,
			Message: "Failed to generate authentication tokens",
			Err:     err,
		}
	}

	// Store refresh token
	if err := s.storeRefreshToken(ctx, uuidToString(user.ID), refreshToken); err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrTokenStorage,
			Message: "Failed to store refresh token",
			Err:     err,
		}
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User: UserInfo{
			ID:        uuidToString(user.ID),
			Email:     user.Email,
			Name:      user.Name,
			IsAdmin:   user.IsAdmin,
			CreatedAt: timestamptzToTime(user.CreatedAt),
			UpdatedAt: timestamptzToTime(user.UpdatedAt),
		},
	}, nil
}

// RefreshToken refreshes an access token using a refresh token
func (s *Service) RefreshToken(ctx context.Context, req RefreshRequest) (*AuthResponse, error) {
	ctx, span := tracer.Start(ctx, "auth.RefreshToken")
	defer span.End()

	// Validate refresh token
	claims, err := s.validateToken(req.RefreshToken)
	if err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrInvalidToken,
			Message: "Invalid refresh token",
			Err:     err,
		}
	}

	// Check if refresh token exists in database
	storedToken, err := s.queries.GetRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrInvalidToken,
			Message: "Refresh token not found",
			Err:     err,
		}
	}

	// Check if token is expired
	if timestamptzToTime(storedToken.ExpiresAt).Before(time.Now()) {
		return nil, &AuthError{
			Type:    ErrTokenExpired,
			Message: "Refresh token has expired",
		}
	}

	// Get user
	userUUID, err := stringToUUID(claims.UserID)
	if err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrInvalidToken,
			Message: "Invalid user ID in token",
			Err:     err,
		}
	}

	user, err := s.queries.GetUserByID(ctx, userUUID)
	if err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrUserNotFound,
			Message: "User not found",
			Err:     err,
		}
	}

	// Check if user is deleted
	if user.DeletedAt.Valid {
		return nil, &AuthError{
			Type:    ErrUserDeleted,
			Message: "User account has been deleted",
		}
	}

	// Generate new tokens
	accessToken, newRefreshToken, expiresAt, err := s.generateTokens(uuidToString(user.ID), user.Email, user.IsAdmin)
	if err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrTokenGeneration,
			Message: "Failed to generate new tokens",
			Err:     err,
		}
	}

	// Delete old refresh token and store new one
	if err := s.queries.DeleteRefreshToken(ctx, req.RefreshToken); err != nil {
		span.RecordError(err)
	}

	if err := s.storeRefreshToken(ctx, uuidToString(user.ID), newRefreshToken); err != nil {
		span.RecordError(err)
		return nil, &AuthError{
			Type:    ErrTokenStorage,
			Message: "Failed to store new refresh token",
			Err:     err,
		}
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    expiresAt,
		User: UserInfo{
			ID:        uuidToString(user.ID),
			Email:     user.Email,
			Name:      user.Name,
			IsAdmin:   user.IsAdmin,
			CreatedAt: timestamptzToTime(user.CreatedAt),
			UpdatedAt: timestamptzToTime(user.UpdatedAt),
		},
	}, nil
}

// ValidateToken validates a JWT token and returns claims
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	return s.validateToken(tokenString)
}

// Logout invalidates a refresh token
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	ctx, span := tracer.Start(ctx, "auth.Logout")
	defer span.End()

	if err := s.queries.DeleteRefreshToken(ctx, refreshToken); err != nil {
		span.RecordError(err)
		return &AuthError{
			Type:    ErrTokenDeletion,
			Message: "Failed to logout",
			Err:     err,
		}
	}

	return nil
}

// ChangePassword changes a user's password
func (s *Service) ChangePassword(ctx context.Context, userID string, req ChangePasswordRequest) error {
	ctx, span := tracer.Start(ctx, "auth.ChangePassword",
		trace.WithAttributes(attribute.String("auth.user_id", userID)))
	defer span.End()

	// Get user
	userUUID, err := stringToUUID(userID)
	if err != nil {
		span.RecordError(err)
		return &AuthError{
			Type:    ErrInvalidCredentials,
			Message: "Invalid user ID",
			Err:     err,
		}
	}

	user, err := s.queries.GetUserByID(ctx, userUUID)
	if err != nil {
		span.RecordError(err)
		return &AuthError{
			Type:    ErrUserNotFound,
			Message: "User not found",
			Err:     err,
		}
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		span.RecordError(err)
		return &AuthError{
			Type:    ErrInvalidCredentials,
			Message: "Current password is incorrect",
			Err:     err,
		}
	}

	// Validate new password
	if err := s.validatePassword(req.NewPassword); err != nil {
		span.RecordError(err)
		return &AuthError{
			Type:    ErrInvalidPassword,
			Message: "New password does not meet requirements",
			Err:     err,
		}
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		span.RecordError(err)
		return &AuthError{
			Type:    ErrPasswordHashing,
			Message: "Failed to hash new password",
			Err:     err,
		}
	}

	// Update password
	updateParams := sqlc.UpdateUserPasswordParams{
		ID:       userUUID,
		Password: string(hashedPassword),
	}

	if err := s.queries.UpdateUserPassword(ctx, updateParams); err != nil {
		span.RecordError(err)
		return &AuthError{
			Type:    ErrPasswordUpdate,
			Message: "Failed to update password",
			Err:     err,
		}
	}

	// Invalidate all refresh tokens for this user
	if err := s.queries.DeleteUserRefreshTokens(ctx, userUUID); err != nil {
		span.RecordError(err)
		// Log error but don't fail the password change
	}

	return nil
}

// GenerateToken generates an access token for OAuth authentication
func (s *Service) GenerateToken(userID, email string, duration time.Duration) (string, error) {
	now := time.Now()
	expiresAt := now.Add(duration)

	// Create access token claims
	accessClaims := &Claims{
		UserID:  userID,
		Email:   email,
		IsAdmin: false, // OAuth users are not admin by default
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.config.JWTIssuer,
			Subject:   userID,
		},
	}

	// Generate access token
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	return accessToken.SignedString([]byte(s.config.JWTSecret))
}

// generateTokens generates access and refresh tokens
func (s *Service) generateTokens(userID, email string, isAdmin bool) (string, string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.config.JWTExpiry)

	// Create access token claims
	accessClaims := &Claims{
		UserID:  userID,
		Email:   email,
		IsAdmin: isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.config.JWTIssuer,
			Subject:   userID,
		},
	}

	// Create refresh token claims
	refreshClaims := &Claims{
		UserID:  userID,
		Email:   email,
		IsAdmin: isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.config.JWTRefreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    s.config.JWTIssuer,
			Subject:   userID,
		},
	}

	// Generate access token
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", "", time.Time{}, err
	}

	// Generate refresh token
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", "", time.Time{}, err
	}

	return accessTokenString, refreshTokenString, expiresAt, nil
}

// validateToken validates a JWT token and returns claims
func (s *Service) validateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// storeRefreshToken stores a refresh token in the database
func (s *Service) storeRefreshToken(ctx context.Context, userID, token string) error {
	userUUID, err := stringToUUID(userID)
	if err != nil {
		return err
	}

	expiresAt, err := timeToTimestamptz(time.Now().Add(s.config.JWTRefreshExpiry))
	if err != nil {
		return err
	}

	params := sqlc.CreateRefreshTokenParams{
		Token:     token,
		UserId:    userUUID,
		ExpiresAt: expiresAt,
	}

	return s.queries.CreateRefreshToken(ctx, params)
}

// generateUserID generates a unique user ID
func (s *Service) generateUserID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// validatePassword validates password complexity requirements
func (s *Service) validatePassword(password string) error {
	if len(password) < s.config.PasswordMinLength {
		return fmt.Errorf("password must be at least %d characters long", s.config.PasswordMinLength)
	}

	if s.config.PasswordRequireUppercase {
		hasUpper := false
		for _, r := range password {
			if r >= 'A' && r <= 'Z' {
				hasUpper = true
				break
			}
		}
		if !hasUpper {
			return fmt.Errorf("password must contain at least one uppercase letter")
		}
	}

	if s.config.PasswordRequireLowercase {
		hasLower := false
		for _, r := range password {
			if r >= 'a' && r <= 'z' {
				hasLower = true
				break
			}
		}
		if !hasLower {
			return fmt.Errorf("password must contain at least one lowercase letter")
		}
	}

	if s.config.PasswordRequireNumbers {
		hasNumber := false
		for _, r := range password {
			if r >= '0' && r <= '9' {
				hasNumber = true
				break
			}
		}
		if !hasNumber {
			return fmt.Errorf("password must contain at least one number")
		}
	}

	if s.config.PasswordRequireSymbols {
		hasSymbol := false
		symbols := "!@#$%^&*()_+-=[]{}|;:,.<>?"
		for _, r := range password {
			for _, s := range symbols {
				if r == s {
					hasSymbol = true
					break
				}
			}
			if hasSymbol {
				break
			}
		}
		if !hasSymbol {
			return fmt.Errorf("password must contain at least one symbol")
		}
	}

	return nil
}
