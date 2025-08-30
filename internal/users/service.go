package users

import (
	"context"
	"encoding/json"
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

var tracer = telemetry.GetTracer("users")

// Service handles user management operations
type Service struct {
	db     *sqlc.Queries
	config *config.Config

	// Metrics
	userCounter       metric.Int64UpDownCounter
	operationCounter  metric.Int64Counter
	operationDuration metric.Float64Histogram
}

// NewService creates a new user management service
func NewService(queries *sqlc.Queries, cfg *config.Config) (*Service, error) {
	meter := telemetry.GetMeter()

	userCounter, err := meter.Int64UpDownCounter(
		"users_total",
		metric.WithDescription("Total number of users in the system"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user counter: %w", err)
	}

	operationCounter, err := meter.Int64Counter(
		"user_operations_total",
		metric.WithDescription("Total number of user operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation counter: %w", err)
	}

	operationDuration, err := meter.Float64Histogram(
		"user_operation_duration_seconds",
		metric.WithDescription("Time spent on user operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create operation duration histogram: %w", err)
	}

	return &Service{
		db:                queries,
		config:            cfg,
		userCounter:       userCounter,
		operationCounter:  operationCounter,
		operationDuration: operationDuration,
	}, nil
}

// GetUser retrieves a user by ID
func (s *Service) GetUser(ctx context.Context, userID uuid.UUID) (*UserInfo, error) {
	ctx, span := tracer.Start(ctx, "users.get_user",
		trace.WithAttributes(attribute.String("user_id", userID.String())))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_user")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_user")))
	}()

	userUUID := pgtype.UUID{}
	if err := userUUID.Scan(userID.String()); err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrInvalidUserID,
			Message: "Invalid user ID format",
			Err:     err,
		}
	}

	user, err := s.db.GetUser(ctx, userUUID)
	if err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrUserNotFound,
			Message: "User not found",
			Err:     err,
		}
	}

	// Check if user is deleted
	if user.DeletedAt.Valid {
		return nil, &UserError{
			Type:    ErrUserDeleted,
			Message: "User has been deleted",
		}
	}

	return s.dbUserToUserInfo(user), nil
}

// GetUserByEmail retrieves a user by email address
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*UserInfo, error) {
	ctx, span := tracer.Start(ctx, "users.get_user_by_email",
		trace.WithAttributes(attribute.String("email", email)))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_user_by_email")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_user_by_email")))
	}()

	user, err := s.db.GetUserByEmail(ctx, email)
	if err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrUserNotFound,
			Message: "User not found",
			Err:     err,
		}
	}

	// Check if user is deleted
	if user.DeletedAt.Valid {
		return nil, &UserError{
			Type:    ErrUserDeleted,
			Message: "User has been deleted",
		}
	}

	return s.dbUserToUserInfo(user), nil
}

// ListUsers retrieves all users with pagination
func (s *Service) ListUsers(ctx context.Context, req ListUsersRequest) (*ListUsersResponse, error) {
	ctx, span := tracer.Start(ctx, "users.list_users",
		trace.WithAttributes(
			attribute.Int("limit", req.Limit),
			attribute.Int("offset", req.Offset),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "list_users")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "list_users")))
	}()

	// Set defaults for pagination
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 50 // Default limit
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	// Ensure values fit in int32 to prevent overflow
	if limit > 2147483647 {
		limit = 100
	}
	if offset > 2147483647 {
		offset = 0
	}

	// Get users from database
	dbUsers, err := s.db.ListUsers(ctx, sqlc.ListUsersParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrDatabaseError,
			Message: "Failed to retrieve users",
			Err:     err,
		}
	}

	// Convert to UserInfo
	users := make([]*UserInfo, 0, len(dbUsers))
	for _, dbUser := range dbUsers {
		// Skip deleted users unless specifically requested
		if dbUser.DeletedAt.Valid && !req.IncludeDeleted {
			continue
		}
		users = append(users, s.dbUserToUserInfo(dbUser))
	}

	// Get total count
	total, err := s.db.CountUsers(ctx, pgtype.Bool{Valid: false})
	if err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrDatabaseError,
			Message: "Failed to count users",
			Err:     err,
		}
	}

	return &ListUsersResponse{
		Users:  users,
		Total:  int(total),
		Limit:  limit,
		Offset: offset,
	}, nil
}

