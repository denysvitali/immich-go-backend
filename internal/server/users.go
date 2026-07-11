package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/calendarheatmap"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/users"
)

type userLicenseMetadata struct {
	ActivatedAt   time.Time `json:"activatedAt"`
	ActivationKey string    `json:"activationKey"`
	LicenseKey    string    `json:"licenseKey"`
}

func (s *Server) GetMyUser(ctx context.Context, empty *emptypb.Empty) (*immichv1.UserAdminResponse, error) {
	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	user, err := s.userService.GetUser(ctx, userID)
	if err != nil {
		if users.IsNotFoundError(err) {
			return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
		}
		return nil, SanitizedInternal(ctx, "failed to get user", err)
	}

	return s.convertUserToAdminProto(user), nil
}

func (s *Server) UpdateMyUser(ctx context.Context, request *immichv1.UserUpdateMeRequest) (*immichv1.UserAdminResponse, error) {
	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Build update request
	updateReq := &users.UpdateUserRequest{}

	if request.Name != nil {
		updateReq.Name = request.Name
	}
	if request.Email != nil {
		updateReq.Email = request.Email
	}
	if request.AvatarColor != nil {
		avatarColor := request.AvatarColor.String()
		updateReq.AvatarColor = &avatarColor
	}

	user, err := s.userService.UpdateUser(ctx, userID, *updateReq)
	if err != nil {
		if users.IsNotFoundError(err) {
			return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
		}
		if users.IsValidationError(err) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
		}
		return nil, SanitizedInternal(ctx, "failed to update user", err)
	}

	return s.convertUserToAdminProto(user), nil
}

func (s *Server) GetUserLicense(ctx context.Context, empty *emptypb.Empty) (*immichv1.UserLicenseResponse, error) {
	userUUID, err := s.userUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	data, err := s.db.GetUserLicenseData(ctx, userUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &immichv1.UserLicenseResponse{}, nil
		}
		return nil, SanitizedInternal(ctx, "failed to get user license", err)
	}

	license, err := parseUserLicense(data)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to parse user license", err)
	}

	return userLicenseToProto(license), nil
}

func (s *Server) SetUserLicense(ctx context.Context, request *immichv1.UserLicenseKeyRequest) (*immichv1.UserLicenseResponse, error) {
	userUUID, err := s.userUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	license := userLicenseMetadata{
		ActivatedAt:   time.Now().UTC(),
		ActivationKey: request.ActivationKey,
		LicenseKey:    request.LicenseKey,
	}
	data, err := json.Marshal(license)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to encode user license", err)
	}

	data, err = s.db.SetUserLicenseData(ctx, sqlc.SetUserLicenseDataParams{
		UserId: userUUID,
		Value:  data,
	})
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to set user license", err)
	}

	license, err = parseUserLicense(data)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to parse user license", err)
	}

	return userLicenseToProto(license), nil
}

func (s *Server) DeleteUserLicense(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	userUUID, err := s.userUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.db.DeleteUserLicenseData(ctx, userUUID); err != nil {
		return nil, SanitizedInternal(ctx, "failed to delete user license", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) claimsFromContext(ctx context.Context) (*auth.Claims, error) {
	return auth.ClaimsFromContext(ctx, s.authService.ValidateToken)
}

func (s *Server) userIDFromContext(ctx context.Context) (uuid.UUID, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return uuid.Nil, err
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return uuid.Nil, SanitizedInternal(ctx, "invalid user ID", err)
	}

	return userID, nil
}

func (s *Server) userUUIDFromContext(ctx context.Context) (pgtype.UUID, error) {
	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: userID, Valid: true}, nil
}

func parseUserLicense(data []byte) (userLicenseMetadata, error) {
	var license userLicenseMetadata
	if err := json.Unmarshal(data, &license); err != nil {
		return userLicenseMetadata{}, err
	}
	return license, nil
}

