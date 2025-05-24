package services

import (
	"errors"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

type CreateUserRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName" binding:"required"`
	IsAdmin   bool   `json:"isAdmin"`
}

type UpdateUserRequest struct {
	Email                *string `json:"email,omitempty"`
	FirstName            *string `json:"firstName,omitempty"`
	LastName             *string `json:"lastName,omitempty"`
	IsAdmin              *bool   `json:"isAdmin,omitempty"`
	ShouldChangePassword *bool   `json:"shouldChangePassword,omitempty"`
	ProfileImagePath     *string `json:"profileImagePath,omitempty"`
	StorageLabel         *string `json:"storageLabel,omitempty"`
	ExternalPath         *string `json:"externalPath,omitempty"`
	MemoriesEnabled      *bool   `json:"memoriesEnabled,omitempty"`
	AvatarColor          *string `json:"avatarColor,omitempty"`
	QuotaSizeInBytes     *int64  `json:"quotaSizeInBytes,omitempty"`
	NotifyUpload         *bool   `json:"notifyUpload,omitempty"`
	NotifyAlbumInvite    *bool   `json:"notifyAlbumInvite,omitempty"`
	NotifyAlbumUpdate    *bool   `json:"notifyAlbumUpdate,omitempty"`
	NotifyComment        *bool   `json:"notifyComment,omitempty"`
}

type UserResponse struct {
	ID                   uuid.UUID  `json:"id"`
	Email                string     `json:"email"`
	FirstName            string     `json:"firstName"`
	LastName             string     `json:"lastName"`
	IsAdmin              bool       `json:"isAdmin"`
	ShouldChangePassword bool       `json:"shouldChangePassword"`
	ProfileImagePath     string     `json:"profileImagePath"`
	CreatedAt            time.Time  `json:"createdAt"`
	DeletedAt            *time.Time `json:"deletedAt,omitempty"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	OAuthID              string     `json:"oauthId"`
	StorageLabel         *string    `json:"storageLabel,omitempty"`
	ExternalPath         *string    `json:"externalPath,omitempty"`
	MemoriesEnabled      *bool      `json:"memoriesEnabled,omitempty"`
	AvatarColor          string     `json:"avatarColor"`
	QuotaSizeInBytes     *int64     `json:"quotaSizeInBytes,omitempty"`
	QuotaUsageInBytes    int64      `json:"quotaUsageInBytes"`
	NotifyUpload         bool       `json:"notifyUpload"`
	NotifyAlbumInvite    bool       `json:"notifyAlbumInvite"`
	NotifyAlbumUpdate    bool       `json:"notifyAlbumUpdate"`
	NotifyComment        bool       `json:"notifyComment"`
}

func (s *UserService) GetAllUsers(includeDeleted bool) ([]UserResponse, error) {
	var users []models.User
	query := s.db

	if !includeDeleted {
		query = query.Where("deleted_at IS NULL")
	}

	if err := query.Find(&users).Error; err != nil {
		return nil, err
	}

	responses := make([]UserResponse, len(users))
	for i, user := range users {
		responses[i] = s.toUserResponse(user)
	}

	return responses, nil
}

func (s *UserService) GetUserByID(userID uuid.UUID) (*UserResponse, error) {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	response := s.toUserResponse(user)
	return &response, nil
}

func (s *UserService) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserService) CreateUser(req CreateUserRequest) (*UserResponse, error) {
	// Check if user already exists
	var existingUser models.User
	if err := s.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return nil, errors.New("user with this email already exists")
	}

	user := models.User{
		Email:                req.Email,
		Name:                 req.FirstName, // Use Name field instead of FirstName/LastName
		IsAdmin:              req.IsAdmin,
		ShouldChangePassword: false,
		AvatarColor:          generateRandomColor(),
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}

	response := s.toUserResponse(user)
	return &response, nil
}

func (s *UserService) UpdateUser(userID uuid.UUID, req UpdateUserRequest) (*UserResponse, error) {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	// Update fields if provided
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.FirstName != nil {
		user.Name = *req.FirstName
	}
	if req.IsAdmin != nil {
		user.IsAdmin = *req.IsAdmin
	}
	if req.ShouldChangePassword != nil {
		user.ShouldChangePassword = *req.ShouldChangePassword
	}
	if req.ProfileImagePath != nil {
		user.ProfileImagePath = *req.ProfileImagePath
	}
	if req.StorageLabel != nil {
		user.StorageLabel = *req.StorageLabel
	}
	if req.AvatarColor != nil {
		user.AvatarColor = *req.AvatarColor
	}
	if req.QuotaSizeInBytes != nil {
		user.QuotaSizeInBytes = *req.QuotaSizeInBytes
	}

	if err := s.db.Save(&user).Error; err != nil {
		return nil, err
	}

	response := s.toUserResponse(user)
	return &response, nil
}

func (s *UserService) DeleteUser(userID uuid.UUID) error {
	if err := s.db.Where("id = ?", userID).Delete(&models.User{}).Error; err != nil {
		return err
	}
	return nil
}

func (s *UserService) RestoreUser(userID uuid.UUID) (*UserResponse, error) {
	var user models.User
	if err := s.db.Unscoped().Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	user.DeletedAt = gorm.DeletedAt{}
	if err := s.db.Unscoped().Save(&user).Error; err != nil {
		return nil, err
	}

	response := s.toUserResponse(user)
	return &response, nil
}

func (s *UserService) GetUserPreferences(userID uuid.UUID) (map[string]interface{}, error) {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	preferences := map[string]interface{}{
		"avatarColor": user.AvatarColor,
	}

	return preferences, nil
}

func (s *UserService) UpdateUserPreferences(userID uuid.UUID, preferences map[string]interface{}) (map[string]interface{}, error) {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	// Update preferences
	if val, ok := preferences["avatarColor"]; ok {
		if color, ok := val.(string); ok {
			user.AvatarColor = color
		}
	}

	if err := s.db.Save(&user).Error; err != nil {
		return nil, err
	}

	return s.GetUserPreferences(userID)
}

func (s *UserService) toUserResponse(user models.User) UserResponse {
	var deletedAt *time.Time
	if user.DeletedAt.Valid {
		deletedAt = &user.DeletedAt.Time
	}
	
	return UserResponse{
		ID:                   user.ID,
		Email:                user.Email,
		FirstName:            user.Name, // Use Name for FirstName
		LastName:             "",        // LastName not in model
		IsAdmin:              user.IsAdmin,
		ShouldChangePassword: user.ShouldChangePassword,
		ProfileImagePath:     user.ProfileImagePath,
		CreatedAt:            user.CreatedAt,
		DeletedAt:            deletedAt,
		UpdatedAt:            user.UpdatedAt,
		OAuthID:              user.OAuthID,
		StorageLabel:         &user.StorageLabel,
		ExternalPath:         nil, // Not in model
		MemoriesEnabled:      nil, // Not in model
		AvatarColor:          user.AvatarColor,
		QuotaSizeInBytes:     &user.QuotaSizeInBytes,
		QuotaUsageInBytes:    0, // Not in model
		NotifyUpload:         false, // Not in model
		NotifyAlbumInvite:    false, // Not in model
		NotifyAlbumUpdate:    false, // Not in model
		NotifyComment:        false, // Not in model
	}
}

func generateRandomColor() string {
	colors := []string{
		"primary", "pink", "red", "yellow", "blue", "green", "purple", "orange", "gray", "amber",
	}
	return colors[time.Now().UnixNano()%int64(len(colors))]
}
