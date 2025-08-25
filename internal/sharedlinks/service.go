package sharedlinks

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
)

// Service handles shared link operations
type Service struct {
	db *sqlc.Queries
}

// NewService creates a new shared links service
func NewService(db *sqlc.Queries) *Service {
	return &Service{db: db}
}

// SharedLink represents a shared link
type SharedLink struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"userId"`
	Key              string     `json:"key"`
	Type             string     `json:"type"`
	Description      string     `json:"description,omitempty"`
	Password         string     `json:"-"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	AllowDownload    bool       `json:"allowDownload"`
	AllowUpload      bool       `json:"allowUpload"`
	ShowMetadata     bool       `json:"showMetadata"`
	AssetCount       int        `json:"assetCount"`
	AlbumID          *uuid.UUID `json:"albumId,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// CreateSharedLinkRequest represents a request to create a shared link
type CreateSharedLinkRequest struct {
	Type            string     `json:"type"`
	AssetIDs        []string   `json:"assetIds,omitempty"`
	AlbumID         *string    `json:"albumId,omitempty"`
	Description     string     `json:"description,omitempty"`
	Password        string     `json:"password,omitempty"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
	AllowDownload   bool       `json:"allowDownload"`
	AllowUpload     bool       `json:"allowUpload"`
	ShowMetadata    bool       `json:"showMetadata"`
}