func userLicenseToProto(license userLicenseMetadata) *immichv1.UserLicenseResponse {
	response := &immichv1.UserLicenseResponse{
		ActivationKey: license.ActivationKey,
		LicenseKey:    license.LicenseKey,
	}
	if !license.ActivatedAt.IsZero() {
		response.ActivatedAt = timestamppb.New(license.ActivatedAt)
	}
	return response
}

func (s *Server) GetMyPreferences(ctx context.Context, empty *emptypb.Empty) (*immichv1.UserPreferencesResponse, error) {
	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	prefs, err := s.userService.GetUserPreferences(ctx, userID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get user preferences", err)
	}

	return userPreferencesToProto(prefs), nil
}

func (s *Server) UpdateMyPreferences(ctx context.Context, request *immichv1.UserPreferencesUpdateRequest) (*immichv1.UserPreferencesResponse, error) {
	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	prefs, err := s.userService.UpdateUserPreferences(ctx, userID, userPreferencesUpdateFromProto(request))
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to update user preferences", err)
	}

	return userPreferencesToProto(prefs), nil
}

// userPreferencesToProto must emit every section of the upstream
// UserPreferencesResponseDto: the web app reads fields like
// cast.gCastEnabled and recentlyAdded.sidebarWeb without guarding, so a
// missing section crashes client-side routing (blank page). Defaults mirror
// upstream getDefaultPreferences().
func userPreferencesToProto(prefs *users.UserPreferences) *immichv1.UserPreferencesResponse {
	if prefs == nil {
		prefs = &users.UserPreferences{}
	}

	return &immichv1.UserPreferencesResponse{
		Albums: &immichv1.AlbumsPreferencesResponse{
			DefaultAssetOrder: "desc",
		},
		Cast: &immichv1.CastPreferencesResponse{
			GCastEnabled: false,
		},
		RecentlyAdded: &immichv1.RecentlyAddedPreferencesResponse{
			SidebarWeb: false,
		},
		Download: &immichv1.UserDownloadPreferencesResponse{
			IncludeEmbeddedVideos: boolValue(prefs.DownloadIncludeEmbeddedVideos),
			ArchiveSize:           4 << 30,
		},
		EmailNotifications: &immichv1.EmailNotificationsResponse{
			Enabled:     boolValue(prefs.EmailNotifications),
			AlbumInvite: boolValue(prefs.EmailAlbumInvite),
			AlbumUpdate: boolValue(prefs.EmailAlbumUpdate),
		},
		Folders: &immichv1.FoldersResponse{
			Enabled:       boolValue(prefs.FoldersEnabled),
			SizeThreshold: int32Value(prefs.FoldersSizeThreshold),
		},
		Memories: &immichv1.MemoriesResponse{
			Enabled:  boolValue(prefs.MemoriesEnabled),
			Duration: 5,
		},
		People: &immichv1.PeopleResponse{
			Enabled:       boolValue(prefs.PeopleEnabled),
			SizeThreshold: int32Value(prefs.PeopleSizeThreshold),
		},
		Purchase: &immichv1.PurchaseResponse{
			ShowSupportBadge:   boolValue(prefs.PurchaseShowSupportBadge),
			HideBuyButtonUntil: "2022-02-12T00:00:00.000Z",
		},
		Ratings: &immichv1.RatingsResponse{
			Enabled: boolValue(prefs.RatingsEnabled),
		},
		SharedLinks: &immichv1.SharedLinksResponse{
			Enabled:         boolValue(prefs.SharedLinksEnabled),
			ShowMetadata:    boolValue(prefs.SharedLinksShowMetadata),
			PasswordOptions: stringValue(prefs.SharedLinksPasswordOptions),
		},
		Tags: &immichv1.TagsResponse{
			Enabled:       boolValue(prefs.TagsEnabled),
			SizeThreshold: int32Value(prefs.TagsSizeThreshold),
		},
	}
}

