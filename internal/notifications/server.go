package notifications

import (
	"context"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the NotificationsService
type Server struct {
	immichv1.UnimplementedNotificationsServiceServer
	service *Service
}

// NewServer creates a new notifications server
func NewServer(service *Service) *Server {
	return &Server{
		service: service,
	}
}

// GetNotifications returns notifications based on filters
func (s *Server) GetNotifications(ctx context.Context, req *immichv1.GetNotificationsRequest) (*immichv1.GetNotificationsResponse, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	unreadOnly := req.Unread != nil && *req.Unread
	notifications, err := s.service.GetNotifications(ctx, claims.UserID, unreadOnly)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get notifications: %v", err)
	}

	// Convert to proto notifications
	var protoNotifications []*immichv1.NotificationDto
	for _, notif := range notifications {
		// Convert notification type
		notifType := immichv1.NotificationType_NOTIFICATION_TYPE_SYSTEM_MESSAGE
		switch notif.Type {
		case "job_failed":
			notifType = immichv1.NotificationType_NOTIFICATION_TYPE_JOB_FAILED
		case "backup_failed":
			notifType = immichv1.NotificationType_NOTIFICATION_TYPE_BACKUP_FAILED
		case "custom":
			notifType = immichv1.NotificationType_NOTIFICATION_TYPE_CUSTOM
		}

		// Convert notification level
		level := immichv1.NotificationLevel_NOTIFICATION_LEVEL_INFO
		switch notif.Type {
		case "error":
			level = immichv1.NotificationLevel_NOTIFICATION_LEVEL_ERROR
		case "warning":
			level = immichv1.NotificationLevel_NOTIFICATION_LEVEL_WARNING
		case "success":
			level = immichv1.NotificationLevel_NOTIFICATION_LEVEL_SUCCESS
		}

		protoNotif := &immichv1.NotificationDto{
			Id:          notif.ID,
			Title:       notif.Title,
			Description: &notif.Description,
			Level:       level,
			Type:        notifType,
			CreatedAt:   timestamppb.New(notif.CreatedAt),
		}

		// Add data if present
		if notif.Data != nil && len(notif.Data) > 0 {
			dataStruct, err := structpb.NewStruct(notif.Data)
			if err == nil {
				protoNotif.Data = dataStruct
			}
		}

		// Add read time if read
		if notif.Read {
			protoNotif.ReadAt = timestamppb.New(time.Now())
		}

		// Apply filters if provided
		if req.Level != nil && *req.Level != protoNotif.Level {
			continue
		}
		if req.Type != nil && *req.Type != protoNotif.Type {
			continue
		}

		protoNotifications = append(protoNotifications, protoNotif)
	}

	return &immichv1.GetNotificationsResponse{
		Notifications: protoNotifications,
	}, nil
}

// GetNotification gets a single notification by ID
func (s *Server) GetNotification(ctx context.Context, req *immichv1.GetNotificationRequest) (*immichv1.NotificationDto, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Get the specific notification from service
	notification, err := s.service.GetNotification(ctx, claims.UserID, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get notification: %v", err)
	}

	if notification == nil {
		return nil, status.Error(codes.NotFound, "notification not found")
	}

	// Convert notification type
	notifType := immichv1.NotificationType_NOTIFICATION_TYPE_SYSTEM_MESSAGE
	switch notification.Type {
	case "job_failed":
		notifType = immichv1.NotificationType_NOTIFICATION_TYPE_JOB_FAILED
	case "backup_failed":
		notifType = immichv1.NotificationType_NOTIFICATION_TYPE_BACKUP_FAILED
	case "custom":
		notifType = immichv1.NotificationType_NOTIFICATION_TYPE_CUSTOM
	}

	// Convert notification level
	level := immichv1.NotificationLevel_NOTIFICATION_LEVEL_INFO
	switch notification.Level {
	case "error":
		level = immichv1.NotificationLevel_NOTIFICATION_LEVEL_ERROR
	case "warning":
		level = immichv1.NotificationLevel_NOTIFICATION_LEVEL_WARNING
	}

	// Build proto notification
	protoNotif := &immichv1.NotificationDto{
		Id:          notification.ID,
		Title:       notification.Title,
		Description: ptrString(notification.Description),
		Level:       level,
		Type:        notifType,
		CreatedAt:   timestamppb.New(notification.CreatedAt),
	}

	if notification.ReadAt != nil {
		protoNotif.ReadAt = timestamppb.New(*notification.ReadAt)
	}

	// Add data if available
	if notification.Data != nil {
		dataStruct, err := structpb.NewStruct(notification.Data)
		if err == nil {
			protoNotif.Data = dataStruct
		}
	}

	return protoNotif, nil
}

