package services

import (
	"errors"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type APIKeyService struct {
	db *gorm.DB
}

func NewAPIKeyService(db *gorm.DB) *APIKeyService {
	return &APIKeyService{db: db}
}

type CreateAPIKeyRequest struct {
	Name *string `json:"name,omitempty"`
}

type APIKeyResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Secret    *string   `json:"secret,omitempty"` // Only returned on creation
}

func (s *APIKeyService) GetAllAPIKeys(userID uuid.UUID) ([]APIKeyResponse, error) {
	var apiKeys []models.APIKey
	if err := s.db.Where("user_id = ?", userID).Find(&apiKeys).Error; err != nil {
		return nil, err
	}

	responses := make([]APIKeyResponse, len(apiKeys))
	for i, apiKey := range apiKeys {
		responses[i] = APIKeyResponse{
			ID:        apiKey.ID,
			Name:      apiKey.Name,
			CreatedAt: apiKey.CreatedAt,
			UpdatedAt: apiKey.UpdatedAt,
			// Don't include secret in list response
		}
	}

	return responses, nil
}

func (s *APIKeyService) GetAPIKeyByID(keyID uuid.UUID, userID uuid.UUID) (*APIKeyResponse, error) {
	var apiKey models.APIKey
	if err := s.db.Where("id = ? AND user_id = ?", keyID, userID).First(&apiKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("API key not found")
		}
		return nil, err
	}

	return &APIKeyResponse{
		ID:        apiKey.ID,
		Name:      apiKey.Name,
		CreatedAt: apiKey.CreatedAt,
		UpdatedAt: apiKey.UpdatedAt,
		// Don't include secret in get response
	}, nil
}

func (s *APIKeyService) CreateAPIKey(userID uuid.UUID, req CreateAPIKeyRequest) (*APIKeyResponse, error) {
	// Generate API key
	keySecret, err := auth.GenerateAPIKey()
	if err != nil {
		return nil, err
	}

	// Hash the key for storage
	hashedKey, err := auth.HashPassword(keySecret)
	if err != nil {
		return nil, err
	}

	name := "API Key"
	if req.Name != nil && *req.Name != "" {
		name = *req.Name
	}

	apiKey := models.APIKey{
		ID:     uuid.New(),
		Name:   name,
		Key:    hashedKey,
		UserID: userID,
	}

	if err := s.db.Create(&apiKey).Error; err != nil {
		return nil, err
	}

	return &APIKeyResponse{
		ID:        apiKey.ID,
		Name:      apiKey.Name,
		CreatedAt: apiKey.CreatedAt,
		UpdatedAt: apiKey.UpdatedAt,
		Secret:    &keySecret, // Return the unhashed secret only on creation
	}, nil
}

func (s *APIKeyService) UpdateAPIKey(keyID uuid.UUID, userID uuid.UUID, req CreateAPIKeyRequest) (*APIKeyResponse, error) {
	var apiKey models.APIKey
	if err := s.db.Where("id = ? AND user_id = ?", keyID, userID).First(&apiKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("API key not found")
		}
		return nil, err
	}

	if req.Name != nil {
		apiKey.Name = *req.Name
	}

	if err := s.db.Save(&apiKey).Error; err != nil {
		return nil, err
	}

	return &APIKeyResponse{
		ID:        apiKey.ID,
		Name:      apiKey.Name,
		CreatedAt: apiKey.CreatedAt,
		UpdatedAt: apiKey.UpdatedAt,
	}, nil
}

func (s *APIKeyService) DeleteAPIKey(keyID uuid.UUID, userID uuid.UUID) error {
	var apiKey models.APIKey
	if err := s.db.Where("id = ? AND user_id = ?", keyID, userID).First(&apiKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("API key not found")
		}
		return err
	}

	return s.db.Delete(&apiKey).Error
}

func (s *APIKeyService) ValidateAPIKey(keySecret string) (*models.User, error) {
	// Get all API keys and check each one
	// This is not optimal for large numbers of keys, but works for most use cases
	var apiKeys []models.APIKey
	if err := s.db.Preload("User").Find(&apiKeys).Error; err != nil {
		return nil, err
	}

	for _, apiKey := range apiKeys {
		if auth.CheckPasswordHash(keySecret, apiKey.Key) {
			return &apiKey.User, nil
		}
	}

	return nil, errors.New("invalid API key")
}
