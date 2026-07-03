package notifications

import (
	"testing"
	"time"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationTypeToProto(t *testing.T) {
	tests := []struct {
		name             string
		notificationType string
		want             immichv1.NotificationType
	}{
		{"job failed", "job_failed", immichv1.NotificationType_NOTIFICATION_TYPE_JOB_FAILED},
		{"backup failed", "backup_failed", immichv1.NotificationType_NOTIFICATION_TYPE_BACKUP_FAILED},
		{"custom", "custom", immichv1.NotificationType_NOTIFICATION_TYPE_CUSTOM},
		{"system default", "system", immichv1.NotificationType_NOTIFICATION_TYPE_SYSTEM_MESSAGE},
		{"push default", "push", immichv1.NotificationType_NOTIFICATION_TYPE_SYSTEM_MESSAGE},
		{"empty default", "", immichv1.NotificationType_NOTIFICATION_TYPE_SYSTEM_MESSAGE},
		{"unknown default", "unexpected", immichv1.NotificationType_NOTIFICATION_TYPE_SYSTEM_MESSAGE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, notificationTypeToProto(tt.notificationType))
		})
	}
}

func TestNotificationLevelToProto(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  immichv1.NotificationLevel
	}{
		{"success", "success", immichv1.NotificationLevel_NOTIFICATION_LEVEL_SUCCESS},
		{"error", "error", immichv1.NotificationLevel_NOTIFICATION_LEVEL_ERROR},
		{"warning", "warning", immichv1.NotificationLevel_NOTIFICATION_LEVEL_WARNING},
		{"info", "info", immichv1.NotificationLevel_NOTIFICATION_LEVEL_INFO},
		{"empty default", "", immichv1.NotificationLevel_NOTIFICATION_LEVEL_INFO},
		{"unknown default", "unexpected", immichv1.NotificationLevel_NOTIFICATION_LEVEL_INFO},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, notificationLevelToProto(tt.level))
		})
	}
}

func TestNotificationToProto(t *testing.T) {
	createdAt := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	readAt := createdAt.Add(time.Hour)

	got := notificationToProto(&Notification{
		ID:          "notification-id",
		Type:        "custom",
		Level:       "success",
		Title:       "Backup complete",
		Description: "Everything finished",
		Data: map[string]interface{}{
			"assetCount": float64(42),
			"source":     "library",
		},
		Read:      true,
		ReadAt:    &readAt,
		CreatedAt: createdAt,
	})

	require.NotNil(t, got)
	assert.Equal(t, "notification-id", got.Id)
	assert.Equal(t, "Backup complete", got.Title)
	require.NotNil(t, got.Description)
	assert.Equal(t, "Everything finished", got.GetDescription())
	assert.Equal(t, immichv1.NotificationType_NOTIFICATION_TYPE_CUSTOM, got.Type)
	assert.Equal(t, immichv1.NotificationLevel_NOTIFICATION_LEVEL_SUCCESS, got.Level)
	assert.Equal(t, createdAt, got.CreatedAt.AsTime())
	require.NotNil(t, got.ReadAt)
	assert.Equal(t, readAt, got.ReadAt.AsTime())
	require.NotNil(t, got.Data)
	assert.Equal(t, float64(42), got.Data.Fields["assetCount"].GetNumberValue())
	assert.Equal(t, "library", got.Data.Fields["source"].GetStringValue())
}

func TestNotificationToProtoUsesReadFlagFallback(t *testing.T) {
	before := time.Now()

	got := notificationToProto(&Notification{
		ID:        "notification-id",
		Read:      true,
		CreatedAt: before,
	})

	require.NotNil(t, got.ReadAt)
	assert.False(t, got.ReadAt.AsTime().Before(before))
	assert.False(t, got.ReadAt.AsTime().After(time.Now()))
}
