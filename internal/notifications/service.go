package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
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
	// CRITICAL: SQLC regeneration required - run 'make sqlc-gen' in Nix environment
	// The GetNotifications query exists in sqlc/queries.sql but hasn't been generated
	// This implementation cannot access the database until SQLC is regenerated
	// According to CLAUDE.md: NO MOCKS OR STUBS ALLOWED
	return nil, fmt.Errorf("notifications service requires SQLC regeneration: run 'make sqlc-gen' in Nix environment")
}

func (s *Service) MarkAsRead(ctx context.Context, userID string, notificationID string) error {
	// CRITICAL: SQLC regeneration required
	// The MarkNotificationAsRead query exists in sqlc/queries.sql but hasn't been generated
	return fmt.Errorf("notifications service requires SQLC regeneration: run 'make sqlc-gen' in Nix environment")
}

func (s *Service) MarkAllAsRead(ctx context.Context, userID string) error {
	// CRITICAL: SQLC regeneration required
	// This needs GetNotifications and MarkNotificationAsRead queries from SQLC
	return fmt.Errorf("notifications service requires SQLC regeneration: run 'make sqlc-gen' in Nix environment")
}

func (s *Service) DeleteNotification(ctx context.Context, userID string, notificationID string) error {
	// CRITICAL: SQLC regeneration required
	// The DeleteNotification query exists in sqlc/queries.sql but hasn't been generated
	return fmt.Errorf("notifications service requires SQLC regeneration: run 'make sqlc-gen' in Nix environment")
}

func (s *Service) CreateNotification(ctx context.Context, notification *Notification) error {
	// CRITICAL: SQLC regeneration required
	// The CreateNotification query exists in sqlc/queries.sql but hasn't been generated
	// Cannot create fake data as per CLAUDE.md: NO MOCKS OR STUBS ALLOWED
	return fmt.Errorf("notifications service requires SQLC regeneration: run 'make sqlc-gen' in Nix environment")
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

func (s *Service) GetNotification(ctx context.Context, userID string, notificationID string) (*Notification, error) {
	// CRITICAL: SQLC regeneration required
	// The GetNotification query exists in sqlc/queries.sql but hasn't been generated
	return nil, fmt.Errorf("notifications service requires SQLC regeneration: run 'make sqlc-gen' in Nix environment")
}

func (s *Service) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	// CRITICAL: SQLC regeneration required
	// The CountUnreadNotifications query exists in sqlc/queries.sql but hasn't been generated
	return 0, fmt.Errorf("notifications service requires SQLC regeneration: run 'make sqlc-gen' in Nix environment")
}