func userPreferencesUpdateFromProto(request *immichv1.UserPreferencesUpdateRequest) users.UpdateUserPreferencesRequest {
	update := users.UpdateUserPreferencesRequest{}
	if request == nil {
		return update
	}

	if request.Download != nil && request.Download.IncludeEmbeddedVideos != nil {
		update.DownloadIncludeEmbeddedVideos = request.Download.IncludeEmbeddedVideos
	}
	if request.EmailNotifications != nil {
		update.EmailNotifications = request.EmailNotifications.Enabled
		update.EmailAlbumInvite = request.EmailNotifications.AlbumInvite
		update.EmailAlbumUpdate = request.EmailNotifications.AlbumUpdate
	}
	if request.Folders != nil {
		update.FoldersEnabled = request.Folders.Enabled
		update.FoldersSizeThreshold = request.Folders.SizeThreshold
	}
	if request.Memories != nil && request.Memories.Enabled != nil {
		update.MemoriesEnabled = request.Memories.Enabled
	}
	if request.People != nil {
		update.PeopleEnabled = request.People.Enabled
		update.PeopleSizeThreshold = request.People.SizeThreshold
	}
	if request.Purchase != nil && request.Purchase.ShowSupportBadge != nil {
		update.PurchaseShowSupportBadge = request.Purchase.ShowSupportBadge
	}
	if request.Ratings != nil && request.Ratings.Enabled != nil {
		update.RatingsEnabled = request.Ratings.Enabled
	}
	if request.SharedLinks != nil {
		update.SharedLinksEnabled = request.SharedLinks.Enabled
		update.SharedLinksShowMetadata = request.SharedLinks.ShowMetadata
		update.SharedLinksPasswordOptions = request.SharedLinks.PasswordOptions
	}
	if request.Tags != nil {
		update.TagsEnabled = request.Tags.Enabled
		update.TagsSizeThreshold = request.Tags.SizeThreshold
	}

	return update
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func int32Value(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *Server) GetMyCalendarHeatmap(ctx context.Context, request *immichv1.GetMyCalendarHeatmapRequest) (*immichv1.CalendarHeatmapResponseDto, error) {
	userUUID, err := s.userUUIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	response, err := calendarheatmap.Get(ctx, s.db.Queries, userUUID, request.GetFrom(), request.GetTo(), request.GetType())
	if err != nil {
		if calendarheatmap.IsInvalidArgument(err) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, SanitizedInternal(ctx, "failed to get calendar heatmap", err)
	}

	return response, nil
}

func (s *Server) CreateProfileImage(ctx context.Context, request *immichv1.CreateProfileImageRequest) (*immichv1.CreateProfileImageResponse, error) {
	// Get user ID from context
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Validate image data
	if len(request.File) == 0 {
		return nil, status.Error(codes.InvalidArgument, "image data is required")
	}

	// Detect image type
	contentType := http.DetectContentType(request.File)
	if !strings.HasPrefix(contentType, "image/") {
		return nil, status.Error(codes.InvalidArgument, "file must be an image")
	}

	// Generate storage path for profile image
	ext := ".jpg"
	if strings.Contains(contentType, "png") {
		ext = ".png"
	} else if strings.Contains(contentType, "webp") {
		ext = ".webp"
	}
	profilePath := fmt.Sprintf("profile/%s/avatar%s", userID.String(), ext)

	// Get storage service
	storageService := s.assetService.GetStorageService()

	// Upload the profile image
	if err := storageService.Upload(ctx, profilePath, bytes.NewReader(request.File), contentType); err != nil {
		return nil, SanitizedInternal(ctx, "failed to upload profile image", err)
	}

	// Update user record with profile image path
	updatedUser, err := s.db.SetUserProfileImage(ctx, sqlc.SetUserProfileImageParams{
		ID:               userUUID,
		ProfileImagePath: profilePath,
	})
	if err != nil {
		// Profile image cleanup on failure would go here
		return nil, SanitizedInternal(ctx, "failed to update user profile", err)
	}

	return &immichv1.CreateProfileImageResponse{
		UserId:           userID.String(),
		ProfileImagePath: profilePath,
		ProfileChangedAt: timestamppb.New(updatedUser.ProfileChangedAt.Time),
	}, nil
}

func (s *Server) DeleteProfileImage(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	claims, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "invalid user ID", err)
	}
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	user, err := s.db.GetUserByID(ctx, userUUID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	if user.ProfileImagePath == "" {
		return &emptypb.Empty{}, nil
	}

	storageService := s.assetService.GetStorageService()
	if err := storageService.DeleteAsset(ctx, user.ProfileImagePath); err != nil {
		return nil, SanitizedInternal(ctx, "failed to delete profile image", err)
	}

	if _, err := s.db.ClearUserProfileImage(ctx, userUUID); err != nil {
		return nil, SanitizedInternal(ctx, "failed to update user profile", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *Server) GetUser(ctx context.Context, request *immichv1.GetUserRequest) (*immichv1.UserResponse, error) {
	userID, err := uuid.Parse(request.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}

	user, err := s.userService.GetUser(ctx, userID)
	if err != nil {
		if users.IsNotFoundError(err) {
			return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
		}
		return nil, SanitizedInternal(ctx, "failed to get user", err)
	}

	return s.convertUserToProto(user), nil
}

func (s *Server) GetProfileImage(ctx context.Context, request *immichv1.GetProfileImageRequest) (*immichv1.GetProfileImageResponse, error) {
	// Parse user ID
	userID, err := uuid.Parse(request.UserId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}

	// Convert to pgtype.UUID
	userUUID := pgtype.UUID{Bytes: userID, Valid: true}

	// Get user from database to retrieve profile image path
	user, err := s.db.GetUserByID(ctx, userUUID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	// Check if user has a profile image
	if user.ProfileImagePath == "" {
		return nil, status.Errorf(codes.NotFound, "profile image not found")
	}

	// Get storage service
	storageService := s.assetService.GetStorageService()

	// Download the profile image
	imageData, err := storageService.Download(ctx, user.ProfileImagePath)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to retrieve profile image", err)
	}
	defer imageData.Close()

	// Read image data
	data, err := io.ReadAll(imageData)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to read image data", err)
	}

	// Determine content type from file extension
	contentType := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(user.ProfileImagePath), ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(strings.ToLower(user.ProfileImagePath), ".webp") {
		contentType = "image/webp"
	}

	return &immichv1.GetProfileImageResponse{
		ImageData:   data,
		ContentType: contentType,
	}, nil
}

// Helper functions to convert user service types to proto
func (s *Server) convertUserToAdminProto(user *users.UserInfo) *immichv1.UserAdminResponse {
	response := &immichv1.UserAdminResponse{
		Id:                   user.ID.String(),
		Email:                user.Email,
		Name:                 user.Name,
		IsAdmin:              user.IsAdmin,
		AvatarColor:          userAvatarColorToProto(user.AvatarColor),
		ShouldChangePassword: user.ShouldChangePassword,
		Status:               userStatusToProto(user.Status),
		CreatedAt:            timestamppb.New(user.CreatedAt),
		UpdatedAt:            timestamppb.New(user.UpdatedAt),
		OauthId:              user.OAuthID,
	}

	if user.ProfileImagePath != nil {
		response.ProfileImagePath = *user.ProfileImagePath
	}

	if user.ProfileChangedAt != nil {
		response.ProfileChangedAt = timestamppb.New(*user.ProfileChangedAt)
	}

	if user.QuotaSizeInBytes != nil {
		response.QuotaSizeInBytes = user.QuotaSizeInBytes
	}

	response.QuotaUsageInBytes = &user.QuotaUsageInBytes

	if user.StorageLabel != nil {
		response.StorageLabel = user.StorageLabel
	}

	return response
}

func (s *Server) convertUserToProto(user *users.UserInfo) *immichv1.UserResponse {
	response := &immichv1.UserResponse{
		Id:          user.ID.String(),
		Email:       user.Email,
		Name:        user.Name,
		AvatarColor: userAvatarColorToProto(user.AvatarColor),
	}

	if user.ProfileImagePath != nil {
		response.ProfileImagePath = *user.ProfileImagePath
	}

	if user.ProfileChangedAt != nil {
		response.ProfileChangedAt = timestamppb.New(*user.ProfileChangedAt)
	}

	return response
}

var userAvatarColorValues = map[string]immichv1.UserAvatarColor{
	"red":    immichv1.UserAvatarColor_USER_AVATAR_COLOR_RED,
	"green":  immichv1.UserAvatarColor_USER_AVATAR_COLOR_GREEN,
	"yellow": immichv1.UserAvatarColor_USER_AVATAR_COLOR_YELLOW,
	"orange": immichv1.UserAvatarColor_USER_AVATAR_COLOR_ORANGE,
	"purple": immichv1.UserAvatarColor_USER_AVATAR_COLOR_PURPLE,
	"pink":   immichv1.UserAvatarColor_USER_AVATAR_COLOR_PINK,
}

func userAvatarColorToProto(color *string) immichv1.UserAvatarColor {
	if color == nil {
		return immichv1.UserAvatarColor_USER_AVATAR_COLOR_BLUE
	}
	if protoColor, ok := userAvatarColorValues[*color]; ok {
		return protoColor
	}
	return immichv1.UserAvatarColor_USER_AVATAR_COLOR_BLUE
}

var userStatusValues = map[string]immichv1.UserStatus{
	"removing": immichv1.UserStatus_USER_STATUS_REMOVING,
	"deleted":  immichv1.UserStatus_USER_STATUS_DELETED,
}

func userStatusToProto(status string) immichv1.UserStatus {
	if protoStatus, ok := userStatusValues[status]; ok {
		return protoStatus
	}
	return immichv1.UserStatus_USER_STATUS_ACTIVE
}

// ListUsers returns all users (for authenticated users)
func (s *Server) ListUsers(ctx context.Context, _ *emptypb.Empty) (*immichv1.ListUsersResponse, error) {
	// Ensure user is authenticated
	_, err := s.claimsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Get all users from service
	usersList, err := s.userService.GetAllUsers(ctx)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to list users", err)
	}

	// Convert to proto
	protoUsers := make([]*immichv1.UserResponse, 0, len(usersList))
	for _, user := range usersList {
		protoUsers = append(protoUsers, s.convertUserToProto(user))
	}

	return &immichv1.ListUsersResponse{
		Users: protoUsers,
	}, nil
}

