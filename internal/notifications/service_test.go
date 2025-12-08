//go:build integration
// +build integration

package notifications

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestUser creates a test user and returns the user ID
func createTestUser(t *testing.T, tdb *testdb.TestDB, email string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	userID := uuid.New()
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	_, err := tdb.Queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    email,
		Name:     "Test User",
		Password: "hashedpassword",
		IsAdmin:  false,
	})
	require.NoError(t, err)

	return userID
}

func TestIntegration_CreateNotification(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "notification@test.com")

	// Create notification
	notification := &Notification{
		UserID:      userID.String(),
		Type:        "system",
		Level:       "info",
		Title:       "Test Notification",
		Description: "This is a test notification",
	}

	err := service.CreateNotification(ctx, notification)
	require.NoError(t, err)
	assert.NotEmpty(t, notification.ID)
	assert.False(t, notification.CreatedAt.IsZero())
}

func TestIntegration_CreateNotification_InvalidUserID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Try to create notification with invalid user ID
	notification := &Notification{
		UserID: "not-a-valid-uuid",
		Title:  "Test",
	}

	err := service.CreateNotification(ctx, notification)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid user ID")
}

func TestIntegration_GetNotification(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and notification
	userID := createTestUser(t, tdb, "getnotif@test.com")

	notification := &Notification{
		UserID:      userID.String(),
		Type:        "alert",
		Level:       "warning",
		Title:       "Important Alert",
		Description: "Something needs attention",
	}

	err := service.CreateNotification(ctx, notification)
	require.NoError(t, err)

	// Get the notification
	result, err := service.GetNotification(ctx, userID.String(), notification.ID)
	require.NoError(t, err)
	assert.Equal(t, notification.ID, result.ID)
	assert.Equal(t, "Important Alert", result.Title)
	assert.Equal(t, "warning", result.Level)
	assert.False(t, result.Read)
}

func TestIntegration_GetNotification_NotOwned(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "owner@test.com")
	user2ID := createTestUser(t, tdb, "notowner@test.com")

	// Create notification for user1
	notification := &Notification{
		UserID: user1ID.String(),
		Title:  "Private Notification",
	}

	err := service.CreateNotification(ctx, notification)
	require.NoError(t, err)

	// User2 tries to get user1's notification
	_, err = service.GetNotification(ctx, user2ID.String(), notification.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestIntegration_GetNotifications(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "multinotif@test.com")

	// Create multiple notifications
	for i := 0; i < 3; i++ {
		notification := &Notification{
			UserID: userID.String(),
			Title:  "Notification",
			Type:   "system",
			Level:  "info",
		}
		err := service.CreateNotification(ctx, notification)
		require.NoError(t, err)
	}

	// Get all notifications
	notifications, err := service.GetNotifications(ctx, userID.String(), false)
	require.NoError(t, err)
	assert.Len(t, notifications, 3)
}

func TestIntegration_GetNotifications_UnreadOnly(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "unreadonly@test.com")

	// Create notifications
	notif1 := &Notification{UserID: userID.String(), Title: "Unread 1", Type: "system", Level: "info"}
	notif2 := &Notification{UserID: userID.String(), Title: "Unread 2", Type: "system", Level: "info"}
	notif3 := &Notification{UserID: userID.String(), Title: "Unread 3", Type: "system", Level: "info"}

	err := service.CreateNotification(ctx, notif1)
	require.NoError(t, err)
	err = service.CreateNotification(ctx, notif2)
	require.NoError(t, err)
	err = service.CreateNotification(ctx, notif3)
	require.NoError(t, err)

	// Mark one as read
	err = service.MarkAsRead(ctx, userID.String(), notif1.ID)
	require.NoError(t, err)

	// Get unread only
	unread, err := service.GetNotifications(ctx, userID.String(), true)
	require.NoError(t, err)
	assert.Len(t, unread, 2)

	// Get all
	all, err := service.GetNotifications(ctx, userID.String(), false)
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestIntegration_GetNotifications_UserIsolation(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "user1notif@test.com")
	user2ID := createTestUser(t, tdb, "user2notif@test.com")

	// Create notifications for user1
	for i := 0; i < 2; i++ {
		notification := &Notification{UserID: user1ID.String(), Title: "User1", Type: "system", Level: "info"}
		err := service.CreateNotification(ctx, notification)
		require.NoError(t, err)
	}

	// Create notification for user2
	notification := &Notification{UserID: user2ID.String(), Title: "User2", Type: "system", Level: "info"}
	err := service.CreateNotification(ctx, notification)
	require.NoError(t, err)

	// User1 should only see their notifications
	notifs1, err := service.GetNotifications(ctx, user1ID.String(), false)
	require.NoError(t, err)
	assert.Len(t, notifs1, 2)

	// User2 should only see their notifications
	notifs2, err := service.GetNotifications(ctx, user2ID.String(), false)
	require.NoError(t, err)
	assert.Len(t, notifs2, 1)
}

