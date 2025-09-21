package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/bcrypt"
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

	// Template rendering requires template system to be implemented
	// Return error instead of mock data
	return nil, fmt.Errorf("template rendering not yet implemented - requires template system")
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

	// Email testing requires email system to be implemented
	// Return error instead of mock data
	return nil, fmt.Errorf("email testing not yet implemented - requires email system")
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

	// Validate input
	if req.Email == "" || req.Name == "" || req.Password == "" {
		return nil, fmt.Errorf("email, name, and password are required")
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Generate new user ID
	userID := uuid.New()
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Create user in database
	user, err := s.db.CreateUser(ctx, sqlc.CreateUserParams{
		ID:       userUUID,
		Email:    req.Email,
		Name:     req.Name,
		Password: string(hashedPassword),
		IsAdmin:  false, // New users are not admins by default
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Update optional fields if provided
	if req.QuotaSizeInBytes != nil || req.ShouldChangePassword != nil || req.StorageLabel != nil {
		var updateParams sqlc.UpdateUserParams
		updateParams.ID = userUUID
		if req.QuotaSizeInBytes != nil {
			quota := pgtype.Int8{Int64: *req.QuotaSizeInBytes, Valid: true}
			updateParams.QuotaSizeInBytes = quota
		}
		if req.ShouldChangePassword != nil {
			changePass := pgtype.Bool{Bool: *req.ShouldChangePassword, Valid: true}
			updateParams.ShouldChangePassword = changePass
		}
		if req.StorageLabel != nil {
			label := pgtype.Text{String: *req.StorageLabel, Valid: true}
			updateParams.StorageLabel = label
		}
		user, err = s.db.UpdateUser(ctx, updateParams)
		if err != nil {
			return nil, fmt.Errorf("failed to update user fields: %w", err)
		}
	}

	return s.convertUserToDto(&user), nil
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

	// Parse user ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Get user from database
	user, err := s.db.GetUserByID(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return s.convertUserToDto(&user), nil
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

	// Parse user ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Build update parameters
	updateParams := sqlc.UpdateUserParams{
		ID: userUUID,
	}

	if req.Name != nil {
		updateParams.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Email != nil {
		updateParams.Email = pgtype.Text{String: *req.Email, Valid: true}
	}
	if req.IsAdmin != nil {
		updateParams.IsAdmin = pgtype.Bool{Bool: *req.IsAdmin, Valid: true}
	}
	if req.AvatarColor != nil {
		color := pgtype.Text{String: fmt.Sprintf("%d", *req.AvatarColor), Valid: true}
		updateParams.AvatarColor = color
	}
	if req.QuotaSizeInBytes != nil {
		updateParams.QuotaSizeInBytes = pgtype.Int8{Int64: *req.QuotaSizeInBytes, Valid: true}
	}
	if req.ShouldChangePassword != nil {
		updateParams.ShouldChangePassword = pgtype.Bool{Bool: *req.ShouldChangePassword, Valid: true}
	}
	if req.StorageLabel != nil {
		updateParams.StorageLabel = pgtype.Text{String: *req.StorageLabel, Valid: true}
	}

	// Update user in database
	user, err := s.db.UpdateUser(ctx, updateParams)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// If password change requested, update password separately
	if req.Password != nil && *req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		err = s.db.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
			ID:       userUUID,
			Password: string(hashedPassword),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to update password: %w", err)
		}
		// Re-fetch user after password update
		user, err = s.db.GetUserByID(ctx, userUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to get updated user: %w", err)
		}
	}

	return s.convertUserToDto(&user), nil
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

	// Parse user ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Get user before deletion for return
	user, err := s.db.GetUserByID(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Delete user sessions first
	err = s.db.DeleteUserRefreshTokens(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete user sessions: %w", err)
	}

	// Perform deletion based on force flag
	if force {
		// Hard delete - permanently remove from database
		err = s.db.HardDeleteUser(ctx, userUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to hard delete user: %w", err)
		}
	} else {
		// Soft delete - set deletedAt timestamp
		err = s.db.SoftDeleteUser(ctx, userUUID)
		if err != nil {
			return nil, fmt.Errorf("failed to soft delete user: %w", err)
		}
	}

	// Return the user data as it was before deletion
	dto := s.convertUserToDto(&user)
	if !force {
		now := time.Now()
		dto.DeletedAt = &now
	}
	return dto, nil
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

	// Parse user ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Restore user in database
	user, err := s.db.RestoreUser(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to restore user: %w", err)
	}

	return s.convertUserToDto(&user), nil
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

	// Parse user ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Count photos (IMAGE type assets)
	imageType := pgtype.Text{String: "IMAGE", Valid: true}
	photoCount, err := s.db.CountAssets(ctx, sqlc.CountAssetsParams{
		OwnerId: userUUID,
		Type:    imageType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to count photos: %w", err)
	}

	// Count videos (VIDEO type assets)
	videoType := pgtype.Text{String: "VIDEO", Valid: true}
	videoCount, err := s.db.CountAssets(ctx, sqlc.CountAssetsParams{
		OwnerId: userUUID,
		Type:    videoType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to count videos: %w", err)
	}

	// Calculate total storage usage by getting all user assets and summing file sizes
	var totalUsage int64
	userAssets, err := s.db.GetUserAssets(ctx, sqlc.GetUserAssetsParams{
		OwnerId: userUUID,
		Limit:   pgtype.Int4{Int32: 10000, Valid: true},
		Offset:  pgtype.Int4{Int32: 0, Valid: true},
	})
	if err == nil {
		for _, asset := range userAssets {
			// Get exif data for file size
			exif, err := s.db.GetExifByAssetId(ctx, asset.ID)
			if err == nil && exif.FileSizeInByte.Valid {
				totalUsage += exif.FileSizeInByte.Int64
			}
		}
	}

	return &UserStatisticsResponseDto{
		Photos: int32(photoCount),
		Usage:  totalUsage,
		Videos: int32(videoCount),
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

// convertUserToDto converts a database user to a DTO
func (s *Service) convertUserToDto(user *sqlc.User) *UserAdminResponseDto {
	dto := &UserAdminResponseDto{
		ID:                   uuid.UUID(user.ID.Bytes).String(),
		Email:                user.Email,
		Name:                 user.Name,
		IsAdmin:              user.IsAdmin,
		CreatedAt:            user.CreatedAt.Time,
		UpdatedAt:            user.UpdatedAt.Time,
		ShouldChangePassword: user.ShouldChangePassword,
	}

	// Set avatar color
	if user.AvatarColor != "" {
		// Parse avatar color from string
		var colorValue int
		if _, err := fmt.Sscanf(user.AvatarColor, "%d", &colorValue); err == nil {
			dto.AvatarColor = UserAvatarColor(colorValue)
		}
	} else {
		dto.AvatarColor = UserAvatarColor_PRIMARY
	}

	// Set optional fields
	if user.ProfileImagePath.Valid {
		dto.ProfileImagePath = user.ProfileImagePath.String
	}
	if user.OauthId.Valid {
		dto.OauthID = user.OauthId.String
	}
	if user.DeletedAt.Valid {
		delTime := user.DeletedAt.Time
		dto.DeletedAt = &delTime
	}
	if user.ProfileChangedAt.Valid {
		profTime := user.ProfileChangedAt.Time
		dto.ProfileChangedAt = &profTime
	}
	if user.QuotaSizeInBytes.Valid {
		quota := user.QuotaSizeInBytes.Int64
		dto.QuotaSizeInBytes = &quota
	}
	if user.StorageLabel.Valid {
		label := user.StorageLabel.String
		dto.StorageLabel = &label
	}

	return dto
}