// UpdateNotification updates a notification (mainly to mark as read)
func (s *Server) UpdateNotification(ctx context.Context, req *immichv1.UpdateNotificationRequest) (*immichv1.NotificationDto, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Mark as read if read_at is provided
	if req.ReadAt != nil {
		err := s.service.MarkAsRead(ctx, claims.UserID, req.Id)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to mark as read: %v", err)
		}
	}

	// Retrieve the updated notification from service
	notification, err := s.service.GetNotification(ctx, claims.UserID, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get updated notification: %v", err)
	}

	if notification == nil {
		return nil, status.Error(codes.NotFound, "notification not found")
	}

	// Convert notification type
	notifType := immichv1.NotificationType_NOTIFICATION_TYPE_SYSTEM_MESSAGE
	switch notification.Type {
	case "job_failed":
		notifType = immichv1.NotificationType_NOTIFICATION_TYPE_JOB_FAILED
	case "backup_failed":
		notifType = immichv1.NotificationType_NOTIFICATION_TYPE_BACKUP_FAILED
	case "custom":
		notifType = immichv1.NotificationType_NOTIFICATION_TYPE_CUSTOM
	}

	// Convert notification level
	level := immichv1.NotificationLevel_NOTIFICATION_LEVEL_INFO
	switch notification.Level {
	case "error":
		level = immichv1.NotificationLevel_NOTIFICATION_LEVEL_ERROR
	case "warning":
		level = immichv1.NotificationLevel_NOTIFICATION_LEVEL_WARNING
	}

	// Build proto notification with actual data
	protoNotif := &immichv1.NotificationDto{
		Id:          notification.ID,
		Title:       notification.Title,
		Description: ptrString(notification.Description),
		Level:       level,
		Type:        notifType,
		CreatedAt:   timestamppb.New(notification.CreatedAt),
	}

	// Use the updated read time
	if req.ReadAt != nil {
		protoNotif.ReadAt = req.ReadAt
	} else if notification.ReadAt != nil {
		protoNotif.ReadAt = timestamppb.New(*notification.ReadAt)
	}

	// Add data if available
	if notification.Data != nil {
		dataStruct, err := structpb.NewStruct(notification.Data)
		if err == nil {
			protoNotif.Data = dataStruct
		}
	}

	return protoNotif, nil
}

// DeleteNotification deletes a notification
func (s *Server) DeleteNotification(ctx context.Context, req *immichv1.DeleteNotificationRequest) (*emptypb.Empty, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	err := s.service.DeleteNotification(ctx, claims.UserID, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete notification: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// UpdateNotifications updates multiple notifications
func (s *Server) UpdateNotifications(ctx context.Context, req *immichv1.UpdateNotificationsRequest) (*emptypb.Empty, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Mark all as read if read_at is provided
	if req.ReadAt != nil {
		if len(req.Ids) == 0 {
			// Mark all as read
			err := s.service.MarkAllAsRead(ctx, claims.UserID)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to mark all as read: %v", err)
			}
		} else {
			// Mark specific ones as read
			for _, id := range req.Ids {
				err := s.service.MarkAsRead(ctx, claims.UserID, id)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "failed to mark notification %s as read: %v", id, err)
				}
			}
		}
	}

	return &emptypb.Empty{}, nil
}

// DeleteNotifications deletes multiple notifications
func (s *Server) DeleteNotifications(ctx context.Context, req *immichv1.DeleteNotificationsRequest) (*emptypb.Empty, error) {
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	for _, id := range req.Ids {
		err := s.service.DeleteNotification(ctx, claims.UserID, id)
		if err != nil {
			// Log error but continue with other deletions
			continue
		}
	}

	return &emptypb.Empty{}, nil
}

// Helper function to create string pointer
func ptrString(s string) *string {
	return &s
}
