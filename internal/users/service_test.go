package users

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// MockQueries implements a minimal mock for testing
type MockQueries struct {
	users map[uuid.UUID]*UserInfo
}

func NewMockQueries() *MockQueries {
	return &MockQueries{
		users: make(map[uuid.UUID]*UserInfo),
	}
}

func TestService_GetUser(t *testing.T) {
	service, _ := NewService(nil, nil)

	t.Run("service exists", func(t *testing.T) {
		// This test would need proper mocking of the database layer
		// For now, we're testing the service structure
		assert.NotNil(t, service)
	})
}

func TestService_ValidatePassword(t *testing.T) {
	service, _ := NewService(nil, nil)

	tests := []struct {
		name     string
		password string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid password",
			password: "ValidPass123!",
			wantErr:  false,
		},
		{
			name:     "too short",
			password: "Short1!",
			wantErr:  true,
			errMsg:   "at least 8 characters",
		},
		{
			name:     "empty password",
			password: "",
			wantErr:  true,
			errMsg:   "password cannot be empty",
		},
		{
			name:     "only spaces",
			password: "        ",
			wantErr:  false, // Spaces are technically valid unless checked
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validatePassword(tt.password)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestService_GetDefaultUserPreferences(t *testing.T) {
	service, _ := NewService(nil, nil)
	userID := uuid.New()

	prefs := service.getDefaultUserPreferences(userID)

	assert.NotNil(t, prefs)
	assert.Equal(t, userID, prefs.UserID)

	// Check default values
	assert.NotNil(t, prefs.EmailNotifications)
	assert.True(t, *prefs.EmailNotifications)

	assert.NotNil(t, prefs.MemoriesEnabled)
	assert.True(t, *prefs.MemoriesEnabled)

	assert.NotNil(t, prefs.FoldersEnabled)
	assert.True(t, *prefs.FoldersEnabled)

	assert.NotNil(t, prefs.PeopleEnabled)
	assert.True(t, *prefs.PeopleEnabled)

	assert.NotNil(t, prefs.TagsEnabled)
	assert.True(t, *prefs.TagsEnabled)
}

func TestService_UpdateUserRequest(t *testing.T) {
	t.Run("partial update", func(t *testing.T) {
		newName := "Updated Name"
		req := UpdateUserRequest{
			Name: &newName,
		}

		assert.NotNil(t, req.Name)
		assert.Equal(t, "Updated Name", *req.Name)
		assert.Nil(t, req.Email)
		assert.Nil(t, req.AvatarColor)
	})

	t.Run("full update", func(t *testing.T) {
		name := "Full Name"
		email := "new@example.com"
		color := "blue"
		path := "/profile.jpg"
		quota := int64(10737418240) // 10GB
		label := "primary"

		req := UpdateUserRequest{
			Name:             &name,
			Email:            &email,
			AvatarColor:      &color,
			ProfileImagePath: &path,
			QuotaSizeInBytes: &quota,
			StorageLabel:     &label,
		}

		assert.NotNil(t, req.Name)
		assert.NotNil(t, req.Email)
		assert.NotNil(t, req.AvatarColor)
		assert.NotNil(t, req.ProfileImagePath)
		assert.NotNil(t, req.QuotaSizeInBytes)
		assert.NotNil(t, req.StorageLabel)
	})
}

func TestListUsersRequest(t *testing.T) {
	t.Run("default pagination", func(t *testing.T) {
		req := ListUsersRequest{
			Limit:  10,
			Offset: 0,
		}

		assert.Equal(t, 10, req.Limit)
		assert.Equal(t, 0, req.Offset)
		assert.False(t, req.IncludeDeleted)
	})

	t.Run("with deleted users", func(t *testing.T) {
		req := ListUsersRequest{
			Limit:          20,
			Offset:         10,
			IncludeDeleted: true,
		}

		assert.Equal(t, 20, req.Limit)
		assert.Equal(t, 10, req.Offset)
		assert.True(t, req.IncludeDeleted)
	})
}

func TestUserPreferences(t *testing.T) {
	t.Run("update preferences", func(t *testing.T) {
		emailNotif := false
		memoriesEnabled := false
		peopleThreshold := int32(5)

		req := UpdateUserPreferencesRequest{
			EmailNotifications:  &emailNotif,
			MemoriesEnabled:     &memoriesEnabled,
			PeopleSizeThreshold: &peopleThreshold,
		}

		assert.NotNil(t, req.EmailNotifications)
		assert.False(t, *req.EmailNotifications)
		assert.NotNil(t, req.MemoriesEnabled)
		assert.False(t, *req.MemoriesEnabled)
		assert.NotNil(t, req.PeopleSizeThreshold)
		assert.Equal(t, int32(5), *req.PeopleSizeThreshold)

		// Check that unset fields remain nil
		assert.Nil(t, req.FoldersEnabled)
		assert.Nil(t, req.TagsEnabled)
	})
}

func TestUserError(t *testing.T) {
	t.Run("error without wrapped error", func(t *testing.T) {
		err := &UserError{
			Type:    UserErrorType("not_found"),
			Message: "User not found",
		}

		assert.Equal(t, "User not found", err.Error())
		assert.Nil(t, err.Unwrap())
	})

	t.Run("error with wrapped error", func(t *testing.T) {
		innerErr := assert.AnError
		err := &UserError{
			Type:    UserErrorType("database_error"),
			Message: "Database operation failed",
			Err:     innerErr,
		}

		assert.Contains(t, err.Error(), "Database operation failed")
		assert.Contains(t, err.Error(), innerErr.Error())
		assert.Equal(t, innerErr, err.Unwrap())
	})
}

func TestUserInfo_Helpers(t *testing.T) {
	now := time.Now()
	profilePath := "/profile.jpg"
	avatarColor := "blue"
	storageLabel := "primary"
	quotaSize := int64(10737418240)

	user := UserInfo{
		ID:               uuid.New(),
		Email:            "test@example.com",
		Name:             "Test User",
		IsAdmin:          true,
		Status:           "active",
		CreatedAt:        now,
		UpdatedAt:        now,
		ProfileImagePath: &profilePath,
		AvatarColor:      &avatarColor,
		StorageLabel:     &storageLabel,
		QuotaSizeInBytes: &quotaSize,
	}

	assert.NotEqual(t, uuid.Nil, user.ID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.True(t, user.IsAdmin)
	assert.NotNil(t, user.ProfileImagePath)
	assert.Equal(t, "/profile.jpg", *user.ProfileImagePath)
	assert.NotNil(t, user.QuotaSizeInBytes)
	assert.Equal(t, int64(10737418240), *user.QuotaSizeInBytes)
}