package server

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) GetMyUser(ctx context.Context, empty *emptypb.Empty) (*immichv1.UserAdminResponse, error) {
	return &immichv1.UserAdminResponse{
		Id:                   uuid.New().String(),
		Email:                "foo@example.com",
		Name:                 "Example",
		IsAdmin:              false,
		AvatarColor:          immichv1.UserAvatarColor_USER_AVATAR_COLOR_BLUE,
		ProfileImagePath:     "",
		ProfileChangedAt:     nil,
		ShouldChangePassword: false,
		QuotaSizeInBytes:     ref(int64(1024 * 1024 * 1024)), // 1 GB
		QuotaUsageInBytes:    ref(int64(512 * 1024 * 1024)),  // 512 MB
		StorageLabel:         nil,
		Status:               immichv1.UserStatus_USER_STATUS_ACTIVE,
		CreatedAt:            timestamppb.New(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		UpdatedAt:            nil,
		DeletedAt:            nil,
		OauthId:              "",
		License:              nil,
	}, nil
}

func (s *Server) UpdateMyUser(ctx context.Context, request *immichv1.UserUpdateMeRequest) (*immichv1.UserAdminResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetUserLicense(ctx context.Context, empty *emptypb.Empty) (*immichv1.UserLicenseResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) SetUserLicense(ctx context.Context, request *immichv1.UserLicenseKeyRequest) (*immichv1.UserLicenseResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) DeleteUserLicense(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetMyPreferences(ctx context.Context, empty *emptypb.Empty) (*immichv1.UserPreferencesResponse, error) {
	return &immichv1.UserPreferencesResponse{
		Download:           &immichv1.DownloadResponse{IncludeEmbeddedVideos: true},
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
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) CreateProfileImage(ctx context.Context, request *immichv1.CreateProfileImageRequest) (*immichv1.CreateProfileImageResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) DeleteProfileImage(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetUser(ctx context.Context, request *immichv1.GetUserRequest) (*immichv1.UserResponse, error) {
	return &immichv1.UserResponse{
		Id:               uuid.New().String(),
		Email:            "foo@example.com",
		Name:             "User 1",
		AvatarColor:      immichv1.UserAvatarColor_USER_AVATAR_COLOR_BLUE,
		ProfileImagePath: "",
		ProfileChangedAt: nil,
	}, nil
}

func (s *Server) GetProfileImage(ctx context.Context, request *immichv1.GetProfileImageRequest) (*immichv1.GetProfileImageResponse, error) {
	//TODO implement me
	return nil, fmt.Errorf("not implemented")
}
