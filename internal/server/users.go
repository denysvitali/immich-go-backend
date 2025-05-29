package server

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) GetMyUser(ctx context.Context, empty *emptypb.Empty) (*immichv1.UserAdminResponse, error) {
	// TODO: Get user ID from context/auth
	userID := pgtype.UUID{}
	if err := userID.Scan("00000000-0000-0000-0000-000000000000"); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}

	user, err := s.db.GetUser(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	return s.convertUserToAdminProto(user), nil
}

func (s *Server) UpdateMyUser(ctx context.Context, request *immichv1.UserUpdateMeRequest) (*immichv1.UserAdminResponse, error) {
	// TODO: Get user ID from context/auth
	userID := pgtype.UUID{}
	if err := userID.Scan("00000000-0000-0000-0000-000000000000"); err != nil {
		return nil, status.Errorf(codes.Internal, "invalid user ID: %v", err)
	}

	var name, email, avatarColor pgtype.Text
	if request.Name != nil {
		name = pgtype.Text{String: *request.Name, Valid: true}
	}
	if request.Email != nil {
		email = pgtype.Text{String: *request.Email, Valid: true}
	}
	if request.AvatarColor != nil {
		avatarColor = pgtype.Text{String: request.AvatarColor.String(), Valid: true}
	}

	user, err := s.db.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:                   userID,
		Name:                 name,
		Email:                email,
		AvatarColor:          avatarColor,
		ProfileImagePath:     pgtype.Text{}, // Not in UserUpdateMeRequest
		ShouldChangePassword: pgtype.Bool{}, // Not in UserUpdateMeRequest
		QuotaSizeInBytes:     pgtype.Int8{}, // Not in UserUpdateMeRequest
		StorageLabel:         pgtype.Text{}, // Not in UserUpdateMeRequest
	})
	if err != nil {
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
	// User preferences would be stored in a separate table
	// For now, return empty responses (would need proper conversion from Update to Response types)
	return &immichv1.UserPreferencesResponse{
		Download:           &immichv1.DownloadResponse{},
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
	userID := pgtype.UUID{}
	if err := userID.Scan(request.UserId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user ID: %v", err)
	}

	user, err := s.db.GetUser(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	return s.convertUserToProto(user), nil
}

func (s *Server) GetProfileImage(ctx context.Context, request *immichv1.GetProfileImageRequest) (*immichv1.GetProfileImageResponse, error) {
	// Profile image retrieval would be implemented here
	return nil, status.Errorf(codes.Unimplemented, "get profile image not implemented")
}

// Helper functions to convert database user to proto
func (s *Server) convertUserToAdminProto(user sqlc.User) *immichv1.UserAdminResponse {
	avatarColor := immichv1.UserAvatarColor_USER_AVATAR_COLOR_BLUE
	if user.AvatarColor.Valid {
		// Map avatar color string to enum
		switch user.AvatarColor.String {
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
		ProfileImagePath:     user.ProfileImagePath,
		ShouldChangePassword: user.ShouldChangePassword,
		Status:               status,
		CreatedAt:            timestamppb.New(user.CreatedAt.Time),
		UpdatedAt:            timestamppb.New(user.UpdatedAt.Time),
		OauthId:              user.OauthId,
	}

	if user.QuotaSizeInBytes.Valid {
		response.QuotaSizeInBytes = &user.QuotaSizeInBytes.Int64
	}

	response.QuotaUsageInBytes = &user.QuotaUsageInBytes

	if user.StorageLabel.Valid {
		response.StorageLabel = &user.StorageLabel.String
	}

	if user.ProfileChangedAt.Valid {
		response.ProfileChangedAt = timestamppb.New(user.ProfileChangedAt.Time)
	}

	if user.DeletedAt.Valid {
		response.DeletedAt = timestamppb.New(user.DeletedAt.Time)
	}

	return response
}

func (s *Server) convertUserToProto(user sqlc.User) *immichv1.UserResponse {
	avatarColor := immichv1.UserAvatarColor_USER_AVATAR_COLOR_BLUE
	if user.AvatarColor.Valid {
		// Map avatar color string to enum
		switch user.AvatarColor.String {
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
		Id:               user.ID.String(),
		Email:            user.Email,
		Name:             user.Name,
		AvatarColor:      avatarColor,
		ProfileImagePath: user.ProfileImagePath,
	}

	if user.ProfileChangedAt.Valid {
		response.ProfileChangedAt = timestamppb.New(user.ProfileChangedAt.Time)
	}

	return response
}
