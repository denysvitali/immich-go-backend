package server

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/users"
)

func (s *Server) GetMyUser(ctx context.Context, empty *emptypb.Empty) (*immichv1.UserAdminResponse, error) {
	// TODO: Get user ID from context/auth
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000000")

	user, err := s.userService.GetUser(ctx, userID)
	if err != nil {
		if users.IsNotFoundError(err) {
			return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}

	return s.convertUserToAdminProto(user), nil
}

func (s *Server) UpdateMyUser(ctx context.Context, request *immichv1.UserUpdateMeRequest) (*immichv1.UserAdminResponse, error) {
	// TODO: Get user ID from context/auth
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000000")

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
		return nil, status.Errorf(codes.Internal, "failed to update user: %v", err)
	}

	return s.convertUserToAdminProto(user), nil
}

func (s *Server) GetUserLicense(ctx context.Context, empty *emptypb.Empty) (*immichv1.UserLicenseResponse, error) {
	// License functionality would be implemented here
	return &immichv1.UserLicenseResponse{
		LicenseKey:    "",
		ActivatedAt:   nil,
		ActivationKey: "",
	}, nil
}

func (s *Server) SetUserLicense(ctx context.Context, request *immichv1.UserLicenseKeyRequest) (*immichv1.UserLicenseResponse, error) {
	// License activation would be implemented here
	return &immichv1.UserLicenseResponse{
		LicenseKey:    request.LicenseKey,
		ActivatedAt:   timestamppb.Now(),
		ActivationKey: request.ActivationKey,
	}, nil
}

func (s *Server) DeleteUserLicense(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	// License deletion would be implemented here
	return &emptypb.Empty{}, nil
}

func (s *Server) GetMyPreferences(ctx context.Context, empty *emptypb.Empty) (*immichv1.UserPreferencesResponse, error) {
	return &immichv1.UserPreferencesResponse{
		Download:           &immichv1.UserDownloadPreferencesResponse{IncludeEmbeddedVideos: true},
		EmailNotifications: &immichv1.EmailNotificationsResponse{Enabled: false},
		Folders:            &immichv1.FoldersResponse{Enabled: true},
		Memories: &immichv1.MemoriesResponse{
			Enabled: true,
		},
		People: &immichv1.PeopleResponse{
			Enabled:       true,
			SizeThreshold: 0,
		},
		Purchase: &immichv1.PurchaseResponse{
			ShowSupportBadge: true,
		},
		Ratings: nil,
		SharedLinks: &immichv1.SharedLinksResponse{
			Enabled: true,
		},
		Tags: &immichv1.TagsResponse{
			Enabled:       true,
			SizeThreshold: 0,
		},
	}, nil
}

func (s *Server) UpdateMyPreferences(ctx context.Context, request *immichv1.UserPreferencesUpdateRequest) (*immichv1.UserPreferencesResponse, error) {
	// User preferences would be stored in a separate table
	// For now, return empty responses (would need proper conversion from Update to Response types)
	return &immichv1.UserPreferencesResponse{
		Download:           &immichv1.UserDownloadPreferencesResponse{},
		EmailNotifications: &immichv1.EmailNotificationsResponse{},
		Folders:            &immichv1.FoldersResponse{},
		Memories:           &immichv1.MemoriesResponse{},
		People:             &immichv1.PeopleResponse{},
		Purchase:           &immichv1.PurchaseResponse{},
		Ratings:            &immichv1.RatingsResponse{},
		SharedLinks:        &immichv1.SharedLinksResponse{},
		Tags:               &immichv1.TagsResponse{},
	}, nil
}

func (s *Server) CreateProfileImage(ctx context.Context, request *immichv1.CreateProfileImageRequest) (*immichv1.CreateProfileImageResponse, error) {
	// Profile image upload would be implemented here
	return &immichv1.CreateProfileImageResponse{
		UserId:           "00000000-0000-0000-0000-000000000000",
		ProfileImagePath: "/uploads/profile/image.jpg",
	}, nil
}

