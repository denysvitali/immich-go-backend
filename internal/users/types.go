package users

import (
	"time"

	"github.com/google/uuid"
)

// UserInfo represents user information
type UserInfo struct {
	ID                   uuid.UUID  `json:"id"`
	Email                string     `json:"email"`
	Name                 string     `json:"name"`
	IsAdmin              bool       `json:"isAdmin"`
	ShouldChangePassword bool       `json:"shouldChangePassword"`
	Status               string     `json:"status"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	QuotaUsageInBytes    int64      `json:"quotaUsageInBytes"`
	OAuthID              string     `json:"oauthId"`
	ProfileImagePath     *string    `json:"profileImagePath,omitempty"`
	StorageLabel         *string    `json:"storageLabel,omitempty"`
	QuotaSizeInBytes     *int64     `json:"quotaSizeInBytes,omitempty"`
	AvatarColor          *string    `json:"avatarColor,omitempty"`
	ProfileChangedAt     *time.Time `json:"profileChangedAt,omitempty"`
	DeletedAt            *time.Time `json:"deletedAt,omitempty"`
}

// UserPreferences represents user preferences
type UserPreferences struct {
	UserID                        uuid.UUID `json:"userId"`
	EmailNotifications            *bool     `json:"emailNotifications,omitempty"`
	DownloadIncludeEmbeddedVideos *bool     `json:"downloadIncludeEmbeddedVideos,omitempty"`
	FoldersEnabled                *bool     `json:"foldersEnabled,omitempty"`
	MemoriesEnabled               *bool     `json:"memoriesEnabled,omitempty"`
	PeopleEnabled                 *bool     `json:"peopleEnabled,omitempty"`
	PeopleSizeThreshold           *int32    `json:"peopleSizeThreshold,omitempty"`
	SharedLinksEnabled            *bool     `json:"sharedLinksEnabled,omitempty"`
	TagsEnabled                   *bool     `json:"tagsEnabled,omitempty"`
	TagsSizeThreshold             *int32    `json:"tagsSizeThreshold,omitempty"`
}

// ListUsersRequest represents a request to list users
type ListUsersRequest struct {
	Limit          int  `json:"limit"`
	Offset         int  `json:"offset"`
	IncludeDeleted bool `json:"includeDeleted"`
}

// ListUsersResponse represents the response from listing users
type ListUsersResponse struct {
	Users  []*UserInfo `json:"users"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

// UpdateUserRequest represents a request to update user information
type UpdateUserRequest struct {
	Name             *string `json:"name,omitempty"`
	Email            *string `json:"email,omitempty"`
	AvatarColor      *string `json:"avatarColor,omitempty"`
	ProfileImagePath *string `json:"profileImagePath,omitempty"`
	QuotaSizeInBytes *int64  `json:"quotaSizeInBytes,omitempty"`
	StorageLabel     *string `json:"storageLabel,omitempty"`
}

// UpdatePasswordRequest represents a request to update a user's password
type UpdatePasswordRequest struct {
	NewPassword string `json:"newPassword"`
}

// UpdateUserPreferencesRequest represents a request to update user preferences
type UpdateUserPreferencesRequest struct {
	EmailNotifications            *bool  `json:"emailNotifications,omitempty"`
	DownloadIncludeEmbeddedVideos *bool  `json:"downloadIncludeEmbeddedVideos,omitempty"`
	FoldersEnabled                *bool  `json:"foldersEnabled,omitempty"`
	MemoriesEnabled               *bool  `json:"memoriesEnabled,omitempty"`
	PeopleEnabled                 *bool  `json:"peopleEnabled,omitempty"`
	PeopleSizeThreshold           *int32 `json:"peopleSizeThreshold,omitempty"`
	SharedLinksEnabled            *bool  `json:"sharedLinksEnabled,omitempty"`
	TagsEnabled                   *bool  `json:"tagsEnabled,omitempty"`
	TagsSizeThreshold             *int32 `json:"tagsSizeThreshold,omitempty"`
}

// OnboardingStatus represents user's onboarding status
type OnboardingStatus struct {
	IsOnboarded bool `json:"isOnboarded"`
}

// UserError represents errors that can occur in user operations
type UserError struct {
	Type    UserErrorType `json:"type"`
	Message string        `json:"message"`
	Err     error         `json:"-"`
}

func (e *UserError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *UserError) Unwrap() error {
	return e.Err
}

// UserErrorType represents different types of user errors
type UserErrorType string

const (
	ErrInvalidUserID   UserErrorType = "invalid_user_id"
	ErrUserNotFound    UserErrorType = "user_not_found"
	ErrUserDeleted     UserErrorType = "user_deleted"
	ErrUserExists      UserErrorType = "user_exists"
	ErrInvalidPassword UserErrorType = "invalid_password"
	ErrPasswordHashing UserErrorType = "password_hashing"
	ErrDatabaseError   UserErrorType = "database_error"
	ErrUnauthorized    UserErrorType = "unauthorized"
	ErrInvalidInput    UserErrorType = "invalid_input"
)

// NewUserError creates a new user error
func NewUserError(errorType UserErrorType, message string, err error) *UserError {
	return &UserError{
		Type:    errorType,
		Message: message,
		Err:     err,
	}
}

// NewUserNotFoundError creates a user not found error
func NewUserNotFoundError(message string) *UserError {
	return &UserError{
		Type:    ErrUserNotFound,
		Message: message,
	}
}

// NewInvalidUserIDError creates an invalid user ID error
func NewInvalidUserIDError(message string, err error) *UserError {
	return &UserError{
		Type:    ErrInvalidUserID,
		Message: message,
		Err:     err,
	}
}

// NewDatabaseError creates a database error
func NewDatabaseError(message string, err error) *UserError {
	return &UserError{
		Type:    ErrDatabaseError,
		Message: message,
		Err:     err,
	}
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string) *UserError {
	return &UserError{
		Type:    ErrUnauthorized,
		Message: message,
	}
}

// IsNotFoundError checks if an error is a user not found error
func IsNotFoundError(err error) bool {
	if userErr, ok := err.(*UserError); ok {
		return userErr.Type == ErrUserNotFound
	}
	return false
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	if userErr, ok := err.(*UserError); ok {
		return userErr.Type == ErrInvalidInput
	}
	return false
}

// IsUserError checks if an error is a UserError
func IsUserError(err error) bool {
	_, ok := err.(*UserError)
	return ok
}

// GetUserErrorType returns the type of a UserError
func GetUserErrorType(err error) UserErrorType {
	if userErr, ok := err.(*UserError); ok {
		return userErr.Type
	}
	return ""
}
