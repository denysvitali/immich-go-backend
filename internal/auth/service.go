package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/bcrypt"
)

var tracer = otel.Tracer("immich-go-backend/auth")

func recordedAuthError(span trace.Span, errorType AuthErrorType, message string, err error) *AuthError {
	if err != nil {
		span.RecordError(err)
	}
	return NewAuthError(errorType, message, err)
}

// Service provides authentication functionality
type Service struct {
	config       config.AuthConfig
	queries      *sqlc.Queries
	loginLimiter *loginRateLimiter
}

// NewService creates a new authentication service
func NewService(config config.AuthConfig, queries *sqlc.Queries) *Service {
	return &Service{
		config:       config,
		queries:      queries,
		loginLimiter: newLoginRateLimiter(config.LoginRateLimit, config.LoginRateWindow),
	}
}

// Claims represents JWT claims
type Claims struct {
	UserID   string   `json:"user_id"`
	Email    string   `json:"email"`
	IsAdmin  bool     `json:"is_admin"`
	TokenUse TokenUse `json:"token_use,omitempty"`
	jwt.RegisteredClaims
}

// TokenUse identifies which authentication flow may consume a JWT. Access
// and refresh tokens otherwise use the same key and claims, so validators must
// not infer their purpose from the signature or lifetime.
type TokenUse string

