package apikeys

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

// hashAPIKey hashes an API key for storage/lookup
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
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

	// Look up the API key directly in the database
	// The key is stored hashed in the database, so we compare the hashed value
	apiKey, err := s.db.GetApiKey(ctx, hashAPIKey(rawKey))
	if err != nil {
		return nil, fmt.Errorf("invalid API key: %w", err)
	}

	// Note: API keys don't have expiration in the current schema
	// This could be added as a future enhancement

	return &apiKey, nil
}
