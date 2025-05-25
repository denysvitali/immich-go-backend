package server

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

// Mock notification data for demonstration
var mockNotifications = []*immichv1.NotificationDto{
	{
		Id:          "550e8400-e29b-41d4-a716-446655440001",
		Title:       "Backup completed successfully",
		Description: stringPtr("Your photo backup has been completed with 150 new photos"),
		Level:       immichv1.NotificationLevel_NOTIFICATION_LEVEL_SUCCESS,
		Type:        immichv1.NotificationType_NOTIFICATION_TYPE_BACKUP_FAILED,
		CreatedAt:   timestamppb.New(time.Now().Add(-2 * time.Hour)),
		ReadAt:      timestamppb.New(time.Now().Add(-1 * time.Hour)),
		Data: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"photoCount": structpb.NewNumberValue(150),
				"albumId":    structpb.NewStringValue("album-123"),
			},
		},
	},
	{
		Id:          "550e8400-e29b-41d4-a716-446655440002",
		Title:       "Backup failed",
		Description: stringPtr("Failed to backup 5 photos due to insufficient storage"),
		Level:       immichv1.NotificationLevel_NOTIFICATION_LEVEL_ERROR,
		Type:        immichv1.NotificationType_NOTIFICATION_TYPE_BACKUP_FAILED,
		CreatedAt:   timestamppb.New(time.Now().Add(-30 * time.Minute)),
		ReadAt:      nil, // Unread
		Data: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"failedCount": structpb.NewNumberValue(5),
				"reason":      structpb.NewStringValue("insufficient_storage"),
			},
		},
	},
	{
		Id:          "550e8400-e29b-41d4-a716-446655440003",
		Title:       "System maintenance scheduled",
		Description: stringPtr("The system will be under maintenance tomorrow from 2-4 AM UTC"),
		Level:       immichv1.NotificationLevel_NOTIFICATION_LEVEL_INFO,
		Type:        immichv1.NotificationType_NOTIFICATION_TYPE_SYSTEM_MESSAGE,
		CreatedAt:   timestamppb.New(time.Now().Add(-1 * time.Hour)),
		ReadAt:      nil, // Unread
		Data: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"maintenanceStart": structpb.NewStringValue("2025-05-26T02:00:00Z"),
				"maintenanceEnd":   structpb.NewStringValue("2025-05-26T04:00:00Z"),
			},
		},
	},
}

func stringPtr(s string) *string {
	return &s
}

func (s *Server) GetNotifications(ctx context.Context, req *immichv1.GetNotificationsRequest) (*immichv1.GetNotificationsResponse, error) {
	var filteredNotifications []*immichv1.NotificationDto

	for _, notification := range mockNotifications {
		// Apply filters
		if req.Id != nil && *req.Id != notification.Id {
			continue
		}
		if req.Level != nil && *req.Level != notification.Level {
			continue
		}
		if req.Type != nil && *req.Type != notification.Type {
			continue
		}
		if req.Unread != nil && *req.Unread {
			if notification.ReadAt != nil {
				continue // Skip read notifications when unread filter is true
			}
		}

		filteredNotifications = append(filteredNotifications, notification)
	}

	return &immichv1.GetNotificationsResponse{
		Notifications: filteredNotifications,
	}, nil
}

func (s *Server) GetNotification(ctx context.Context, req *immichv1.GetNotificationRequest) (*immichv1.NotificationDto, error) {
	for _, notification := range mockNotifications {
		if notification.Id == req.Id {
			return notification, nil
		}
	}

	return nil, status.Error(codes.NotFound, "notification not found")
}

func (s *Server) UpdateNotification(ctx context.Context, req *immichv1.UpdateNotificationRequest) (*immichv1.NotificationDto, error) {
	for _, notification := range mockNotifications {
		if notification.Id == req.Id {
			// Update the read status
			if req.ReadAt != nil {
				notification.ReadAt = req.ReadAt
			} else {
				// If ReadAt is nil, mark as unread
				notification.ReadAt = nil
			}
			return notification, nil
		}
	}

	return nil, status.Error(codes.NotFound, "notification not found")
}

func (s *Server) DeleteNotification(ctx context.Context, req *immichv1.DeleteNotificationRequest) (*emptypb.Empty, error) {
	for i, notification := range mockNotifications {
		if notification.Id == req.Id {
			// Remove notification from slice
			mockNotifications = append(mockNotifications[:i], mockNotifications[i+1:]...)
			return &emptypb.Empty{}, nil
		}
	}

	return nil, status.Error(codes.NotFound, "notification not found")
}

func (s *Server) UpdateNotifications(ctx context.Context, req *immichv1.UpdateNotificationsRequest) (*emptypb.Empty, error) {
	if len(req.Ids) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no notification IDs provided")
	}

	updatedCount := 0
	for _, notification := range mockNotifications {
		for _, id := range req.Ids {
			if notification.Id == id {
				if req.ReadAt != nil {
					notification.ReadAt = req.ReadAt
				} else {
					// If ReadAt is nil, mark as unread
					notification.ReadAt = nil
				}
				updatedCount++
				break
			}
		}
	}

	if updatedCount == 0 {
		return nil, status.Error(codes.NotFound, "no notifications found with provided IDs")
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) DeleteNotifications(ctx context.Context, req *immichv1.DeleteNotificationsRequest) (*emptypb.Empty, error) {
	if len(req.Ids) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no notification IDs provided")
	}

	idsToDelete := make(map[string]bool)
	for _, id := range req.Ids {
		idsToDelete[id] = true
	}

	var remainingNotifications []*immichv1.NotificationDto
	deletedCount := 0

	for _, notification := range mockNotifications {
		if idsToDelete[notification.Id] {
			deletedCount++
		} else {
			remainingNotifications = append(remainingNotifications, notification)
		}
	}

	if deletedCount == 0 {
		return nil, status.Error(codes.NotFound, "no notifications found with provided IDs")
	}

	mockNotifications = remainingNotifications
	return &emptypb.Empty{}, nil
}
