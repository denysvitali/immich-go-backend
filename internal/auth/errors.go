package auth

import "fmt"

// AuthErrorType represents the type of authentication error
type AuthErrorType string

const (
	// Authentication errors
	ErrInvalidCredentials AuthErrorType = "invalid_credentials"
	ErrInvalidToken       AuthErrorType = "invalid_token"
	ErrTokenExpired       AuthErrorType = "token_expired"
	ErrTokenGeneration    AuthErrorType = "token_generation"
	ErrTokenStorage       AuthErrorType = "token_storage"
	ErrTokenDeletion      AuthErrorType = "token_deletion"

	// User errors
	ErrUserNotFound AuthErrorType = "user_not_found"
	ErrUserExists   AuthErrorType = "user_exists"
	ErrUserDeleted  AuthErrorType = "user_deleted"
	ErrUserCreation AuthErrorType = "user_creation"

	// Password errors
	ErrInvalidPassword AuthErrorType = "invalid_password"
	ErrPasswordHashing AuthErrorType = "password_hashing"
	ErrPasswordUpdate  AuthErrorType = "password_update"

	// Registration errors
	ErrRegistrationDisabled AuthErrorType = "registration_disabled"

	// Permission errors
	ErrInsufficientPermissions AuthErrorType = "insufficient_permissions"
	ErrUnauthorized            AuthErrorType = "unauthorized"
)

// AuthError represents an authentication error
type AuthError struct {
	Type    AuthErrorType `json:"type"`
	Message string        `json:"message"`
	Err     error         `json:"-"`
}

// Error implements the error interface
func (e *AuthError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *AuthError) Unwrap() error {
	return e.Err
}

// IsAuthError checks if an error is an AuthError
func IsAuthError(err error) bool {
	_, ok := err.(*AuthError)
	return ok
}

// GetAuthErrorType returns the type of an AuthError
func GetAuthErrorType(err error) AuthErrorType {
	if authErr, ok := err.(*AuthError); ok {
		return authErr.Type
	}
	return ""
}

// NewAuthError creates a new AuthError
func NewAuthError(errorType AuthErrorType, message string, err error) *AuthError {
	return &AuthError{
		Type:    errorType,
		Message: message,
		Err:     err,
	}
}

// NewInvalidCredentialsError creates an invalid credentials error
func NewInvalidCredentialsError(message string) *AuthError {
	return &AuthError{
		Type:    ErrInvalidCredentials,
		Message: message,
	}
}

// NewInvalidTokenError creates an invalid token error
func NewInvalidTokenError(message string, err error) *AuthError {
	return &AuthError{
		Type:    ErrInvalidToken,
		Message: message,
		Err:     err,
	}
}

// NewTokenExpiredError creates a token expired error
func NewTokenExpiredError() *AuthError {
	return &AuthError{
		Type:    ErrTokenExpired,
		Message: "Token has expired",
	}
}

// NewUserNotFoundError creates a user not found error
func NewUserNotFoundError() *AuthError {
	return &AuthError{
		Type:    ErrUserNotFound,
		Message: "User not found",
	}
}

// NewUserExistsError creates a user exists error
func NewUserExistsError() *AuthError {
	return &AuthError{
		Type:    ErrUserExists,
		Message: "User already exists",
	}
}

// NewRegistrationDisabledError creates a registration disabled error
func NewRegistrationDisabledError() *AuthError {
	return &AuthError{
		Type:    ErrRegistrationDisabled,
		Message: "User registration is disabled",
	}
}

// NewInsufficientPermissionsError creates an insufficient permissions error
func NewInsufficientPermissionsError(message string) *AuthError {
	return &AuthError{
		Type:    ErrInsufficientPermissions,
		Message: message,
	}
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string) *AuthError {
	return &AuthError{
		Type:    ErrUnauthorized,
		Message: message,
	}
}
