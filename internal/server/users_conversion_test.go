package server

import (
	"testing"
	"time"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/users"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUserAvatarColorToProto(t *testing.T) {
	tests := []struct {
		name  string
		color *string
		want  immichv1.UserAvatarColor
	}{
		{"nil", nil, immichv1.UserAvatarColor_USER_AVATAR_COLOR_BLUE},
		{"red", stringPtr("red"), immichv1.UserAvatarColor_USER_AVATAR_COLOR_RED},
		{"green", stringPtr("green"), immichv1.UserAvatarColor_USER_AVATAR_COLOR_GREEN},
		{"yellow", stringPtr("yellow"), immichv1.UserAvatarColor_USER_AVATAR_COLOR_YELLOW},
		{"orange", stringPtr("orange"), immichv1.UserAvatarColor_USER_AVATAR_COLOR_ORANGE},
		{"purple", stringPtr("purple"), immichv1.UserAvatarColor_USER_AVATAR_COLOR_PURPLE},
		{"pink", stringPtr("pink"), immichv1.UserAvatarColor_USER_AVATAR_COLOR_PINK},
		{"unknown", stringPtr("unknown"), immichv1.UserAvatarColor_USER_AVATAR_COLOR_BLUE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, userAvatarColorToProto(tt.color))
		})
	}
}

func TestUserStatusToProto(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   immichv1.UserStatus
	}{
		{"active", "active", immichv1.UserStatus_USER_STATUS_ACTIVE},
		{"empty", "", immichv1.UserStatus_USER_STATUS_ACTIVE},
		{"removing", "removing", immichv1.UserStatus_USER_STATUS_REMOVING},
		{"deleted", "deleted", immichv1.UserStatus_USER_STATUS_DELETED},
		{"unknown", "unknown", immichv1.UserStatus_USER_STATUS_ACTIVE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, userStatusToProto(tt.status))
		})
	}
}

func TestConvertUserToAdminProtoUsesSharedMappings(t *testing.T) {
	userID := uuid.New()
	createdAt := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)

	got := (&Server{}).convertUserToAdminProto(&users.UserInfo{
		ID:                   userID,
		Email:                "user@example.com",
		Name:                 "Test User",
		IsAdmin:              true,
		ShouldChangePassword: true,
		Status:               "deleted",
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
		QuotaUsageInBytes:    42,
		AvatarColor:          stringPtr("purple"),
	})

	assert.Equal(t, userID.String(), got.Id)
	assert.Equal(t, "user@example.com", got.Email)
	assert.Equal(t, "Test User", got.Name)
	assert.True(t, got.IsAdmin)
	assert.True(t, got.ShouldChangePassword)
	assert.Equal(t, immichv1.UserAvatarColor_USER_AVATAR_COLOR_PURPLE, got.AvatarColor)
	assert.Equal(t, immichv1.UserStatus_USER_STATUS_DELETED, got.Status)
	assert.Equal(t, createdAt, got.CreatedAt.AsTime())
	assert.Equal(t, updatedAt, got.UpdatedAt.AsTime())
	assert.NotNil(t, got.QuotaUsageInBytes)
	assert.Equal(t, int64(42), *got.QuotaUsageInBytes)
}

func TestConvertUserToProtoUsesSharedAvatarColorMapping(t *testing.T) {
	userID := uuid.New()
	got := (&Server{}).convertUserToProto(&users.UserInfo{
		ID:          userID,
		Email:       "user@example.com",
		Name:        "Test User",
		AvatarColor: stringPtr("orange"),
	})

	assert.Equal(t, userID.String(), got.Id)
	assert.Equal(t, "user@example.com", got.Email)
	assert.Equal(t, "Test User", got.Name)
	assert.Equal(t, immichv1.UserAvatarColor_USER_AVATAR_COLOR_ORANGE, got.AvatarColor)
}

func stringPtr(value string) *string {
	return &value
}
