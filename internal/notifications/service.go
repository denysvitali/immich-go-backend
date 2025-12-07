package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	queries *sqlc.Queries
}

func NewService(queries *sqlc.Queries) *Service {
	return &Service{
		queries: queries,
	}
}

type Notification struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"userId"`
	Type        string                 `json:"type"`
	Level       string                 `json:"level"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Read        bool                   `json:"read"`
	ReadAt      *time.Time             `json:"readAt,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

func (s *Service) GetNotifications(ctx context.Context, userID string, unreadOnly bool) ([]*Notification, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Get notifications from database
	dbNotifications, err := s.queries.GetNotifications(ctx, sqlc.GetNotificationsParams{
		UserId: userUUID,
		Limit:  100,
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get notifications: %w", err)
	}

	notifications := make([]*Notification, 0, len(dbNotifications))
	for _, dbN := range dbNotifications {
		// Filter unread if requested
		isRead := dbN.ReadAt.Valid
		if unreadOnly && isRead {
			continue
		}

		// Parse JSON data
		var data map[string]interface{}
		if len(dbN.Data) > 0 {
			if err := json.Unmarshal(dbN.Data, &data); err != nil {
				data = make(map[string]interface{})
			}
		}

		var readAt *time.Time
		if dbN.ReadAt.Valid {
			readAt = &dbN.ReadAt.Time
		}

		var description string
		if dbN.Description.Valid {
			description = dbN.Description.String
		}

		notifications = append(notifications, &Notification{
			ID:          uuid.UUID(dbN.ID.Bytes).String(),
			UserID:      userID,
			Type:        dbN.Type,
			Level:       string(dbN.Level),
			Title:       dbN.Title,
			Description: description,
			Data:        data,
			Read:        isRead,
			ReadAt:      readAt,
			CreatedAt:   dbN.CreatedAt.Time,
			UpdatedAt:   dbN.UpdatedAt.Time,
		})
	}

	return notifications, nil
}

func (s *Service) GetNotification(ctx context.Context, userID string, notificationID string) (*Notification, error) {
	nid, err := uuid.Parse(notificationID)
	if err != nil {
		return nil, fmt.Errorf("invalid notification ID: %w", err)
	}
	notifUUID := pgtype.UUID{Bytes: nid, Valid: true}

	dbN, err := s.queries.GetNotification(ctx, notifUUID)
	if err != nil {
		return nil, fmt.Errorf("notification not found: %w", err)
	}

	// Verify ownership
	if uuid.UUID(dbN.UserId.Bytes).String() != userID {
		return nil, fmt.Errorf("access denied: notification does not belong to user")
	}

	// Parse JSON data
	var data map[string]interface{}
	if len(dbN.Data) > 0 {
		if err := json.Unmarshal(dbN.Data, &data); err != nil {
			data = make(map[string]interface{})
		}
	}

	var readAt *time.Time
	if dbN.ReadAt.Valid {
		readAt = &dbN.ReadAt.Time
	}

	var description string
	if dbN.Description.Valid {
		description = dbN.Description.String
	}

	return &Notification{
		ID:          notificationID,
		UserID:      userID,
		Type:        dbN.Type,
		Level:       string(dbN.Level),
		Title:       dbN.Title,
		Description: description,
		Data:        data,
		Read:        dbN.ReadAt.Valid,
		ReadAt:      readAt,
		CreatedAt:   dbN.CreatedAt.Time,
		UpdatedAt:   dbN.UpdatedAt.Time,
	}, nil
}

func (s *Service) MarkAsRead(ctx context.Context, userID string, notificationID string) error {
	nid, err := uuid.Parse(notificationID)
	if err != nil {
		return fmt.Errorf("invalid notification ID: %w", err)
	}
	notifUUID := pgtype.UUID{Bytes: nid, Valid: true}

	// Verify ownership first
	dbN, err := s.queries.GetNotification(ctx, notifUUID)
	if err != nil {
		return fmt.Errorf("notification not found: %w", err)
	}

	if uuid.UUID(dbN.UserId.Bytes).String() != userID {
		return fmt.Errorf("access denied: notification does not belong to user")
	}

	// Mark as read
	_, err = s.queries.MarkNotificationAsRead(ctx, notifUUID)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	return nil
}

func (s *Service) MarkAllAsRead(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Get all unread notifications
	dbNotifications, err := s.queries.GetNotifications(ctx, sqlc.GetNotificationsParams{
		UserId: userUUID,
		Limit:  1000, // Process up to 1000 at a time
		Offset: 0,
	})
	if err != nil {
		return fmt.Errorf("failed to get notifications: %w", err)
	}

	// Mark each unread notification as read
	for _, dbN := range dbNotifications {
		if !dbN.ReadAt.Valid {
			_, err = s.queries.MarkNotificationAsRead(ctx, dbN.ID)
			if err != nil {
				// Continue with other notifications even if one fails
				continue
			}
		}
	}

	return nil
}

func (s *Service) DeleteNotification(ctx context.Context, userID string, notificationID string) error {
	nid, err := uuid.Parse(notificationID)
	if err != nil {
		return fmt.Errorf("invalid notification ID: %w", err)
	}
	notifUUID := pgtype.UUID{Bytes: nid, Valid: true}

	// Verify ownership first
	dbN, err := s.queries.GetNotification(ctx, notifUUID)
	if err != nil {
		return fmt.Errorf("notification not found: %w", err)
	}

	if uuid.UUID(dbN.UserId.Bytes).String() != userID {
		return fmt.Errorf("access denied: notification does not belong to user")
	}

	// Delete notification (soft delete)
	err = s.queries.DeleteNotification(ctx, notifUUID)
	if err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	return nil
}

func (s *Service) CreateNotification(ctx context.Context, notification *Notification) error {
	uid, err := uuid.Parse(notification.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Prepare JSON data
	var jsonData []byte
	if notification.Data != nil {
		jsonData, err = json.Marshal(notification.Data)
		if err != nil {
			jsonData = []byte("{}")
		}
	} else {
		jsonData = []byte("{}")
	}

	// Set default level if not provided
	level := notification.Level
	if level == "" {
		level = "info"
	}

	// Set default type if not provided
	notifType := notification.Type
	if notifType == "" {
		notifType = "system"
	}

	// Create notification in database
	dbN, err := s.queries.CreateNotification(ctx, sqlc.CreateNotificationParams{
		UserId:      userUUID,
		Level:       level,
		Type:        notifType,
		Data:        jsonData,
		Title:       notification.Title,
		Description: pgtype.Text{String: notification.Description, Valid: notification.Description != ""},
	})
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	// Update the notification with the generated ID
	notification.ID = uuid.UUID(dbN.ID.Bytes).String()
	notification.CreatedAt = dbN.CreatedAt.Time
	notification.UpdatedAt = dbN.UpdatedAt.Time

	return nil
}

func (s *Service) SendPushNotification(ctx context.Context, userID string, title, body string) error {
	// This would integrate with FCM/APNs for push notifications
	// For now, just create a notification in the database
	notification := &Notification{
		UserID:      userID,
		Type:        "push",
		Level:       "info",
		Title:       title,
		Description: body,
		Read:        false,
	}

	return s.CreateNotification(ctx, notification)
}

func (s *Service) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	count, err := s.queries.CountUnreadNotifications(ctx, userUUID)
	if err != nil {
		return 0, fmt.Errorf("failed to count unread notifications: %w", err)
	}

	return int(count), nil
}