func TestIntegration_MarkAsRead(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and notification
	userID := createTestUser(t, tdb, "markread@test.com")

	notification := &Notification{
		UserID: userID.String(),
		Title:  "To Be Read",
		Type:   "system",
		Level:  "info",
	}

	err := service.CreateNotification(ctx, notification)
	require.NoError(t, err)

	// Verify it's unread
	result, err := service.GetNotification(ctx, userID.String(), notification.ID)
	require.NoError(t, err)
	assert.False(t, result.Read)

	// Mark as read
	err = service.MarkAsRead(ctx, userID.String(), notification.ID)
	require.NoError(t, err)

	// Verify it's now read
	result, err = service.GetNotification(ctx, userID.String(), notification.ID)
	require.NoError(t, err)
	assert.True(t, result.Read)
	assert.NotNil(t, result.ReadAt)
}

func TestIntegration_MarkAsRead_NotOwned(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "readowner@test.com")
	user2ID := createTestUser(t, tdb, "readnotowner@test.com")

	// Create notification for user1
	notification := &Notification{UserID: user1ID.String(), Title: "Private", Type: "system", Level: "info"}
	err := service.CreateNotification(ctx, notification)
	require.NoError(t, err)

	// User2 tries to mark user1's notification as read
	err = service.MarkAsRead(ctx, user2ID.String(), notification.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestIntegration_MarkAllAsRead(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "markallread@test.com")

	// Create multiple notifications
	for i := 0; i < 3; i++ {
		notification := &Notification{UserID: userID.String(), Title: "Unread", Type: "system", Level: "info"}
		err := service.CreateNotification(ctx, notification)
		require.NoError(t, err)
	}

	// Verify all are unread
	unread, err := service.GetNotifications(ctx, userID.String(), true)
	require.NoError(t, err)
	assert.Len(t, unread, 3)

	// Mark all as read
	err = service.MarkAllAsRead(ctx, userID.String())
	require.NoError(t, err)

	// Verify none are unread
	unread, err = service.GetNotifications(ctx, userID.String(), true)
	require.NoError(t, err)
	assert.Empty(t, unread)
}

func TestIntegration_DeleteNotification(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user and notification
	userID := createTestUser(t, tdb, "delete@test.com")

	notification := &Notification{
		UserID: userID.String(),
		Title:  "To Be Deleted",
		Type:   "system",
		Level:  "info",
	}

	err := service.CreateNotification(ctx, notification)
	require.NoError(t, err)

	// Delete notification
	err = service.DeleteNotification(ctx, userID.String(), notification.ID)
	require.NoError(t, err)

	// Verify it's deleted (GetNotification should fail)
	_, err = service.GetNotification(ctx, userID.String(), notification.ID)
	assert.Error(t, err)
}

func TestIntegration_DeleteNotification_NotOwned(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create two users
	user1ID := createTestUser(t, tdb, "deleteowner@test.com")
	user2ID := createTestUser(t, tdb, "deletenotowner@test.com")

	// Create notification for user1
	notification := &Notification{UserID: user1ID.String(), Title: "Private", Type: "system", Level: "info"}
	err := service.CreateNotification(ctx, notification)
	require.NoError(t, err)

	// User2 tries to delete user1's notification
	err = service.DeleteNotification(ctx, user2ID.String(), notification.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestIntegration_GetUnreadCount(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "count@test.com")

	// Initially should be 0
	count, err := service.GetUnreadCount(ctx, userID.String())
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Create notifications
	for i := 0; i < 5; i++ {
		notification := &Notification{UserID: userID.String(), Title: "Unread", Type: "system", Level: "info"}
		err := service.CreateNotification(ctx, notification)
		require.NoError(t, err)
	}

	// Should be 5
	count, err = service.GetUnreadCount(ctx, userID.String())
	require.NoError(t, err)
	assert.Equal(t, 5, count)

	// Mark all as read
	err = service.MarkAllAsRead(ctx, userID.String())
	require.NoError(t, err)

	// Should be 0 again
	count, err = service.GetUnreadCount(ctx, userID.String())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestIntegration_SendPushNotification(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "push@test.com")

	// Send push notification
	err := service.SendPushNotification(ctx, userID.String(), "Push Title", "Push Body")
	require.NoError(t, err)

	// Verify notification was created
	notifications, err := service.GetNotifications(ctx, userID.String(), false)
	require.NoError(t, err)
	assert.Len(t, notifications, 1)
	assert.Equal(t, "Push Title", notifications[0].Title)
	assert.Equal(t, "Push Body", notifications[0].Description)
	assert.Equal(t, "push", notifications[0].Type)
}

func TestIntegration_NotificationWithData(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	// Create user
	userID := createTestUser(t, tdb, "data@test.com")

	// Create notification with data
	notification := &Notification{
		UserID: userID.String(),
		Title:  "With Data",
		Type:   "system",
		Level:  "info",
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		},
	}

	err := service.CreateNotification(ctx, notification)
	require.NoError(t, err)

	// Get notification and verify data
	result, err := service.GetNotification(ctx, userID.String(), notification.ID)
	require.NoError(t, err)
	assert.NotNil(t, result.Data)
	assert.Equal(t, "value1", result.Data["key1"])
}

func TestIntegration_InvalidNotificationID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	service := NewService(tdb.Queries)

	userID := createTestUser(t, tdb, "invalid@test.com")

	// Test with invalid notification ID
	_, err := service.GetNotification(ctx, userID.String(), "not-a-valid-uuid")
	assert.Error(t, err)

	err = service.MarkAsRead(ctx, userID.String(), "not-a-valid-uuid")
	assert.Error(t, err)

	err = service.DeleteNotification(ctx, userID.String(), "not-a-valid-uuid")
	assert.Error(t, err)
}
