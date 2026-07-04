package admin

import (
	"context"
	"encoding/json"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/calendarheatmap"
	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/grpcutil"
	"github.com/denysvitali/immich-go-backend/internal/jobs"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the AdminService
type Server struct {
	immichv1.UnimplementedAdminServiceServer
	service    *Service
	jobService *jobs.Service
}

// NewServer creates a new admin server
func NewServer(service *Service, jobService *jobs.Service) *Server {
	return &Server{
		service:    service,
		jobService: jobService,
	}
}

// SendNotification sends notification to users
func (s *Server) SendNotification(ctx context.Context, request *immichv1.SendNotificationRequest) (*emptypb.Empty, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Convert request
	req := SendNotificationRequest{
		Message: request.GetMessage(),
		Subject: request.GetSubject(),
		UserIDs: request.GetUserIds(),
	}

	// Call service
	err = s.service.SendNotification(ctx, req)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to send notification", err)
	}

	return &emptypb.Empty{}, nil
}

// RenderNotificationTemplate renders a notification template
func (s *Server) RenderNotificationTemplate(ctx context.Context, request *immichv1.RenderNotificationTemplateRequest) (*immichv1.TemplateResponseDto, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Call service
	response, err := s.service.RenderNotificationTemplate(ctx, request.GetName(), request.GetData())
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to render template", err)
	}

	return &immichv1.TemplateResponseDto{
		Html:    response.HTML,
		Subject: response.Subject,
	}, nil
}

// TestEmailNotification tests email notification functionality
func (s *Server) TestEmailNotification(ctx context.Context, request *immichv1.TestEmailNotificationRequest) (*immichv1.TestEmailResponseDto, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Call service
	response, err := s.service.TestEmailNotification(ctx, request.GetRecipient())
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to test email notification", err)
	}

	return &immichv1.TestEmailResponseDto{
		Message: response.Message,
	}, nil
}

// SearchUsersAdmin searches for users (admin function)
func (s *Server) SearchUsersAdmin(ctx context.Context, request *immichv1.SearchUsersAdminRequest) (*immichv1.SearchUsersAdminResponse, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Convert request
	req := SearchUsersAdminRequest{}
	if request.Email != nil {
		email := request.GetEmail()
		req.Email = &email
	}
	if request.Name != nil {
		name := request.GetName()
		req.Name = &name
	}
	if request.WithDeleted != nil {
		withDeleted := request.GetWithDeleted()
		req.WithDeleted = &withDeleted
	}

	// Call service
	response, err := s.service.SearchUsersAdmin(ctx, req)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to search users", err)
	}

	// Convert response
	users := make([]*immichv1.UserAdminResponseDto, len(response.Users))
	for i, user := range response.Users {
		users[i] = s.convertToProtoUser(user)
	}

	return &immichv1.SearchUsersAdminResponse{
		Users: users,
	}, nil
}

// CreateUserAdmin creates a new user (admin function)
func (s *Server) CreateUserAdmin(ctx context.Context, request *immichv1.CreateUserAdminRequest) (*immichv1.UserAdminResponseDto, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Validate request
	if request.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if request.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if request.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}

	// Convert request
	req := CreateUserAdminRequest{
		Email:    request.GetEmail(),
		Name:     request.GetName(),
		Password: request.GetPassword(),
	}
	if request.QuotaSizeInBytes != nil {
		quota := request.GetQuotaSizeInBytes()
		req.QuotaSizeInBytes = &quota
	}
	if request.ShouldChangePassword != nil {
		shouldChange := request.GetShouldChangePassword()
		req.ShouldChangePassword = &shouldChange
	}
	if request.StorageLabel != nil {
		label := request.GetStorageLabel()
		req.StorageLabel = &label
	}

	// Call service
	response, err := s.service.CreateUserAdmin(ctx, req)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to create user", err)
	}

	return s.convertToProtoUser(response), nil
}

// GetUserAdmin retrieves a user by ID (admin function)
func (s *Server) GetUserAdmin(ctx context.Context, request *immichv1.GetUserAdminRequest) (*immichv1.UserAdminResponseDto, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Call service
	response, err := s.service.GetUserAdmin(ctx, request.GetId())
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to get user", err)
	}

	return s.convertToProtoUser(response), nil
}