const (
	TokenUseAccess  TokenUse = "access"
	TokenUseRefresh TokenUse = "refresh"
)

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
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	IsAdmin     bool      `json:"is_admin"`
	IsOnboarded bool      `json:"is_onboarded"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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

	loginKey := loginRateLimitKey(req.Email)
	if !s.allowLoginAttempt(loginKey) {
		return nil, NewAuthError(ErrRateLimited, "Too many failed login attempts", nil)
	}

	// Validate password complexity
	if err := s.validatePassword(req.Password); err != nil {
		s.recordFailedLogin(loginKey)
		return nil, recordedAuthError(span, ErrInvalidCredentials, "Invalid password format", err)
	}

	// Get user by email
	user, err := s.queries.GetUserByEmail(ctx, req.Email)
	if err != nil {
		s.recordFailedLogin(loginKey)
		return nil, recordedAuthError(span, ErrInvalidCredentials, "Invalid email or password", err)
	}

	// Check if user is deleted
	if user.DeletedAt.Valid {
		s.recordFailedLogin(loginKey)
		return nil, NewAuthError(ErrUserDeleted, "User account has been deleted", nil)
	}
	if user.Status != "active" {
		s.recordFailedLogin(loginKey)
		return nil, NewAuthError(ErrUserDeleted, "User account is not active", nil)
	}

	// Verify password
	if passwordErr := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); passwordErr != nil {
		s.recordFailedLogin(loginKey)
		return nil, recordedAuthError(span, ErrInvalidCredentials, "Invalid email or password", passwordErr)
	}

	s.resetLoginAttempts(loginKey)

	// Generate tokens
	accessToken, refreshToken, expiresAt, err := s.generateTokens(pgutil.UUIDToString(user.ID), user.Email, user.IsAdmin)
	if err != nil {
		return nil, recordedAuthError(span, ErrTokenGeneration, "Failed to generate authentication tokens", err)
	}

	// Store refresh token
	if err := s.storeRefreshToken(ctx, pgutil.UUIDToString(user.ID), refreshToken); err != nil {
		return nil, recordedAuthError(span, ErrTokenStorage, "Failed to store refresh token", err)
	}

	// Update last login
	if err := s.queries.UpdateUserLastLogin(ctx, user.ID); err != nil {
		// Log error but don't fail the login
		span.RecordError(err)
	}

	// On the user's first successful login, mark them as onboarded
	isOnboarded := user.IsOnboarded
	if !isOnboarded {
		if err := s.queries.SetUserOnboarded(ctx, sqlc.SetUserOnboardedParams{
			ID:          user.ID,
			IsOnboarded: true,
		}); err != nil {
			span.RecordError(err)
		} else {
			isOnboarded = true
		}
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User: UserInfo{
			ID:          pgutil.UUIDToString(user.ID),
			Email:       user.Email,
			Name:        user.Name,
			IsAdmin:     user.IsAdmin,
			IsOnboarded: isOnboarded,
			CreatedAt:   pgutil.TimestamptzToTime(user.CreatedAt),
			UpdatedAt:   pgutil.TimestamptzToTime(user.UpdatedAt),
		},
	}, nil
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	return s.register(ctx, req, false)
}

// AdminSignUp creates the initial administrator account during setup.
func (s *Service) AdminSignUp(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	initialized, err := s.IsInitialized(ctx)
	if err != nil {
		return nil, err
	}
	if initialized {
		return nil, NewAuthError(ErrAlreadyInitialized, "System setup has already been completed", nil)
	}
	return s.register(ctx, req, true)
}

// IsInitialized reports whether the one-time setup window has been closed.
// The durable marker remains true even if every user is later deleted.
func (s *Service) IsInitialized(ctx context.Context) (bool, error) {
	initialized, err := s.queries.IsSystemInitialized(ctx)
	if err != nil {
		return false, NewAuthError(ErrUserCreation, "Failed to check system initialization", err)
	}
	return initialized.Valid && initialized.Bool, nil
}

type registeredUser struct {
	ID          string
	Email       string
	Name        string
	IsAdmin     bool
	IsOnboarded bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (s *Service) register(ctx context.Context, req RegisterRequest, initialAdmin bool) (*AuthResponse, error) {
	ctx, span := tracer.Start(ctx, "auth.Register",
		trace.WithAttributes(attribute.String("auth.email", req.Email)))
	defer span.End()

	// Check if registration is enabled
	if !s.config.RegistrationEnabled {
		return nil, NewAuthError(ErrRegistrationDisabled, "User registration is disabled", nil)
	}

	// Validate password
	if err := s.validatePassword(req.Password); err != nil {
		return nil, recordedAuthError(span, ErrInvalidPassword, "Password does not meet requirements", err)
	}

	// Preserve the ordinary registration duplicate response. Initial admin
	// registration deliberately goes through the atomic database claim first,
	// so every post-initialization call has the same failed-precondition result.
	if !initialAdmin {
		existingUser, err := s.queries.GetUserByEmail(ctx, req.Email)
		if err == nil && existingUser.ID.Valid {
			return nil, NewAuthError(ErrUserExists, "User with this email already exists", nil)
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, recordedAuthError(span, ErrPasswordHashing, "Failed to hash password", err)
	}

	// Generate user ID
	userID, err := s.generateUserID()
	if err != nil {
		return nil, recordedAuthError(span, ErrUserCreation, "Failed to generate user ID", err)
	}

	// Create user
	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return nil, recordedAuthError(span, ErrUserCreation, "Failed to convert user ID", err)
	}

	// Generate tokens before the database statement. The refresh session is
	// inserted alongside the user and bootstrap marker, so none can commit if
	// any database write fails.
	accessToken, refreshToken, expiresAt, err := s.generateTokens(userID, req.Email, initialAdmin)
	if err != nil {
		return nil, recordedAuthError(span, ErrTokenGeneration, "Failed to generate authentication tokens", err)
	}

	refreshExpiresAt := pgutil.TimeToTimestamptz(time.Now().Add(s.config.JWTRefreshExpiry))
	var user registeredUser
	if initialAdmin {
		created, createErr := s.queries.CreateInitialAdmin(ctx, sqlc.CreateInitialAdminParams{
			ID:               userUUID,
			Email:            req.Email,
			Name:             req.Name,
			Password:         string(hashedPassword),
			RefreshToken:     refreshToken,
			RefreshExpiresAt: refreshExpiresAt,
		})
		if errors.Is(createErr, pgx.ErrNoRows) {
			return nil, NewAuthError(ErrAlreadyInitialized, "System setup has already been completed", nil)
		}
		if createErr != nil {
			return nil, recordedAuthError(span, ErrUserCreation, "Failed to create initial administrator", createErr)
		}
		user = registeredUser{
			ID:          pgutil.UUIDToString(created.ID),
			Email:       created.Email,
			Name:        created.Name,
			IsAdmin:     created.IsAdmin,
			IsOnboarded: created.IsOnboarded,
			CreatedAt:   pgutil.TimestamptzToTime(created.CreatedAt),
			UpdatedAt:   pgutil.TimestamptzToTime(created.UpdatedAt),
		}
	} else {
		created, createErr := s.queries.CreateRegisteredUser(ctx, sqlc.CreateRegisteredUserParams{
			ID:               userUUID,
			Email:            req.Email,
			Name:             req.Name,
			Password:         string(hashedPassword),
			RefreshToken:     refreshToken,
			RefreshExpiresAt: refreshExpiresAt,
		})
		if createErr != nil {
			return nil, recordedAuthError(span, ErrUserCreation, "Failed to create user account", createErr)
		}
		user = registeredUser{
			ID:          pgutil.UUIDToString(created.ID),
			Email:       created.Email,
			Name:        created.Name,
			IsAdmin:     created.IsAdmin,
			IsOnboarded: created.IsOnboarded,
			CreatedAt:   pgutil.TimestamptzToTime(created.CreatedAt),
			UpdatedAt:   pgutil.TimestamptzToTime(created.UpdatedAt),
		}
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         UserInfo(user),
	}, nil
}

// RefreshToken refreshes an access token using a refresh token
func (s *Service) RefreshToken(ctx context.Context, req RefreshRequest) (*AuthResponse, error) {
	ctx, span := tracer.Start(ctx, "auth.RefreshToken")
	defer span.End()

	// Validate the token's explicit purpose before consulting storage. Legacy
	// untyped tokens fail closed: refresh tokens and access/session tokens share
	// the same sessions table, so storage membership cannot safely distinguish
	// their intended use.
	claims, err := s.validateRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, recordedAuthError(span, ErrInvalidToken, "Invalid refresh token", err)
	}

	// Check if refresh token exists in database
	storedToken, err := s.queries.GetRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, recordedAuthError(span, ErrInvalidToken, "Refresh token not found", err)
	}

	// Check if token is expired
	if pgutil.TimestamptzToTime(storedToken.ExpiresAt).Before(time.Now()) {
		return nil, NewAuthError(ErrTokenExpired, "Refresh token has expired", nil)
	}

	// Get user
	userUUID, err := pgutil.StringToUUID(claims.UserID)
	if err != nil {
		return nil, recordedAuthError(span, ErrInvalidToken, "Invalid user ID in token", err)
	}

	user, err := s.queries.GetUserByID(ctx, userUUID)
	if err != nil {
		return nil, recordedAuthError(span, ErrUserNotFound, "User not found", err)
	}

	// Check if user is deleted
	if user.DeletedAt.Valid {
		return nil, NewAuthError(ErrUserDeleted, "User account has been deleted", nil)
	}
	if user.Status != "active" {
		return nil, NewAuthError(ErrUserDeleted, "User account is not active", nil)
	}

	// Generate new tokens
	accessToken, newRefreshToken, expiresAt, err := s.generateTokens(pgutil.UUIDToString(user.ID), user.Email, user.IsAdmin)
	if err != nil {
		return nil, recordedAuthError(span, ErrTokenGeneration, "Failed to generate new tokens", err)
	}

	// Delete old refresh token and store new one
	if err := s.queries.DeleteRefreshToken(ctx, req.RefreshToken); err != nil {
		span.RecordError(err)
	}

	if err := s.storeRefreshToken(ctx, pgutil.UUIDToString(user.ID), newRefreshToken); err != nil {
		return nil, recordedAuthError(span, ErrTokenStorage, "Failed to store new refresh token", err)
	}

	return &AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    expiresAt,
		User: UserInfo{
			ID:          pgutil.UUIDToString(user.ID),
			Email:       user.Email,
			Name:        user.Name,
			IsAdmin:     user.IsAdmin,
			IsOnboarded: user.IsOnboarded,
			CreatedAt:   pgutil.TimestamptzToTime(user.CreatedAt),
			UpdatedAt:   pgutil.TimestamptzToTime(user.UpdatedAt),
		},
	}, nil
}

// ValidateToken validates a JWT token and returns claims
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	claims, err := s.parseToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenUse != TokenUseAccess {
		return nil, fmt.Errorf("invalid token use: access token required")
	}
	return claims, nil
}

// Logout invalidates every refresh session for a user. The current logout API
// identifies the authenticated user but does not submit a refresh token, so
// deleting all of that user's sessions is the only complete revocation.
func (s *Service) Logout(ctx context.Context, userID string) error {
	ctx, span := tracer.Start(ctx, "auth.Logout")
	defer span.End()

	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return recordedAuthError(span, ErrInvalidCredentials, "Invalid user ID", err)
	}
	if err := s.queries.DeleteUserRefreshTokens(ctx, userUUID); err != nil {
		return recordedAuthError(span, ErrTokenDeletion, "Failed to logout", err)
	}

	return nil
}

// ChangePassword changes a user's password
func (s *Service) ChangePassword(ctx context.Context, userID string, req ChangePasswordRequest) error {
	ctx, span := tracer.Start(ctx, "auth.ChangePassword",
		trace.WithAttributes(attribute.String("auth.user_id", userID)))
	defer span.End()

	// Get user
	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return recordedAuthError(span, ErrInvalidCredentials, "Invalid user ID", err)
	}

	user, err := s.queries.GetUserByID(ctx, userUUID)
	if err != nil {
		return recordedAuthError(span, ErrUserNotFound, "User not found", err)
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		return recordedAuthError(span, ErrInvalidCredentials, "Current password is incorrect", err)
	}

	// Validate new password
	if err := s.validatePassword(req.NewPassword); err != nil {
		return recordedAuthError(span, ErrInvalidPassword, "New password does not meet requirements", err)
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return recordedAuthError(span, ErrPasswordHashing, "Failed to hash new password", err)
	}

	// Update password
	updateParams := sqlc.UpdateUserPasswordParams{
		ID:       userUUID,
		Password: string(hashedPassword),
	}

	if err := s.queries.UpdateUserPassword(ctx, updateParams); err != nil {
		return recordedAuthError(span, ErrPasswordUpdate, "Failed to update password", err)
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
		UserID:   userID,
		Email:    email,
		IsAdmin:  false, // OAuth users are not admin by default
		TokenUse: TokenUseAccess,
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
		UserID:   userID,
		Email:    email,
		IsAdmin:  isAdmin,
		TokenUse: TokenUseAccess,
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
		UserID:   userID,
		Email:    email,
		IsAdmin:  isAdmin,
		TokenUse: TokenUseRefresh,
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

// validateRefreshToken validates a JWT intended for the refresh
// flow. Both access tokens and untyped legacy tokens fail closed.
func (s *Service) validateRefreshToken(tokenString string) (*Claims, error) {
	claims, err := s.parseToken(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenUse != TokenUseRefresh {
		return nil, fmt.Errorf("invalid token use: refresh token required")
	}
	return claims, nil
}

// parseToken verifies a JWT signature and registered claims without assigning
// it an authentication purpose. Callers must enforce token_use for their flow.
func (s *Service) parseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
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
	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return err
	}

	params := sqlc.CreateRefreshTokenParams{
		Token:     token,
		UserId:    userUUID,
		ExpiresAt: pgutil.TimeToTimestamptz(time.Now().Add(s.config.JWTRefreshExpiry)),
	}

	return s.queries.CreateRefreshToken(ctx, params)
}

// generateUserID generates a unique user ID
func (s *Service) generateUserID() (string, error) {
	return uuid.NewString(), nil
}

// HasPinCode checks if the user has a PIN code set
func (s *Service) HasPinCode(ctx context.Context, userID string) (bool, error) {
	ctx, span := tracer.Start(ctx, "auth.HasPinCode",
		trace.WithAttributes(attribute.String("auth.user_id", userID)))
	defer span.End()

	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		span.RecordError(err)
		return false, err
	}

	hasPinCode, err := s.queries.HasUserPinCode(ctx, userUUID)
	if err != nil {
		span.RecordError(err)
		return false, err
	}

	return hasPinCode, nil
}

// SetupPinCode sets up a new PIN code for the user
func (s *Service) SetupPinCode(ctx context.Context, userID, pinCode string) error {
	ctx, span := tracer.Start(ctx, "auth.SetupPinCode",
		trace.WithAttributes(attribute.String("auth.user_id", userID)))
	defer span.End()

	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return recordedAuthError(span, ErrInvalidCredentials, "Invalid user ID", err)
	}

	// Check if PIN code already exists
	hasPinCode, err := s.queries.HasUserPinCode(ctx, userUUID)
	if err != nil {
		return recordedAuthError(span, ErrPinCodeUpdate, "Failed to check PIN code status", err)
	}

	if hasPinCode {
		return NewAuthError(ErrPinCodeExists, "PIN code already set. Use change PIN code instead.", nil)
	}

	// Hash the PIN code
	hashedPinCode, err := bcrypt.GenerateFromPassword([]byte(pinCode), bcrypt.DefaultCost)
	if err != nil {
		return recordedAuthError(span, ErrPasswordHashing, "Failed to hash PIN code", err)
	}

	// Store the PIN code
	if err := s.queries.SetUserPinCode(ctx, sqlc.SetUserPinCodeParams{
		PinCode: pgutil.Text(string(hashedPinCode)),
		ID:      userUUID,
	}); err != nil {
		return recordedAuthError(span, ErrPinCodeUpdate, "Failed to set PIN code", err)
	}

	return nil
}

// ChangePinCode changes the PIN code for the user
func (s *Service) ChangePinCode(ctx context.Context, userID, currentPinCode, newPinCode string) error {
	ctx, span := tracer.Start(ctx, "auth.ChangePinCode",
		trace.WithAttributes(attribute.String("auth.user_id", userID)))
	defer span.End()

	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return recordedAuthError(span, ErrInvalidCredentials, "Invalid user ID", err)
	}

	// Get current PIN code
	result, err := s.queries.GetUserPinCode(ctx, userUUID)
	if err != nil {
		return recordedAuthError(span, ErrPinCodeUpdate, "Failed to get current PIN code", err)
	}

	if !result.Valid {
		return NewAuthError(ErrNoPinCode, "No PIN code set", nil)
	}

	// Verify current PIN code
	if err := bcrypt.CompareHashAndPassword([]byte(result.String), []byte(currentPinCode)); err != nil {
		return recordedAuthError(span, ErrInvalidCredentials, "Current PIN code is incorrect", err)
	}

	// Hash the new PIN code
	hashedPinCode, err := bcrypt.GenerateFromPassword([]byte(newPinCode), bcrypt.DefaultCost)
	if err != nil {
		return recordedAuthError(span, ErrPasswordHashing, "Failed to hash new PIN code", err)
	}

	// Store the new PIN code
	if err := s.queries.SetUserPinCode(ctx, sqlc.SetUserPinCodeParams{
		PinCode: pgutil.Text(string(hashedPinCode)),
		ID:      userUUID,
	}); err != nil {
		return recordedAuthError(span, ErrPinCodeUpdate, "Failed to update PIN code", err)
	}

	return nil
}

// ResetPinCode resets the PIN code by verifying the account password
func (s *Service) ResetPinCode(ctx context.Context, userID, password string) error {
	ctx, span := tracer.Start(ctx, "auth.ResetPinCode",
		trace.WithAttributes(attribute.String("auth.user_id", userID)))
	defer span.End()

	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return recordedAuthError(span, ErrInvalidCredentials, "Invalid user ID", err)
	}

	// Get user to verify password
	user, err := s.queries.GetUserByID(ctx, userUUID)
	if err != nil {
		return recordedAuthError(span, ErrUserNotFound, "User not found", err)
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return recordedAuthError(span, ErrInvalidCredentials, "Password is incorrect", err)
	}

	// Clear the PIN code
	if err := s.queries.ClearUserPinCode(ctx, userUUID); err != nil {
		return recordedAuthError(span, ErrPinCodeUpdate, "Failed to clear PIN code", err)
	}

	return nil
}

// UnlockSession unlocks a session with PIN code for elevated access
func (s *Service) UnlockSession(ctx context.Context, userID, sessionID, pinCode string) error {
	ctx, span := tracer.Start(ctx, "auth.UnlockSession",
		trace.WithAttributes(
			attribute.String("auth.user_id", userID),
			attribute.String("auth.session_id", sessionID)))
	defer span.End()

	userUUID, err := pgutil.StringToUUID(userID)
	if err != nil {
		return recordedAuthError(span, ErrInvalidCredentials, "Invalid user ID", err)
	}

	// Get user's PIN code
	result, err := s.queries.GetUserPinCode(ctx, userUUID)
	if err != nil {
		return recordedAuthError(span, ErrPinCodeUpdate, "Failed to get PIN code", err)
	}

	if !result.Valid {
		return NewAuthError(ErrNoPinCode, "No PIN code set", nil)
	}

	// Verify PIN code
	if err := bcrypt.CompareHashAndPassword([]byte(result.String), []byte(pinCode)); err != nil {
		return recordedAuthError(span, ErrInvalidCredentials, "PIN code is incorrect", err)
	}

	// Set session PIN elevation (expires in 1 hour)
	sessionUUID, err := pgutil.StringToUUID(sessionID)
	if err != nil {
		return recordedAuthError(span, ErrInvalidCredentials, "Invalid session ID", err)
	}

	if err := s.queries.SetSessionPinElevation(ctx, sqlc.SetSessionPinElevationParams{
		PinExpiresAt: pgutil.TimeToTimestamptz(time.Now().Add(1 * time.Hour)),
		ID:           sessionUUID,
	}); err != nil {
		return recordedAuthError(span, ErrSessionUpdate, "Failed to elevate session", err)
	}

	return nil
}

// LockSession locks a session to revoke elevated access
func (s *Service) LockSession(ctx context.Context, sessionID string) error {
	ctx, span := tracer.Start(ctx, "auth.LockSession",
		trace.WithAttributes(attribute.String("auth.session_id", sessionID)))
	defer span.End()

	sessionUUID, err := pgutil.StringToUUID(sessionID)
	if err != nil {
		return recordedAuthError(span, ErrInvalidCredentials, "Invalid session ID", err)
	}

	if err := s.queries.ClearSessionPinElevation(ctx, sessionUUID); err != nil {
		return recordedAuthError(span, ErrSessionUpdate, "Failed to lock session", err)
	}

	return nil
}

// IsSessionElevated checks if a session has elevated access
func (s *Service) IsSessionElevated(ctx context.Context, sessionID string) (bool, error) {
	ctx, span := tracer.Start(ctx, "auth.IsSessionElevated",
		trace.WithAttributes(attribute.String("auth.session_id", sessionID)))
	defer span.End()

	sessionUUID, err := pgutil.StringToUUID(sessionID)
	if err != nil {
		span.RecordError(err)
		return false, err
	}

	isElevated, err := s.queries.IsSessionElevated(ctx, sessionUUID)
	if err != nil {
		span.RecordError(err)
		return false, err
	}

	return isElevated, nil
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
