package notifications

import (
	"context"
	"errors"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/grpcutil"
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

var notificationTypesToProto = map[string]immichv1.NotificationType{
	"job_failed":    immichv1.NotificationType_NOTIFICATION_TYPE_JOB_FAILED,
	"backup_failed": immichv1.NotificationType_NOTIFICATION_TYPE_BACKUP_FAILED,
	"custom":        immichv1.NotificationType_NOTIFICATION_TYPE_CUSTOM,
}

var notificationLevelsToProto = map[string]immichv1.NotificationLevel{
	"success": immichv1.NotificationLevel_NOTIFICATION_LEVEL_SUCCESS,
	"error":   immichv1.NotificationLevel_NOTIFICATION_LEVEL_ERROR,
	"warning": immichv1.NotificationLevel_NOTIFICATION_LEVEL_WARNING,
	"info":    immichv1.NotificationLevel_NOTIFICATION_LEVEL_INFO,
}

func notificationTypeToProto(notificationType string) immichv1.NotificationType {
	if protoType, ok := notificationTypesToProto[notificationType]; ok {
		return protoType
	}

	return immichv1.NotificationType_NOTIFICATION_TYPE_SYSTEM_MESSAGE
}

func notificationLevelToProto(level string) immichv1.NotificationLevel {
	if protoLevel, ok := notificationLevelsToProto[level]; ok {
		return protoLevel
	}

	return immichv1.NotificationLevel_NOTIFICATION_LEVEL_INFO
}

func notificationToProto(notification *Notification) *immichv1.NotificationDto {
	protoNotif := &immichv1.NotificationDto{
		Id:          notification.ID,
		Title:       notification.Title,
		Description: &notification.Description,
		Level:       notificationLevelToProto(notification.Level),
		Type:        notificationTypeToProto(notification.Type),
		CreatedAt:   timestamppb.New(notification.CreatedAt),
	}

	if notification.ReadAt != nil {
		protoNotif.ReadAt = timestamppb.New(*notification.ReadAt)
	} else if notification.Read {
		protoNotif.ReadAt = timestamppb.New(time.Now())
	}

	if len(notification.Data) > 0 {
		dataStruct, err := structpb.NewStruct(notification.Data)
		if err == nil {
			protoNotif.Data = dataStruct
		}
	}

	return protoNotif
}

func notificationStatusError(ctx context.Context, publicMsg string, err error) error {
	switch {
	case errors.Is(err, ErrInvalidUserID), errors.Is(err, ErrInvalidNotificationID):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ErrAccessDenied):
		return status.Error(codes.PermissionDenied, "access denied")
	case errors.Is(err, ErrNotificationNotFound):
		return status.Error(codes.NotFound, "notification not found")
	default:
		return grpcutil.SanitizedInternal(ctx, publicMsg, err)
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
		return nil, notificationStatusError(ctx, "failed to get notifications", err)
	}

	// Convert to proto notifications
	var protoNotifications []*immichv1.NotificationDto
	for _, notif := range notifications {
		protoNotif := notificationToProto(notif)

		// Apply filters if provided
		if req.Id != nil && *req.Id != protoNotif.Id {
			continue
		}
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
		return nil, notificationStatusError(ctx, "failed to get notification", err)
	}

	if notification == nil {
		return nil, status.Error(codes.NotFound, "notification not found")
	}

	return notificationToProto(notification), nil
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
			return nil, notificationStatusError(ctx, "failed to mark as read", err)
		}
	}

	// Retrieve the updated notification from service
	notification, err := s.service.GetNotification(ctx, claims.UserID, req.Id)
	if err != nil {
		return nil, notificationStatusError(ctx, "failed to get updated notification", err)
	}

	if notification == nil {
		return nil, status.Error(codes.NotFound, "notification not found")
	}

	protoNotif := notificationToProto(notification)

	// Use the updated read time
	if req.ReadAt != nil {
		protoNotif.ReadAt = req.ReadAt
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
		return nil, notificationStatusError(ctx, "failed to delete notification", err)
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
				return nil, notificationStatusError(ctx, "failed to mark all as read", err)
			}
		} else {
			// Mark specific ones as read
			for _, id := range req.Ids {
				err := s.service.MarkAsRead(ctx, claims.UserID, id)
				if err != nil {
					return nil, notificationStatusError(ctx, "failed to mark notification as read", err)
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
			return nil, notificationStatusError(ctx, "failed to delete notification", err)
		}
	}

	return &emptypb.Empty{}, nil
}