// GetOnboarding returns the user's onboarding status
func (s *Server) GetOnboarding(ctx context.Context, _ *emptypb.Empty) (*immichv1.OnboardingResponse, error) {
	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	onboarding, err := s.userService.GetUserOnboarding(ctx, userID)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to get onboarding status", err)
	}

	return &immichv1.OnboardingResponse{
		IsOnboarded: onboarding.IsOnboarded,
	}, nil
}

// UpdateOnboarding updates the user's onboarding status
func (s *Server) UpdateOnboarding(ctx context.Context, req *immichv1.OnboardingUpdateRequest) (*immichv1.OnboardingResponse, error) {
	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	onboarding, err := s.userService.UpdateUserOnboarding(ctx, userID, req.IsOnboarded)
	if err != nil {
		return nil, SanitizedInternal(ctx, "failed to update onboarding status", err)
	}

	return &immichv1.OnboardingResponse{
		IsOnboarded: onboarding.IsOnboarded,
	}, nil
}

// DeleteOnboarding clears the user's onboarding status.
func (s *Server) DeleteOnboarding(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := s.userService.UpdateUserOnboarding(ctx, userID, false); err != nil {
		return nil, SanitizedInternal(ctx, "failed to delete onboarding status", err)
	}

	return &emptypb.Empty{}, nil
}
