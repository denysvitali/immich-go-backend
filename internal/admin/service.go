package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var tracer = telemetry.GetTracer("admin")

// Service handles administrative operations
type Service struct {
	db     *sqlc.Queries
	config *config.Config

	// Metrics
	operationCounter  metric.Int64Counter
	operationDuration metric.Float64Histogram
}

// NewService creates a new admin service
func NewService(queries *sqlc.Queries, cfg *config.Config) (*Service, error) {
	meter := telemetry.GetMeter()

	operationCounter, err := meter.Int64Counter(
		"admin_operations_total",
		metric.WithDescription("Total number of admin operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation counter: %w", err)
	}

	operationDuration, err := meter.Float64Histogram(
		"admin_operation_duration_seconds",
		metric.WithDescription("Time spent on admin operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation duration histogram: %w", err)
	}

	return &Service{
		db:                queries,
		config:            cfg,
		operationCounter:  operationCounter,
		operationDuration: operationDuration,
	}, nil
}

// SendNotification sends notification to users
func (s *Service) SendNotification(ctx context.Context, req SendNotificationRequest) error {
	ctx, span := tracer.Start(ctx, "admin.send_notification",
		trace.WithAttributes(
			attribute.String("subject", req.Subject),
			attribute.Int("user_count", len(req.UserIDs)),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "send_notification")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "send_notification")))
	}()

	// TODO: Implement actual notification sending when notification system is available
	// This would involve:
	// 1. Validating user IDs exist
	// 2. Creating notification records
	// 3. Sending via configured channels (email, push, etc.)
	return nil
}

// RenderNotificationTemplate renders a notification template
func (s *Service) RenderNotificationTemplate(ctx context.Context, name string, data map[string]string) (*TemplateResponseDto, error) {
	ctx, span := tracer.Start(ctx, "admin.render_notification_template",
		trace.WithAttributes(attribute.String("template_name", name)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "render_notification_template")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "render_notification_template")))
	}()

	// TODO: Implement actual template rendering when template system is available
	// For now, return a mock response
	return &TemplateResponseDto{
		HTML:    "<h1>Test Template</h1><p>This is a test template.</p>",
		Subject: "Test Subject",
	}, nil
}

// TestEmailNotification tests email notification functionality
func (s *Service) TestEmailNotification(ctx context.Context, recipient string) (*TestEmailResponseDto, error) {
	ctx, span := tracer.Start(ctx, "admin.test_email_notification",
		trace.WithAttributes(attribute.String("recipient", recipient)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "test_email_notification")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "test_email_notification")))
	}()

	// TODO: Implement actual email testing when email system is available
	// For now, return a mock response
	return &TestEmailResponseDto{
		Message: "Test email would be sent to " + recipient,
	}, nil
}

// SearchUsersAdmin searches for users (admin function)
func (s *Service) SearchUsersAdmin(ctx context.Context, req SearchUsersAdminRequest) (*SearchUsersAdminResponse, error) {
	ctx, span := tracer.Start(ctx, "admin.search_users_admin")
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "search_users_admin")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "search_users_admin")))
	}()

	// TODO: Implement actual user search when SQLC queries are available
	// This should search users with optional filters for email, name, and deleted status
	// For now, return empty response
	return &SearchUsersAdminResponse{
		Users: []*UserAdminResponseDto{},
	}, nil
}

// CreateUserAdmin creates a new user (admin function)
func (s *Service) CreateUserAdmin(ctx context.Context, req CreateUserAdminRequest) (*UserAdminResponseDto, error) {
	ctx, span := tracer.Start(ctx, "admin.create_user_admin",
		trace.WithAttributes(
			attribute.String("email", req.Email),
			attribute.String("name", req.Name),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "create_user_admin")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "create_user_admin")))
	}()

	// TODO: Implement actual user creation when SQLC queries are available
	// This should validate input and create user with proper password hashing
	// For now, return a mock response
	userID := uuid.New()
	now := time.Now()

	return &UserAdminResponseDto{
		AvatarColor:          UserAvatarColor_PRIMARY,
		CreatedAt:            now,
		DeletedAt:            nil,
		Email:                req.Email,
		ID:                   userID.String(),
		IsAdmin:              false,
		Name:                 req.Name,
		OauthID:              "",
		ProfileImagePath:     "",
		ProfileChangedAt:     nil,
		QuotaSizeInBytes:     req.QuotaSizeInBytes,
		ShouldChangePassword: req.ShouldChangePassword != nil && *req.ShouldChangePassword,
		StorageLabel:         req.StorageLabel,
		UpdatedAt:            now,
	}, nil
}

