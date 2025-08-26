package apikeys

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	db *sqlc.Queries
}

func NewService(db *sqlc.Queries) *Service {
	return &Service{
		db: db,
	}
}

// GenerateAPIKey generates a new random API key
func (s *Service) GenerateAPIKey() (string, error) {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode to base64 URL-safe string
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// HashAPIKey hashes an API key for storage
func (s *Service) HashAPIKey(key string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash API key: %w", err)
	}
	return string(hashedBytes), nil
}

// VerifyAPIKey verifies an API key against its hash
func (s *Service) VerifyAPIKey(key, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
	return err == nil
}

// CreateAPIKey creates a new API key for a user
func (s *Service) CreateAPIKey(ctx context.Context, userID uuid.UUID, name string) (*sqlc.ApiKey, string, error) {
	// Generate a new API key
	rawKey, err := s.GenerateAPIKey()
	if err != nil {
		return nil, "", err
	}

	// Hash the key for storage
	hashedKey, err := s.HashAPIKey(rawKey)
	if err != nil {
		return nil, "", err
	}

	// Store in database
	apiKey, err := s.db.CreateApiKey(ctx, sqlc.CreateApiKeyParams{
		Name:        name,
		Key:         hashedKey,
		UserId:      pgtype.UUID{Bytes: userID, Valid: true},
		Permissions: []string{}, // Default permissions - can be expanded later
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to create API key: %w", err)
	}

	// Return the created key and the raw key (only shown once)
	return &apiKey, rawKey, nil
}

// GetAPIKeysByUser retrieves all API keys for a user
func (s *Service) GetAPIKeysByUser(ctx context.Context, userID uuid.UUID) ([]sqlc.ApiKey, error) {
	keys, err := s.db.GetApiKeysByUser(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to get API keys: %w", err)
	}
	return keys, nil
}

// DeleteAPIKey deletes an API key
func (s *Service) DeleteAPIKey(ctx context.Context, keyID, userID uuid.UUID) error {
	err := s.db.DeleteApiKey(ctx, sqlc.DeleteApiKeyParams{
		ID:     pgtype.UUID{Bytes: keyID, Valid: true},
		UserId: pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}
	return nil
}

// ValidateAPIKey validates an API key and returns the associated user ID
func (s *Service) ValidateAPIKey(ctx context.Context, rawKey string) (*sqlc.ApiKey, error) {
	// Note: In production, you'd want to implement caching here to avoid
	// hitting the database for every API request

	// Since we hash the keys, we need to fetch all keys and check each one
	// In a production system, you might want to use a different approach,
	// such as storing a prefix or using a cache

	// For now, this is a simplified implementation
	// In reality, Immich stores the key in a way that allows direct lookup
	return nil, fmt.Errorf("ValidateAPIKey not fully implemented - needs optimization")
}