// UpdateUserAdmin updates a user (admin function)
func (s *Server) UpdateUserAdmin(ctx context.Context, request *immichv1.UpdateUserAdminRequest) (*immichv1.UserAdminResponseDto, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Convert request
	req := UpdateUserAdminRequest{}
	if request.AvatarColor != nil {
		avatarColor := UserAvatarColor(request.GetAvatarColor())
		req.AvatarColor = &avatarColor
	}
	if request.Email != nil {
		email := request.GetEmail()
		req.Email = &email
	}
	if request.IsAdmin != nil {
		isAdmin := request.GetIsAdmin()
		req.IsAdmin = &isAdmin
	}
	if request.Name != nil {
		name := request.GetName()
		req.Name = &name
	}
	if request.Password != nil {
		password := request.GetPassword()
		req.Password = &password
	}
	if request.QuotaSizeInBytes != nil {
		quota := request.GetQuotaSizeInBytes()
		req.QuotaSizeInBytes = &quota
	}
	if request.ShouldChangePassword != nil {
		shouldChange := request.GetShouldChangePassword()
		req.ShouldChangePassword = &shouldChange
	}
	if request.StorageLabel != nil {
		label := request.GetStorageLabel()
		req.StorageLabel = &label
	}

	// Call service
	response, err := s.service.UpdateUserAdmin(ctx, request.GetId(), req)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to update user", err)
	}

	return s.convertToProtoUser(response), nil
}

// DeleteUserAdmin deletes a user (admin function)
func (s *Server) DeleteUserAdmin(ctx context.Context, request *immichv1.DeleteUserAdminRequest) (*immichv1.UserAdminResponseDto, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	force := false
	if request.Force != nil {
		force = request.GetForce()
	}

	// Call service
	response, err := s.service.DeleteUserAdmin(ctx, request.GetId(), force)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to delete user", err)
	}

	return s.convertToProtoUser(response), nil
}

// RestoreUserAdmin restores a soft-deleted user (admin function)
func (s *Server) RestoreUserAdmin(ctx context.Context, request *immichv1.RestoreUserAdminRequest) (*immichv1.UserAdminResponseDto, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Call service
	response, err := s.service.RestoreUserAdmin(ctx, request.GetId())
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to restore user", err)
	}

	return s.convertToProtoUser(response), nil
}

// GetUserStatisticsAdmin gets user statistics (admin function)
func (s *Server) GetUserStatisticsAdmin(ctx context.Context, request *immichv1.GetUserStatisticsAdminRequest) (*immichv1.UserStatisticsResponseDto, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Call service
	response, err := s.service.GetUserStatisticsAdmin(ctx, request.GetId())
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to get user statistics", err)
	}

	return &immichv1.UserStatisticsResponseDto{
		Photos: response.Photos,
		Usage:  response.Usage,
		Videos: response.Videos,
	}, nil
}

func (s *Server) GetUserCalendarHeatmapAdmin(ctx context.Context, request *immichv1.GetUserCalendarHeatmapAdminRequest) (*immichv1.CalendarHeatmapResponseDto, error) {
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	userID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}

	response, err := calendarheatmap.Get(
		ctx,
		s.service.db,
		pgtype.UUID{Bytes: userID, Valid: true},
		request.GetFrom(),
		request.GetTo(),
		request.GetType(),
	)
	if err != nil {
		if calendarheatmap.IsInvalidArgument(err) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, grpcutil.SanitizedInternal(ctx, "failed to get user calendar heatmap", err)
	}

	return response, nil
}

// GetUserPreferencesAdmin gets user preferences (admin function)
func (s *Server) GetUserPreferencesAdmin(ctx context.Context, request *immichv1.GetUserPreferencesAdminRequest) (*immichv1.UserPreferencesResponseDto, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Parse user ID
	userID, err := uuid.Parse(request.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Get preferences from database
	prefsData, err := s.service.db.GetUserPreferencesData(ctx, userUUID)
	if err != nil {
		// If no preferences found, return empty preferences
		return &immichv1.UserPreferencesResponseDto{}, nil
	}

	// Parse JSON preferences data
	var prefs immichv1.UserPreferencesResponseDto
	if err := json.Unmarshal(prefsData, &prefs); err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to parse preferences", err)
	}

	return &prefs, nil
}

// UpdateUserPreferencesAdmin updates user preferences (admin function)
func (s *Server) UpdateUserPreferencesAdmin(ctx context.Context, request *immichv1.UpdateUserPreferencesAdminRequest) (*immichv1.UserPreferencesResponseDto, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Parse user ID
	userID, err := uuid.Parse(request.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Serialize preferences to JSON
	prefsData, err := json.Marshal(request.GetPreferences())
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to serialize preferences", err)
	}

	// Update preferences in database
	updatedData, err := s.service.db.UpdateUserPreferencesData(ctx, sqlc.UpdateUserPreferencesDataParams{
		UserId: userUUID,
		Value:  prefsData,
	})
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to update preferences", err)
	}

	// Parse updated preferences
	var updatedPrefs immichv1.UserPreferencesResponseDto
	if err := json.Unmarshal(updatedData, &updatedPrefs); err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to parse updated preferences", err)
	}

	return &updatedPrefs, nil
}

