package services

import (
	"errors"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AlbumService struct {
	db *gorm.DB
}

func NewAlbumService(db *gorm.DB) *AlbumService {
	return &AlbumService{db: db}
}

type CreateAlbumRequest struct {
	AlbumName   string      `json:"albumName" binding:"required"`
	Description *string     `json:"description,omitempty"`
	AssetIds    []uuid.UUID `json:"assetIds,omitempty"`
	SharedWith  []uuid.UUID `json:"sharedWith,omitempty"`
}

type UpdateAlbumRequest struct {
	AlbumName         *string `json:"albumName,omitempty"`
	Description       *string `json:"description,omitempty"`
	IsActivityEnabled *bool   `json:"isActivityEnabled,omitempty"`
}

type AddAssetsRequest struct {
	AssetIds []uuid.UUID `json:"ids" binding:"required"`
}

type RemoveAssetsRequest struct {
	AssetIds []uuid.UUID `json:"ids" binding:"required"`
}

type AlbumResponse struct {
	ID                         uuid.UUID       `json:"id"`
	AlbumName                  string          `json:"albumName"`
	Description                string          `json:"description"`
	CreatedAt                  time.Time       `json:"createdAt"`
	UpdatedAt                  time.Time       `json:"updatedAt"`
	AlbumThumbnailAssetId      *uuid.UUID      `json:"albumThumbnailAssetId"`
	Shared                     bool            `json:"shared"`
	HasSharedLink              bool            `json:"hasSharedLink"`
	IsActivityEnabled          bool            `json:"isActivityEnabled"`
	Order                      string          `json:"order"`
	Owner                      UserResponse    `json:"owner"`
	SharedUsers                []UserResponse  `json:"sharedUsers"`
	Assets                     []AssetResponse `json:"assets"`
	AssetCount                 int             `json:"assetCount"`
	LastModifiedAssetTimestamp *time.Time      `json:"lastModifiedAssetTimestamp"`
	StartDate                  *time.Time      `json:"startDate"`
	EndDate                    *time.Time      `json:"endDate"`
}





func (s *AlbumService) GetAllAlbums(userID uuid.UUID, shared *bool) ([]AlbumResponse, error) {
	var albums []models.Album
	query := s.db.Preload("Owner").Preload("SharedUsers").Preload("Assets")

	if shared != nil {
		if *shared {
			// Get albums shared with user
			query = query.Joins("JOIN album_shared_users asu ON albums.id = asu.album_id").
				Where("asu.user_id = ?", userID)
		} else {
			// Get albums owned by user
			query = query.Where("owner_id = ?", userID)
		}
	} else {
		// Get all albums (owned and shared)
		query = query.Where("owner_id = ? OR id IN (SELECT album_id FROM album_shared_users WHERE user_id = ?)", userID, userID)
	}

	if err := query.Find(&albums).Error; err != nil {
		return nil, err
	}

	responses := make([]AlbumResponse, len(albums))
	for i, album := range albums {
		responses[i] = s.toAlbumResponse(album)
	}

	return responses, nil
}

func (s *AlbumService) GetAlbumByID(albumID uuid.UUID, userID uuid.UUID) (*AlbumResponse, error) {
	var album models.Album
	if err := s.db.Preload("Owner").Preload("SharedUsers").Preload("Assets").
		Where("id = ? AND (owner_id = ? OR id IN (SELECT album_id FROM album_shared_users WHERE user_id = ?))",
			albumID, userID, userID).First(&album).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("album not found")
		}
		return nil, err
	}

	response := s.toAlbumResponse(album)
	return &response, nil
}