// GetUserAdmin retrieves a user by ID (admin function)
func (s *Service) GetUserAdmin(ctx context.Context, userID string) (*UserAdminResponseDto, error) {
	ctx, span := tracer.Start(ctx, "admin.get_user_admin",
		trace.WithAttributes(attribute.String("user_id", userID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_user_admin")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_user_admin")))
	}()

	// TODO: Implement actual user retrieval when SQLC queries are available
	// For now, return a mock response
	now := time.Now()

	return &UserAdminResponseDto{
		AvatarColor:          UserAvatarColor_PRIMARY,
		CreatedAt:            now,
		DeletedAt:            nil,
		Email:                "user@example.com",
		ID:                   userID,
		IsAdmin:              false,
		Name:                 "Test User",
		OauthID:              "",
		ProfileImagePath:     "",
		ProfileChangedAt:     nil,
		QuotaSizeInBytes:     nil,
		ShouldChangePassword: false,
		StorageLabel:         nil,
		UpdatedAt:            now,
	}, nil
}

// UpdateUserAdmin updates a user (admin function)
func (s *Service) UpdateUserAdmin(ctx context.Context, userID string, req UpdateUserAdminRequest) (*UserAdminResponseDto, error) {
	ctx, span := tracer.Start(ctx, "admin.update_user_admin",
		trace.WithAttributes(attribute.String("user_id", userID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "update_user_admin")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "update_user_admin")))
	}()

	// TODO: Implement actual user update when SQLC queries are available
	// For now, return a mock response
	now := time.Now()

	response := &UserAdminResponseDto{
		CreatedAt:            now,
		DeletedAt:            nil,
		ID:                   userID,
		IsAdmin:              req.IsAdmin != nil && *req.IsAdmin,
		OauthID:              "",
		ProfileImagePath:     "",
		ProfileChangedAt:     nil,
		QuotaSizeInBytes:     req.QuotaSizeInBytes,
		ShouldChangePassword: req.ShouldChangePassword != nil && *req.ShouldChangePassword,
		StorageLabel:         req.StorageLabel,
		UpdatedAt:            now,
	}

	if req.AvatarColor != nil {
		response.AvatarColor = *req.AvatarColor
	} else {
		response.AvatarColor = UserAvatarColor_PRIMARY
	}

	if req.Email != nil {
		response.Email = *req.Email
	} else {
		response.Email = "user@example.com"
	}

	if req.Name != nil {
		response.Name = *req.Name
	} else {
		response.Name = "Test User"
	}

	return response, nil
}

// DeleteUserAdmin deletes a user (admin function)
func (s *Service) DeleteUserAdmin(ctx context.Context, userID string, force bool) (*UserAdminResponseDto, error) {
	ctx, span := tracer.Start(ctx, "admin.delete_user_admin",
		trace.WithAttributes(
			attribute.String("user_id", userID),
			attribute.Bool("force", force),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "delete_user_admin")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "delete_user_admin")))
	}()

	// TODO: Implement actual user deletion when SQLC queries are available
	// For now, return a mock response with deletedAt set
	now := time.Now()

	return &UserAdminResponseDto{
		AvatarColor:          UserAvatarColor_PRIMARY,
		CreatedAt:            now,
		DeletedAt:            &now,
		Email:                "deleted@example.com",
		ID:                   userID,
		IsAdmin:              false,
		Name:                 "Deleted User",
		OauthID:              "",
		ProfileImagePath:     "",
		ProfileChangedAt:     nil,
		QuotaSizeInBytes:     nil,
		ShouldChangePassword: false,
		StorageLabel:         nil,
		UpdatedAt:            now,
	}, nil
}