// UpdateUser updates user profile information
func (s *Service) UpdateUser(ctx context.Context, userID uuid.UUID, req UpdateUserRequest) (*UserInfo, error) {
	ctx, span := tracer.Start(ctx, "users.update_user",
		trace.WithAttributes(attribute.String("user_id", userID.String())))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "update_user")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "update_user")))
	}()

	userUUID := pgtype.UUID{}
	if err := userUUID.Scan(userID.String()); err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrInvalidUserID,
			Message: "Invalid user ID format",
			Err:     err,
		}
	}

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

	if req.AvatarColor != nil {
		updateParams.AvatarColor = pgtype.Text{String: *req.AvatarColor, Valid: true}
	}

	if req.ProfileImagePath != nil {
		updateParams.ProfileImagePath = pgtype.Text{String: *req.ProfileImagePath, Valid: true}
	}

	if req.QuotaSizeInBytes != nil {
		updateParams.QuotaSizeInBytes = pgtype.Int8{Int64: *req.QuotaSizeInBytes, Valid: true}
	}

	if req.StorageLabel != nil {
		updateParams.StorageLabel = pgtype.Text{String: *req.StorageLabel, Valid: true}
	}

	// Update user in database
	user, err := s.db.UpdateUser(ctx, updateParams)
	if err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrDatabaseError,
			Message: "Failed to update user",
			Err:     err,
		}
	}

	return s.dbUserToUserInfo(user), nil
}

// UpdateUserPassword updates a user's password (admin function)
func (s *Service) UpdateUserPassword(ctx context.Context, userID uuid.UUID, req UpdatePasswordRequest) error {
	ctx, span := tracer.Start(ctx, "users.update_user_password",
		trace.WithAttributes(attribute.String("user_id", userID.String())))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "update_user_password")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "update_user_password")))
	}()

	userUUID := pgtype.UUID{}
	if err := userUUID.Scan(userID.String()); err != nil {
		span.RecordError(err)
		return &UserError{
			Type:    ErrInvalidUserID,
			Message: "Invalid user ID format",
			Err:     err,
		}
	}

	// Validate password
	if err := s.validatePassword(req.NewPassword); err != nil {
		span.RecordError(err)
		return &UserError{
			Type:    ErrInvalidPassword,
			Message: "Password does not meet requirements",
			Err:     err,
		}
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		span.RecordError(err)
		return &UserError{
			Type:    ErrPasswordHashing,
			Message: "Failed to hash password",
			Err:     err,
		}
	}

	// Update password
	updateParams := sqlc.UpdateUserPasswordParams{
		ID:       userUUID,
		Password: string(hashedPassword),
	}

	if err := s.db.UpdateUserPassword(ctx, updateParams); err != nil {
		span.RecordError(err)
		return &UserError{
			Type:    ErrDatabaseError,
			Message: "Failed to update password",
			Err:     err,
		}
	}

	// Invalidate all refresh tokens for this user
	if err := s.db.DeleteUserRefreshTokens(ctx, userUUID); err != nil {
		span.RecordError(err)
		// Log error but don't fail the password update
	}

	return nil
}

// UpdateUserAdmin updates a user's admin status
func (s *Service) UpdateUserAdmin(ctx context.Context, userID uuid.UUID, isAdmin bool) (*UserInfo, error) {
	ctx, span := tracer.Start(ctx, "users.update_user_admin",
		trace.WithAttributes(
			attribute.String("user_id", userID.String()),
			attribute.Bool("is_admin", isAdmin),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "update_user_admin")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "update_user_admin")))
	}()

	userUUID := pgtype.UUID{}
	if err := userUUID.Scan(userID.String()); err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrInvalidUserID,
			Message: "Invalid user ID format",
			Err:     err,
		}
	}

	user, err := s.db.UpdateUserAdmin(ctx, sqlc.UpdateUserAdminParams{
		ID:      userUUID,
		IsAdmin: isAdmin,
	})
	if err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrDatabaseError,
			Message: "Failed to update user admin status",
			Err:     err,
		}
	}

	return s.dbUserToUserInfo(user), nil
}