// UpdateSharedLinkRequest represents a request to update a shared link
type UpdateSharedLinkRequest struct {
	Description     *string    `json:"description,omitempty"`
	Password        *string    `json:"password,omitempty"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
	AllowDownload   *bool      `json:"allowDownload,omitempty"`
	AllowUpload     *bool      `json:"allowUpload,omitempty"`
	ShowMetadata    *bool      `json:"showMetadata,omitempty"`
	AssetIDs        []string   `json:"assetIds,omitempty"`
}

// CreateSharedLink creates a new shared link
func (s *Service) CreateSharedLink(ctx context.Context, userID uuid.UUID, req *CreateSharedLinkRequest) (*SharedLink, error) {
	// Generate a unique key for the shared link
	key, err := generateShareKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate share key: %w", err)
	}

	// Hash password if provided
	var hashedPassword []byte
	if req.Password != "" {
		hashedPassword, err = bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
	}

	// Convert UUID strings to UUIDs
	var albumID pgtype.UUID
	if req.AlbumID != nil {
		aid, err := uuid.Parse(*req.AlbumID)
		if err != nil {
			return nil, fmt.Errorf("invalid album ID: %w", err)
		}
		albumID = pgtype.UUID{
			Bytes: aid,
			Valid: true,
		}
	}

	// Create the shared link in database
	params := sqlc.CreateSharedLinkParams{
		ID:            uuid.New(),
		UserID:        pgtype.UUID{Bytes: userID, Valid: true},
		Key:           key,
		Type:          req.Type,
		Description:   pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Password:      hashedPassword,
		AllowDownload: req.AllowDownload,
		AllowUpload:   req.AllowUpload,
		ShowMetadata:  req.ShowMetadata,
		AlbumID:       albumID,
	}

	if req.ExpiresAt != nil {
		params.ExpiresAt = pgtype.Timestamptz{
			Time:  *req.ExpiresAt,
			Valid: true,
		}
	}

	link, err := s.db.CreateSharedLink(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create shared link: %w", err)
	}

	// Add assets to the shared link if provided
	if len(req.AssetIDs) > 0 {
		for _, assetIDStr := range req.AssetIDs {
			assetID, err := uuid.Parse(assetIDStr)
			if err != nil {
				continue
			}

			err = s.db.AddAssetToSharedLink(ctx, sqlc.AddAssetToSharedLinkParams{
				SharedLinkID: link.ID,
				AssetID:      pgtype.UUID{Bytes: assetID, Valid: true},
			})
			if err != nil {
				// Log error but continue
				continue
			}
		}
	}

	return s.convertToSharedLink(&link), nil
}

// GetSharedLink retrieves a shared link by ID
func (s *Service) GetSharedLink(ctx context.Context, linkID uuid.UUID) (*SharedLink, error) {
	link, err := s.db.GetSharedLink(ctx, linkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shared link: %w", err)
	}

	return s.convertToSharedLink(&link), nil
}

// GetSharedLinkByKey retrieves a shared link by its key
func (s *Service) GetSharedLinkByKey(ctx context.Context, key string) (*SharedLink, error) {
	link, err := s.db.GetSharedLinkByKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get shared link by key: %w", err)
	}

	// Check if link is expired
	if link.ExpiresAt.Valid && link.ExpiresAt.Time.Before(time.Now()) {
		return nil, fmt.Errorf("shared link has expired")
	}

	return s.convertToSharedLink(&link), nil
}

// ValidatePassword validates the password for a shared link
func (s *Service) ValidatePassword(ctx context.Context, linkID uuid.UUID, password string) error {
	link, err := s.db.GetSharedLink(ctx, linkID)
	if err != nil {
		return fmt.Errorf("failed to get shared link: %w", err)
	}

	if len(link.Password) == 0 {
		return nil // No password required
	}

	err = bcrypt.CompareHashAndPassword(link.Password, []byte(password))
	if err != nil {
		return fmt.Errorf("invalid password")
	}

	return nil
}

// ListSharedLinks lists all shared links for a user
func (s *Service) ListSharedLinks(ctx context.Context, userID uuid.UUID) ([]*SharedLink, error) {
	links, err := s.db.ListSharedLinks(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list shared links: %w", err)
	}

	result := make([]*SharedLink, len(links))
	for i, link := range links {
		result[i] = s.convertToSharedLink(&link)
	}

	return result, nil
}

// UpdateSharedLink updates an existing shared link
func (s *Service) UpdateSharedLink(ctx context.Context, linkID uuid.UUID, req *UpdateSharedLinkRequest) (*SharedLink, error) {
	// Get existing link
	existing, err := s.db.GetSharedLink(ctx, linkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shared link: %w", err)
	}

	// Update fields
	params := sqlc.UpdateSharedLinkParams{
		ID:            linkID,
		Description:   existing.Description,
		AllowDownload: existing.AllowDownload,
		AllowUpload:   existing.AllowUpload,
		ShowMetadata:  existing.ShowMetadata,
		ExpiresAt:     existing.ExpiresAt,
	}

	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: *req.Description != ""}
	}

	if req.AllowDownload != nil {
		params.AllowDownload = *req.AllowDownload
	}

	if req.AllowUpload != nil {
		params.AllowUpload = *req.AllowUpload
	}

	if req.ShowMetadata != nil {
		params.ShowMetadata = *req.ShowMetadata
	}

	if req.ExpiresAt != nil {
		params.ExpiresAt = pgtype.Timestamptz{
			Time:  *req.ExpiresAt,
			Valid: true,
		}
	}

	// Update password if provided
	if req.Password != nil {
		if *req.Password == "" {
			// Remove password
			params.Password = nil
		} else {
			// Hash new password
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
			if err != nil {
				return nil, fmt.Errorf("failed to hash password: %w", err)
			}
			params.Password = hashedPassword
		}
	} else {
		params.Password = existing.Password
	}

	link, err := s.db.UpdateSharedLink(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update shared link: %w", err)
	}

	// Update assets if provided
	if len(req.AssetIDs) > 0 {
		// Remove existing assets
		err = s.db.RemoveAllAssetsFromSharedLink(ctx, linkID)
		if err != nil {
			return nil, fmt.Errorf("failed to remove existing assets: %w", err)
		}

		// Add new assets
		for _, assetIDStr := range req.AssetIDs {
			assetID, err := uuid.Parse(assetIDStr)
			if err != nil {
				continue
			}

			err = s.db.AddAssetToSharedLink(ctx, sqlc.AddAssetToSharedLinkParams{
				SharedLinkID: linkID,
				AssetID:      pgtype.UUID{Bytes: assetID, Valid: true},
			})
			if err != nil {
				continue
			}
		}
	}

	return s.convertToSharedLink(&link), nil
}

// DeleteSharedLink deletes a shared link
func (s *Service) DeleteSharedLink(ctx context.Context, linkID uuid.UUID) error {
	err := s.db.DeleteSharedLink(ctx, linkID)
	if err != nil {
		return fmt.Errorf("failed to delete shared link: %w", err)
	}
	return nil
}

// GetSharedLinkAssets retrieves assets for a shared link
func (s *Service) GetSharedLinkAssets(ctx context.Context, linkID uuid.UUID) ([]uuid.UUID, error) {
	assets, err := s.db.GetSharedLinkAssets(ctx, linkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get shared link assets: %w", err)
	}

	result := make([]uuid.UUID, len(assets))
	for i, asset := range assets {
		if asset.Valid {
			result[i] = asset.Bytes
		}
	}

	return result, nil
}

// AddAssetsToSharedLink adds assets to a shared link
func (s *Service) AddAssetsToSharedLink(ctx context.Context, linkID uuid.UUID, assetIDs []uuid.UUID) error {
	for _, assetID := range assetIDs {
		err := s.db.AddAssetToSharedLink(ctx, sqlc.AddAssetToSharedLinkParams{
			SharedLinkID: linkID,
			AssetID:      pgtype.UUID{Bytes: assetID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to add asset to shared link: %w", err)
		}
	}
	return nil
}

// RemoveAssetsFromSharedLink removes assets from a shared link
func (s *Service) RemoveAssetsFromSharedLink(ctx context.Context, linkID uuid.UUID, assetIDs []uuid.UUID) error {
	for _, assetID := range assetIDs {
		err := s.db.RemoveAssetFromSharedLink(ctx, sqlc.RemoveAssetFromSharedLinkParams{
			SharedLinkID: linkID,
			AssetID:      pgtype.UUID{Bytes: assetID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to remove asset from shared link: %w", err)
		}
	}
	return nil
}

// convertToSharedLink converts a database shared link to service model
func (s *Service) convertToSharedLink(dbLink *sqlc.SharedLink) *SharedLink {
	link := &SharedLink{
		ID:            dbLink.ID,
		Key:           dbLink.Key,
		Type:          dbLink.Type,
		AllowDownload: dbLink.AllowDownload,
		AllowUpload:   dbLink.AllowUpload,
		ShowMetadata:  dbLink.ShowMetadata,
		CreatedAt:     dbLink.CreatedAt.Time,
		UpdatedAt:     dbLink.UpdatedAt.Time,
	}

	if dbLink.UserID.Valid {
		link.UserID = dbLink.UserID.Bytes
	}

	if dbLink.Description.Valid {
		link.Description = dbLink.Description.String
	}

	if dbLink.ExpiresAt.Valid {
		link.ExpiresAt = &dbLink.ExpiresAt.Time
	}

	if dbLink.AlbumID.Valid {
		albumID := dbLink.AlbumID.Bytes
		link.AlbumID = &albumID
	}

	// Get asset count
	if dbLink.AssetCount > 0 {
		link.AssetCount = int(dbLink.AssetCount)
	}

	return link
}

// generateShareKey generates a unique key for a shared link
func generateShareKey() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// IsNotFoundError checks if error is a not found error
func IsNotFoundError(err error) bool {
	// Implementation would check for specific database not found errors
	return false
}

// IsValidationError checks if error is a validation error
func IsValidationError(err error) bool {
	// Implementation would check for validation errors
	return false
}