// RestoreUserAdmin restores a soft-deleted user (admin function)
func (s *Service) RestoreUserAdmin(ctx context.Context, userID string) (*UserAdminResponseDto, error) {
	ctx, span := tracer.Start(ctx, "admin.restore_user_admin",
		trace.WithAttributes(attribute.String("user_id", userID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "restore_user_admin")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "restore_user_admin")))
	}()

	// TODO: Implement actual user restoration when SQLC queries are available
	// For now, return a mock response with deletedAt cleared
	now := time.Now()

	return &UserAdminResponseDto{
		AvatarColor:          UserAvatarColor_PRIMARY,
		CreatedAt:            now,
		DeletedAt:            nil,
		Email:                "restored@example.com",
		ID:                   userID,
		IsAdmin:              false,
		Name:                 "Restored User",
		OauthID:              "",
		ProfileImagePath:     "",
		ProfileChangedAt:     nil,
		QuotaSizeInBytes:     nil,
		ShouldChangePassword: false,
		StorageLabel:         nil,
		UpdatedAt:            now,
	}, nil
}

// GetUserStatisticsAdmin gets user statistics (admin function)
func (s *Service) GetUserStatisticsAdmin(ctx context.Context, userID string) (*UserStatisticsResponseDto, error) {
	ctx, span := tracer.Start(ctx, "admin.get_user_statistics_admin",
		trace.WithAttributes(attribute.String("user_id", userID)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_user_statistics_admin")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_user_statistics_admin")))
	}()

	// TODO: Implement actual statistics retrieval when SQLC queries are available
	// For now, return mock statistics
	return &UserStatisticsResponseDto{
		Photos: 0,
		Usage:  0,
		Videos: 0,
	}, nil
}

// Request/Response types

type SendNotificationRequest struct {
	Message string
	Subject string
	UserIDs []string
}

type TemplateResponseDto struct {
	HTML    string
	Subject string
}

type TestEmailResponseDto struct {
	Message string
}

type SearchUsersAdminRequest struct {
	Email       *string
	Name        *string
	WithDeleted *bool
}

type SearchUsersAdminResponse struct {
	Users []*UserAdminResponseDto
}

type CreateUserAdminRequest struct {
	Email                string
	Name                 string
	Password             string
	QuotaSizeInBytes     *int64
	ShouldChangePassword *bool
	StorageLabel         *string
}

type UpdateUserAdminRequest struct {
	AvatarColor          *UserAvatarColor
	Email                *string
	IsAdmin              *bool
	Name                 *string
	Password             *string
	QuotaSizeInBytes     *int64
	ShouldChangePassword *bool
	StorageLabel         *string
}

type UserAdminResponseDto struct {
	AvatarColor          UserAvatarColor
	CreatedAt            time.Time
	DeletedAt            *time.Time
	Email                string
	ID                   string
	IsAdmin              bool
	Name                 string
	OauthID              string
	ProfileImagePath     string
	ProfileChangedAt     *time.Time
	QuotaSizeInBytes     *int64
	ShouldChangePassword bool
	StorageLabel         *string
	UpdatedAt            time.Time
}

type UserStatisticsResponseDto struct {
	Photos int32
	Usage  int64
	Videos int32
}

type UserAvatarColor int32

const (
	UserAvatarColor_PRIMARY UserAvatarColor = 0
	UserAvatarColor_PINK    UserAvatarColor = 1
	UserAvatarColor_RED     UserAvatarColor = 2
	UserAvatarColor_YELLOW  UserAvatarColor = 3
	UserAvatarColor_BLUE    UserAvatarColor = 4
	UserAvatarColor_GREEN   UserAvatarColor = 5
	UserAvatarColor_PURPLE  UserAvatarColor = 6
	UserAvatarColor_ORANGE  UserAvatarColor = 7
	UserAvatarColor_GRAY    UserAvatarColor = 8
	UserAvatarColor_AMBER   UserAvatarColor = 9
)