// Helper function to convert service user to proto user
func (s *Server) convertToProtoUser(user *UserAdminResponseDto) *immichv1.UserAdminResponseDto {
	protoUser := &immichv1.UserAdminResponseDto{
		AvatarColor:          immichv1.UserAvatarColor(user.AvatarColor),
		CreatedAt:            timestamppb.New(user.CreatedAt),
		Email:                user.Email,
		Id:                   user.ID,
		IsAdmin:              user.IsAdmin,
		Name:                 user.Name,
		OauthId:              user.OauthID,
		ProfileImagePath:     user.ProfileImagePath,
		ShouldChangePassword: user.ShouldChangePassword,
		UpdatedAt:            timestamppb.New(user.UpdatedAt),
	}

	if user.DeletedAt != nil {
		protoUser.DeletedAt = timestamppb.New(*user.DeletedAt)
	}

	if user.ProfileChangedAt != nil {
		protoUser.ProfileChangedAt = timestamppb.New(*user.ProfileChangedAt)
	}

	if user.QuotaSizeInBytes != nil {
		protoUser.QuotaSizeInBytes = *user.QuotaSizeInBytes
	}

	if user.StorageLabel != nil {
		protoUser.StorageLabel = *user.StorageLabel
	}

	return protoUser
}

// GetUserSessionsAdmin gets user sessions (admin function)
func (s *Server) GetUserSessionsAdmin(ctx context.Context, request *immichv1.GetUserSessionsAdminRequest) (*immichv1.GetUserSessionsAdminResponse, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Parse user ID
	userID, err := uuid.Parse(request.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Get sessions from database
	sessions, err := s.service.db.GetUserSessions(ctx, userUUID)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to get user sessions", err)
	}

	// Convert to proto
	var protoSessions []*immichv1.SessionResponseDto
	for _, session := range sessions {
		protoSession := &immichv1.SessionResponseDto{
			Id:        pgutil.UUIDToString(session.ID),
			CreatedAt: timestamppb.New(session.CreatedAt.Time),
			UpdatedAt: timestamppb.New(session.UpdatedAt.Time),
			Current:   false, // Admin viewing, so not their session
		}
		protoSession.DeviceType = session.DeviceType
		protoSession.DeviceOs = session.DeviceOS
		protoSessions = append(protoSessions, protoSession)
	}

	return &immichv1.GetUserSessionsAdminResponse{
		Sessions: protoSessions,
	}, nil
}

// DeleteUserSessionAdmin deletes a user session (admin function)
func (s *Server) DeleteUserSessionAdmin(ctx context.Context, request *immichv1.DeleteUserSessionAdminRequest) (*emptypb.Empty, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Parse session ID
	sessionID, err := uuid.Parse(request.GetSessionId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid session ID: %v", err)
	}
	sessionUUID := pgtype.UUID{Bytes: sessionID, Valid: true}

	// Delete session
	err = s.service.db.DeleteSession(ctx, sessionUUID)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to delete session", err)
	}

	return &emptypb.Empty{}, nil
}

// UnlinkAllOAuthAccounts unlinks all OAuth accounts (admin function)
func (s *Server) UnlinkAllOAuthAccounts(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	// Require admin privileges
	_, err := auth.RequireAdmin(ctx)
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	// Clear all OAuth IDs from users
	err = s.service.db.ClearAllOAuthIds(ctx)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to unlink OAuth accounts", err)
	}

	return &emptypb.Empty{}, nil
}

// ListDeadLetterJobs returns failed jobs that exhausted retries or were
// marked non-retryable.
func (s *Server) ListDeadLetterJobs(ctx context.Context, _ *emptypb.Empty) (*immichv1.DeadLetterJobListResponseDto, error) {
	// Require admin privileges
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	if s.jobService == nil {
		return nil, status.Error(codes.Unavailable, "job service is not available")
	}

	jobs, err := s.jobService.ListDeadLetterJobs(ctx, 1000, 0)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to list dead-letter jobs", err)
	}

	protoJobs := make([]*immichv1.DeadLetterJobDto, len(jobs))
	for i, job := range jobs {
		protoJobs[i] = &immichv1.DeadLetterJobDto{
			Id:           job.ID,
			Queue:        job.Queue,
			JobType:      job.JobType,
			Payload:      job.Payload,
			Error:        job.Error,
			MaxRetries:   int32(job.MaxRetries),
			RetriedCount: int32(job.RetriedCount),
			FailedAt:     timestamppb.New(job.FailedAt),
			LastFailedAt: timestamppb.New(job.LastFailedAt),
		}
	}

	return &immichv1.DeadLetterJobListResponseDto{Jobs: protoJobs}, nil
}

// RetryDeadLetterJob re-enqueues a single dead-letter job.
func (s *Server) RetryDeadLetterJob(ctx context.Context, request *immichv1.RetryDeadLetterJobRequest) (*emptypb.Empty, error) {
	// Require admin privileges
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	if s.jobService == nil {
		return nil, status.Error(codes.Unavailable, "job service is not available")
	}

	if err := s.jobService.RetryDeadLetterJob(ctx, request.GetId()); err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to retry dead-letter job", err)
	}

	return &emptypb.Empty{}, nil
}