func (s *AlbumService) CreateAlbum(userID uuid.UUID, req CreateAlbumRequest) (*AlbumResponse, error) {
	album := models.Album{
		Name:              req.AlbumName,
		Description:       "",
		OwnerID:           userID,
		IsActivityEnabled: true,
	}

	if req.Description != nil {
		album.Description = *req.Description
	}

	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&album).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Add shared users
	if len(req.SharedWith) > 0 {
		for _, userID := range req.SharedWith {
			sharedUser := models.AlbumSharedUser{
				AlbumID: album.ID,
				UserID:  userID,
			}
			if err := tx.Create(&sharedUser).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	// Add assets
	if len(req.AssetIds) > 0 {
		for _, assetID := range req.AssetIds {
			albumAsset := models.AlbumAsset{
				AlbumID: album.ID,
				AssetID: assetID,
			}
			if err := tx.Create(&albumAsset).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	// Reload with associations
	if err := s.db.Preload("Owner").Preload("SharedUsers").Preload("Assets").
		Where("id = ?", album.ID).First(&album).Error; err != nil {
		return nil, err
	}

	response := s.toAlbumResponse(album)
	return &response, nil
}

func (s *AlbumService) UpdateAlbum(albumID uuid.UUID, userID uuid.UUID, req UpdateAlbumRequest) (*AlbumResponse, error) {
	var album models.Album
	if err := s.db.Where("id = ? AND owner_id = ?", albumID, userID).First(&album).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("album not found or access denied")
		}
		return nil, err
	}

	if req.AlbumName != nil {
		album.Name = *req.AlbumName
	}
	if req.Description != nil {
		album.Description = *req.Description
	}
	if req.IsActivityEnabled != nil {
		album.IsActivityEnabled = *req.IsActivityEnabled
	}

	if err := s.db.Save(&album).Error; err != nil {
		return nil, err
	}

	// Reload with associations
	if err := s.db.Preload("Owner").Preload("SharedUsers").Preload("Assets").
		Where("id = ?", album.ID).First(&album).Error; err != nil {
		return nil, err
	}

	response := s.toAlbumResponse(album)
	return &response, nil
}

func (s *AlbumService) DeleteAlbum(albumID uuid.UUID, userID uuid.UUID) error {
	// Check if user owns the album
	var album models.Album
	if err := s.db.Where("id = ? AND owner_id = ?", albumID, userID).First(&album).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("album not found or access denied")
		}
		return err
	}

	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete album assets
	if err := tx.Where("album_id = ?", albumID).Delete(&models.AlbumAsset{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete shared users
	if err := tx.Where("album_id = ?", albumID).Delete(&models.AlbumSharedUser{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete album
	if err := tx.Delete(&album).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (s *AlbumService) AddAssets(albumID uuid.UUID, userID uuid.UUID, req AddAssetsRequest) (*AlbumResponse, error) {
	// Check access to album
	var album models.Album
	if err := s.db.Where("id = ? AND (owner_id = ? OR id IN (SELECT album_id FROM album_shared_users WHERE user_id = ?))",
		albumID, userID, userID).First(&album).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("album not found or access denied")
		}
		return nil, err
	}

	// Add assets
	for _, assetID := range req.AssetIds {
		// Check if asset already exists in album
		var existingAlbumAsset models.AlbumAsset
		if err := s.db.Where("album_id = ? AND asset_id = ?", albumID, assetID).First(&existingAlbumAsset).Error; err == nil {
			continue // Asset already in album
		}

		albumAsset := models.AlbumAsset{
			AlbumID: albumID,
			AssetID: assetID,
		}
		if err := s.db.Create(&albumAsset).Error; err != nil {
			return nil, err
		}
	}

	// Return updated album
	return s.GetAlbumByID(albumID, userID)
}

func (s *AlbumService) RemoveAssets(albumID uuid.UUID, userID uuid.UUID, req RemoveAssetsRequest) (*AlbumResponse, error) {
	// Check access to album
	var album models.Album
	if err := s.db.Where("id = ? AND (owner_id = ? OR id IN (SELECT album_id FROM album_shared_users WHERE user_id = ?))",
		albumID, userID, userID).First(&album).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("album not found or access denied")
		}
		return nil, err
	}

	// Remove assets
	if err := s.db.Where("album_id = ? AND asset_id IN ?", albumID, req.AssetIds).
		Delete(&models.AlbumAsset{}).Error; err != nil {
		return nil, err
	}

	// Return updated album
	return s.GetAlbumByID(albumID, userID)
}

func (s *AlbumService) AddUsersToAlbum(albumID uuid.UUID, userIDs []uuid.UUID) error {
	var album models.Album
	if err := s.db.Where("id = ?", albumID).First(&album).Error; err != nil {
		return errors.New("album not found")
	}

	// Add users to album
	for _, userID := range userIDs {
		var user models.User
		if err := s.db.Where("id = ?", userID).First(&user).Error; err == nil {
			s.db.Model(&album).Association("SharedUsers").Append(&user)
		}
	}

	return nil
}

func (s *AlbumService) RemoveUserFromAlbum(albumID, userID uuid.UUID) error {
	var album models.Album
	if err := s.db.Where("id = ?", albumID).First(&album).Error; err != nil {
		return errors.New("album not found")
	}

	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return errors.New("user not found")
	}

	return s.db.Model(&album).Association("SharedUsers").Delete(&user)
}

func (s *AlbumService) toAlbumResponse(album models.Album) AlbumResponse {
	response := AlbumResponse{
		ID:                album.ID,
		AlbumName:         album.Name,
		Description:       album.Description,
		CreatedAt:         album.CreatedAt,
		UpdatedAt:         album.UpdatedAt,
		Shared:            false, // TODO: implement shared users
		HasSharedLink:     false, // TODO: implement shared links
		IsActivityEnabled: album.IsActivityEnabled,
		Order:             "DESC", // Default order since not in model
		Assets:            make([]AssetResponse, len(album.Assets)),
		AssetCount:        len(album.Assets),
		SharedUsers:       make([]UserResponse, 0), // TODO: implement shared users
	}

	// Convert owner
	if album.Owner.ID != uuid.Nil {
		userService := &UserService{db: s.db}
		response.Owner = userService.toUserResponse(album.Owner)
	}

	// TODO: Convert shared users when implemented

	// Convert assets
	for i, asset := range album.Assets {
		response.Assets[i] = toAssetResponse(asset)
	}

	// Set dates from assets
	if len(album.Assets) > 0 {
		// Find earliest and latest asset dates
		var startDate, endDate *time.Time
		for _, asset := range album.Assets {
			assetDate := asset.CreatedAt
			if !asset.FileCreatedAt.IsZero() {
				assetDate = asset.FileCreatedAt
			}

			if startDate == nil || assetDate.Before(*startDate) {
				startDate = &assetDate
			}
			if endDate == nil || assetDate.After(*endDate) {
				endDate = &assetDate
			}
		}
		response.StartDate = startDate
		response.EndDate = endDate

		// Set thumbnail to first asset
		if len(album.Assets) > 0 {
			response.AlbumThumbnailAssetId = &album.Assets[0].ID
		}
	}

	return response
}


