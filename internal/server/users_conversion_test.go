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

func TestUserPreferencesToProtoMapsStoredValues(t *testing.T) {
	got := userPreferencesToProto(&users.UserPreferences{
		EmailNotifications:            boolPtr(false),
		EmailAlbumInvite:              boolPtr(true),
		EmailAlbumUpdate:              boolPtr(false),
		DownloadIncludeEmbeddedVideos: boolPtr(true),
		FoldersEnabled:                boolPtr(true),
		FoldersSizeThreshold:          int32Ptr(3),
		MemoriesEnabled:               boolPtr(false),
		PeopleEnabled:                 boolPtr(true),
		PeopleSizeThreshold:           int32Ptr(7),
		PurchaseShowSupportBadge:      boolPtr(true),
		RatingsEnabled:                boolPtr(false),
		SharedLinksEnabled:            boolPtr(true),
		SharedLinksShowMetadata:       boolPtr(false),
		SharedLinksPasswordOptions:    stringPtr("required"),
		TagsEnabled:                   boolPtr(true),
		TagsSizeThreshold:             int32Ptr(9),
	})

	assert.True(t, got.Download.IncludeEmbeddedVideos)
	assert.False(t, got.EmailNotifications.Enabled)
	assert.True(t, got.EmailNotifications.AlbumInvite)
	assert.False(t, got.EmailNotifications.AlbumUpdate)
	assert.True(t, got.Folders.Enabled)
	assert.Equal(t, int32(3), got.Folders.SizeThreshold)
	assert.False(t, got.Memories.Enabled)
	assert.True(t, got.People.Enabled)
	assert.Equal(t, int32(7), got.People.SizeThreshold)
	assert.True(t, got.Purchase.ShowSupportBadge)
	assert.False(t, got.Ratings.Enabled)
	assert.True(t, got.SharedLinks.Enabled)
	assert.False(t, got.SharedLinks.ShowMetadata)
	assert.Equal(t, "required", got.SharedLinks.PasswordOptions)
	assert.True(t, got.Tags.Enabled)
	assert.Equal(t, int32(9), got.Tags.SizeThreshold)
}

func TestUserPreferencesUpdateFromProtoMapsNestedFields(t *testing.T) {
	got := userPreferencesUpdateFromProto(&immichv1.UserPreferencesUpdateRequest{
		Download: &immichv1.DownloadUpdate{
			IncludeEmbeddedVideos: boolPtr(true),
		},
		EmailNotifications: &immichv1.EmailNotificationsUpdate{
			Enabled:     boolPtr(false),
			AlbumInvite: boolPtr(true),
			AlbumUpdate: boolPtr(false),
		},
		Folders: &immichv1.FoldersUpdate{
			Enabled:       boolPtr(true),
			SizeThreshold: int32Ptr(4),
		},
		Memories: &immichv1.MemoriesUpdate{
			Enabled: boolPtr(false),
		},
		People: &immichv1.PeopleUpdate{
			Enabled:       boolPtr(true),
			SizeThreshold: int32Ptr(8),
		},
		Purchase: &immichv1.PurchaseUpdate{
			ShowSupportBadge: boolPtr(true),
		},
		Ratings: &immichv1.RatingsUpdate{
			Enabled: boolPtr(false),
		},
		SharedLinks: &immichv1.SharedLinksUpdate{
			Enabled:         boolPtr(true),
			ShowMetadata:    boolPtr(false),
			PasswordOptions: stringPtr("required"),
		},
		Tags: &immichv1.TagsUpdate{
			Enabled:       boolPtr(true),
			SizeThreshold: int32Ptr(12),
		},
	})

	assert.True(t, *got.DownloadIncludeEmbeddedVideos)
	assert.False(t, *got.EmailNotifications)
	assert.True(t, *got.EmailAlbumInvite)
	assert.False(t, *got.EmailAlbumUpdate)
	assert.True(t, *got.FoldersEnabled)
	assert.Equal(t, int32(4), *got.FoldersSizeThreshold)
	assert.False(t, *got.MemoriesEnabled)
	assert.True(t, *got.PeopleEnabled)
	assert.Equal(t, int32(8), *got.PeopleSizeThreshold)
	assert.True(t, *got.PurchaseShowSupportBadge)
	assert.False(t, *got.RatingsEnabled)
	assert.True(t, *got.SharedLinksEnabled)
	assert.False(t, *got.SharedLinksShowMetadata)
	assert.Equal(t, "required", *got.SharedLinksPasswordOptions)
	assert.True(t, *got.TagsEnabled)
	assert.Equal(t, int32(12), *got.TagsSizeThreshold)
}

func stringPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func int32Ptr(value int32) *int32 {
	return &value
}