// DeleteUser soft-deletes a user (sets deletedAt timestamp)
func (s *Service) DeleteUser(ctx context.Context, userID uuid.UUID, hardDelete bool) error {
	ctx, span := tracer.Start(ctx, "users.delete_user",
		trace.WithAttributes(
			attribute.String("user_id", userID.String()),
			attribute.Bool("hard_delete", hardDelete),
		))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "delete_user")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "delete_user")))
	}()

	userUUID := pgtype.UUID{}
	if err := userUUID.Scan(userID.String()); err != nil {
		span.RecordError(err)
		return &UserError{
			Type:    ErrInvalidUserID,
			Message: "Invalid user ID format",
			Err:     err,
		}
	}

	if hardDelete {
		// Hard delete - permanently remove user
		if err := s.db.HardDeleteUser(ctx, userUUID); err != nil {
			span.RecordError(err)
			return &UserError{
				Type:    ErrDatabaseError,
				Message: "Failed to delete user",
				Err:     err,
			}
		}
		s.userCounter.Add(ctx, -1)
	} else {
		// Soft delete - set deletedAt timestamp
		if err := s.db.SoftDeleteUser(ctx, userUUID); err != nil {
			span.RecordError(err)
			return &UserError{
				Type:    ErrDatabaseError,
				Message: "Failed to delete user",
				Err:     err,
			}
		}
	}

	return nil
}

// RestoreUser restores a soft-deleted user
func (s *Service) RestoreUser(ctx context.Context, userID uuid.UUID) (*UserInfo, error) {
	ctx, span := tracer.Start(ctx, "users.restore_user",
		trace.WithAttributes(attribute.String("user_id", userID.String())))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "restore_user")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "restore_user")))
	}()

	userUUID := pgtype.UUID{}
	if err := userUUID.Scan(userID.String()); err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrInvalidUserID,
			Message: "Invalid user ID format",
			Err:     err,
		}
	}

	user, err := s.db.RestoreUser(ctx, userUUID)
	if err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrDatabaseError,
			Message: "Failed to restore user",
			Err:     err,
		}
	}

	return s.dbUserToUserInfo(user), nil
}

// GetUserPreferences retrieves user preferences
func (s *Service) GetUserPreferences(ctx context.Context, userID uuid.UUID) (*UserPreferences, error) {
	ctx, span := tracer.Start(ctx, "users.get_user_preferences",
		trace.WithAttributes(attribute.String("user_id", userID.String())))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "get_user_preferences")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "get_user_preferences")))
	}()

	userUUID := pgtype.UUID{}
	if err := userUUID.Scan(userID.String()); err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrInvalidUserID,
			Message: "Invalid user ID format",
			Err:     err,
		}
	}

	// Get preferences JSON data from database
	prefsData, err := s.db.GetUserPreferencesData(ctx, userUUID)
	if err != nil {
		span.RecordError(err)
		// If no preferences found, return default preferences
		if err.Error() == "no rows in result set" {
			return s.getDefaultUserPreferences(userID), nil
		}
		return nil, &UserError{
			Type:    ErrDatabaseError,
			Message: "Failed to get user preferences",
			Err:     err,
		}
	}

	// Parse JSON preferences
	prefs := &UserPreferences{UserID: userID}
	if err := json.Unmarshal(prefsData, prefs); err != nil {
		span.RecordError(err)
		// If JSON is invalid, return default preferences
		return s.getDefaultUserPreferences(userID), nil
	}

	return prefs, nil
}