func (s *Server) DeleteProfileImage(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	// Profile image deletion would be implemented here
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
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}

	return s.convertUserToProto(user), nil
}

func (s *Server) GetProfileImage(ctx context.Context, request *immichv1.GetProfileImageRequest) (*immichv1.GetProfileImageResponse, error) {
	// Profile image retrieval would be implemented here
	return nil, status.Errorf(codes.Unimplemented, "get profile image not implemented")
}

// Helper functions to convert user service types to proto
func (s *Server) convertUserToAdminProto(user *users.UserInfo) *immichv1.UserAdminResponse {
	avatarColor := immichv1.UserAvatarColor_USER_AVATAR_COLOR_BLUE
	if user.AvatarColor != nil {
		// Map avatar color string to enum
		switch *user.AvatarColor {
		case "red":
			avatarColor = immichv1.UserAvatarColor_USER_AVATAR_COLOR_RED
		case "green":
			avatarColor = immichv1.UserAvatarColor_USER_AVATAR_COLOR_GREEN
		case "yellow":
			avatarColor = immichv1.UserAvatarColor_USER_AVATAR_COLOR_YELLOW
		case "orange":
			avatarColor = immichv1.UserAvatarColor_USER_AVATAR_COLOR_ORANGE
		case "purple":
			avatarColor = immichv1.UserAvatarColor_USER_AVATAR_COLOR_PURPLE
		case "pink":
			avatarColor = immichv1.UserAvatarColor_USER_AVATAR_COLOR_PINK
		}
	}

	status := immichv1.UserStatus_USER_STATUS_ACTIVE
	if user.Status == "removing" {
		status = immichv1.UserStatus_USER_STATUS_REMOVING
	} else if user.Status == "deleted" {
		status = immichv1.UserStatus_USER_STATUS_DELETED
	}

	response := &immichv1.UserAdminResponse{
		Id:                   user.ID.String(),
		Email:                user.Email,
		Name:                 user.Name,
		IsAdmin:              user.IsAdmin,
		AvatarColor:          avatarColor,
		ShouldChangePassword: user.ShouldChangePassword,
		Status:               status,
		CreatedAt:            timestamppb.New(user.CreatedAt),
		UpdatedAt:            timestamppb.New(user.UpdatedAt),
		OauthId:              user.OAuthID,
	}

	if user.ProfileImagePath != nil {
		response.ProfileImagePath = *user.ProfileImagePath
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
	avatarColor := immichv1.UserAvatarColor_USER_AVATAR_COLOR_BLUE
	if user.AvatarColor != nil {
		// Map avatar color string to enum
		switch *user.AvatarColor {
		case "red":
			avatarColor = immichv1.UserAvatarColor_USER_AVATAR_COLOR_RED
		case "green":
			avatarColor = immichv1.UserAvatarColor_USER_AVATAR_COLOR_GREEN
		case "yellow":
			avatarColor = immichv1.UserAvatarColor_USER_AVATAR_COLOR_YELLOW
		case "orange":
			avatarColor = immichv1.UserAvatarColor_USER_AVATAR_COLOR_ORANGE
		case "purple":
			avatarColor = immichv1.UserAvatarColor_USER_AVATAR_COLOR_PURPLE
		case "pink":
			avatarColor = immichv1.UserAvatarColor_USER_AVATAR_COLOR_PINK
		}
	}

	response := &immichv1.UserResponse{
		Id:          user.ID.String(),
		Email:       user.Email,
		Name:        user.Name,
		AvatarColor: avatarColor,
	}

	if user.ProfileImagePath != nil {
		response.ProfileImagePath = *user.ProfileImagePath
	}

	if user.ProfileChangedAt != nil {
		response.ProfileChangedAt = timestamppb.New(*user.ProfileChangedAt)
	}

	return response
}
