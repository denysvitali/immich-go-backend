package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) GetNotifications(ctx context.Context, req *immichv1.GetNotificationsRequest) (*immichv1.GetNotificationsResponse, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Get notifications from notifications service
	notifications, err := s.notificationsService.GetNotifications(ctx, claims.UserID, false)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get notifications: %v", err)
	}

	// Convert to proto notifications
	// For now, return empty since we need SQLC regeneration
	protoNotifications := make([]*immichv1.NotificationDto, len(notifications))
	_ = notifications // Used to convert once SQLC is regenerated

	return &immichv1.GetNotificationsResponse{
		Notifications: protoNotifications,
	}, nil
}

func (s *Server) GetNotification(ctx context.Context, req *immichv1.GetNotificationRequest) (*immichv1.NotificationDto, error) {
	// Get user from context
	_, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Would fetch specific notification from database
	// For now, return not found since we need SQLC regeneration
	return nil, status.Error(codes.NotFound, "notification not found")
}

func (s *Server) UpdateNotification(ctx context.Context, req *immichv1.UpdateNotificationRequest) (*immichv1.NotificationDto, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Mark notification as read
	if req.ReadAt != nil {
		err := s.notificationsService.MarkAsRead(ctx, claims.UserID, req.Id)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update notification: %v", err)
		}
	}

	// Return updated notification
	// For now, return not found since we need SQLC regeneration
	return nil, status.Error(codes.NotFound, "notification not found")
}

func (s *Server) DeleteNotification(ctx context.Context, req *immichv1.DeleteNotificationRequest) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Delete notification
	err := s.notificationsService.DeleteNotification(ctx, claims.UserID, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete notification: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// UpdateNotifications updates multiple notifications at once
func (s *Server) UpdateNotifications(ctx context.Context, req *immichv1.UpdateNotificationsRequest) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// If marking all as read
	if req.GetReadAt() != nil {
		err := s.notificationsService.MarkAllAsRead(ctx, claims.UserID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update notifications: %v", err)
		}
	}

	return &emptypb.Empty{}, nil
}

// DeleteNotifications deletes multiple notifications
func (s *Server) DeleteNotifications(ctx context.Context, req *immichv1.DeleteNotificationsRequest) (*emptypb.Empty, error) {
	// Get user from context
	claims, ok := auth.GetClaimsFromStdContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthorized")
	}

	// Delete all notifications for user
	// This would need proper implementation once SQLC is regenerated
	_ = claims.UserID
	_ = req

	return &emptypb.Empty{}, nil
}