// UpdateUserPreferences updates user preferences
func (s *Service) UpdateUserPreferences(ctx context.Context, userID uuid.UUID, req UpdateUserPreferencesRequest) (*UserPreferences, error) {
	ctx, span := tracer.Start(ctx, "users.update_user_preferences",
		trace.WithAttributes(attribute.String("user_id", userID.String())))
	defer span.End()

	start := time.Now()
	defer func() {
		s.operationDuration.Record(ctx, time.Since(start).Seconds(),
			metric.WithAttributes(attribute.String("operation", "update_user_preferences")))
		s.operationCounter.Add(ctx, 1,
			metric.WithAttributes(attribute.String("operation", "update_user_preferences")))
	}()

	userUUID := pgtype.UUID{}
	if err := userUUID.Scan(userID.String()); err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrInvalidUserID,
			Message: "Invalid user ID format",
			Err:     err,
		}
	}

	// Get current preferences or create default
	currentPrefs, err := s.GetUserPreferences(ctx, userID)
	if err != nil {
		span.RecordError(err)
		currentPrefs = s.getDefaultUserPreferences(userID)
	}

	// Update only the provided fields
	if req.EmailNotifications != nil {
		currentPrefs.EmailNotifications = req.EmailNotifications
	}
	if req.DownloadIncludeEmbeddedVideos != nil {
		currentPrefs.DownloadIncludeEmbeddedVideos = req.DownloadIncludeEmbeddedVideos
	}
	if req.FoldersEnabled != nil {
		currentPrefs.FoldersEnabled = req.FoldersEnabled
	}
	if req.MemoriesEnabled != nil {
		currentPrefs.MemoriesEnabled = req.MemoriesEnabled
	}
	if req.PeopleEnabled != nil {
		currentPrefs.PeopleEnabled = req.PeopleEnabled
	}
	if req.PeopleSizeThreshold != nil {
		currentPrefs.PeopleSizeThreshold = req.PeopleSizeThreshold
	}
	if req.SharedLinksEnabled != nil {
		currentPrefs.SharedLinksEnabled = req.SharedLinksEnabled
	}
	if req.TagsEnabled != nil {
		currentPrefs.TagsEnabled = req.TagsEnabled
	}
	if req.TagsSizeThreshold != nil {
		currentPrefs.TagsSizeThreshold = req.TagsSizeThreshold
	}

	// Marshal preferences to JSON
	prefsJSON, err := json.Marshal(currentPrefs)
	if err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrDatabaseError,
			Message: "Failed to encode preferences",
			Err:     err,
		}
	}

	// Update in database
	_, err = s.db.UpdateUserPreferencesData(ctx, sqlc.UpdateUserPreferencesDataParams{
		UserId: userUUID,
		Value:  prefsJSON,
	})
	if err != nil {
		span.RecordError(err)
		return nil, &UserError{
			Type:    ErrDatabaseError,
			Message: "Failed to update user preferences",
			Err:     err,
		}
	}

	return currentPrefs, nil
}

// Helper functions

// dbUserToUserInfo converts a database user to UserInfo
func (s *Service) dbUserToUserInfo(user sqlc.User) *UserInfo {
	userInfo := &UserInfo{
		ID:                   user.ID.Bytes,
		Email:                user.Email,
		Name:                 user.Name,
		IsAdmin:              user.IsAdmin,
		ShouldChangePassword: user.ShouldChangePassword,
		Status:               user.Status,
		CreatedAt:            user.CreatedAt.Time,
		UpdatedAt:            user.UpdatedAt.Time,
		QuotaUsageInBytes:    user.QuotaUsageInBytes,
		OAuthID:              user.OauthId,
	}

	if user.ProfileImagePath != "" {
		userInfo.ProfileImagePath = &user.ProfileImagePath
	}

	if user.StorageLabel.Valid {
		userInfo.StorageLabel = &user.StorageLabel.String
	}

	if user.QuotaSizeInBytes.Valid {
		userInfo.QuotaSizeInBytes = &user.QuotaSizeInBytes.Int64
	}

	if user.AvatarColor.Valid {
		userInfo.AvatarColor = &user.AvatarColor.String
	}

	if user.ProfileChangedAt.Valid {
		userInfo.ProfileChangedAt = &user.ProfileChangedAt.Time
	}

	if user.DeletedAt.Valid {
		userInfo.DeletedAt = &user.DeletedAt.Time
	}

	return userInfo
}

// getDefaultUserPreferences returns default user preferences
func (s *Service) getDefaultUserPreferences(userID uuid.UUID) *UserPreferences {
	emailNotifications := true
	downloadIncludeEmbeddedVideos := false
	foldersEnabled := true
	memoriesEnabled := true
	peopleEnabled := true
	peopleSizeThreshold := int32(10)
	sharedLinksEnabled := true
	tagsEnabled := true
	tagsSizeThreshold := int32(10)

	return &UserPreferences{
		UserID:                        userID,
		EmailNotifications:            &emailNotifications,
		DownloadIncludeEmbeddedVideos: &downloadIncludeEmbeddedVideos,
		FoldersEnabled:                &foldersEnabled,
		MemoriesEnabled:               &memoriesEnabled,
		PeopleEnabled:                 &peopleEnabled,
		PeopleSizeThreshold:           &peopleSizeThreshold,
		SharedLinksEnabled:            &sharedLinksEnabled,
		TagsEnabled:                   &tagsEnabled,
		TagsSizeThreshold:             &tagsSizeThreshold,
	}
}

// validatePassword validates password complexity requirements
func (s *Service) validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}
	// Add additional password complexity requirements here
	return nil
}
