package services

import (
	"errors"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuthService struct {
	db          *gorm.DB
	authService *auth.Service
}

func NewAuthService(db *gorm.DB, authService *auth.Service) *AuthService {
	return &AuthService{
		db:          db,
		authService: authService,
	}
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken             string `json:"accessToken"`
	IsAdmin                 bool   `json:"isAdmin"`
	Name                    string `json:"name"`
	ProfileImagePath        string `json:"profileImagePath"`
	ShouldChangePassword    bool   `json:"shouldChangePassword"`
	UserEmail               string `json:"userEmail"`
	UserID                  string `json:"userId"`
}

type SignUpRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordRequest struct {
	Password    string `json:"password" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required"`
}

func (s *AuthService) Login(req LoginRequest) (*LoginResponse, error) {
	var user models.User
	if err := s.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}

	if err := s.authService.CheckPassword(req.Password, user.PasswordHash); err != nil {
		return nil, errors.New("invalid credentials")
	}

	token, err := s.authService.GenerateToken(user.ID, user.Email, user.IsAdmin, 3600)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken:          token,
		IsAdmin:              user.IsAdmin,
		Name:                 user.Name,
		ProfileImagePath:     user.ProfileImagePath,
		ShouldChangePassword: user.ShouldChangePassword,
		UserEmail:            user.Email,
		UserID:               user.ID.String(),
	}, nil
}

func (s *AuthService) AdminSignUp(req SignUpRequest) (*models.User, error) {
	// Check if any admin users exist
	var adminCount int64
	s.db.Model(&models.User{}).Where("is_admin = ?", true).Count(&adminCount)
	if adminCount > 0 {
		return nil, errors.New("admin user already exists")
	}

	hashedPassword, err := s.authService.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := models.User{
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: hashedPassword,
		IsAdmin:      true,
		ProfileChangedAt: time.Now(),
		AvatarColor:  "primary",
	}

	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *AuthService) ChangePassword(userID uuid.UUID, req ChangePasswordRequest) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, err
	}

	if err := s.authService.CheckPassword(req.Password, user.PasswordHash); err != nil {
		return nil, errors.New("current password is incorrect")
	}

	hashedPassword, err := s.authService.HashPassword(req.NewPassword)
	if err != nil {
		return nil, err
	}

	user.PasswordHash = hashedPassword
	user.ShouldChangePassword = false

	if err := s.db.Save(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *AuthService) ValidateToken(token string) (*auth.Claims, error) {
	return s.authService.ValidateToken(token)
}
