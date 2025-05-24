package services

import (
	"errors"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/models"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AssetService struct {
	db *gorm.DB
}

func NewAssetService(db *gorm.DB) *AssetService {
	return &AssetService{db: db}
}

type CreateAssetRequest struct {
	DeviceAssetId    string     `json:"deviceAssetId" binding:"required"`
	DeviceID         string     `json:"deviceId" binding:"required"`
	Type             string     `json:"type" binding:"required"`
	OriginalPath     string     `json:"originalPath" binding:"required"`
	OriginalFileName string     `json:"originalFileName" binding:"required"`
	ResizePath       *string    `json:"resizePath,omitempty"`
	WebpPath         *string    `json:"webpPath,omitempty"`
	ThumbhashPath    *string    `json:"thumbhashPath,omitempty"`
	EncodedVideoPath *string    `json:"encodedVideoPath,omitempty"`
	Duration         *string    `json:"duration,omitempty"`
	IsVisible        *bool      `json:"isVisible,omitempty"`
	IsFavorite       *bool      `json:"isFavorite,omitempty"`
	IsArchived       *bool      `json:"isArchived,omitempty"`
	FileCreatedAt    *time.Time `json:"fileCreatedAt,omitempty"`
	FileModifiedAt   *time.Time `json:"fileModifiedAt,omitempty"`
	LibraryId        *uuid.UUID `json:"libraryId,omitempty"`
}

type UpdateAssetRequest struct {
	IsFavorite       *bool      `json:"isFavorite,omitempty"`
	IsArchived       *bool      `json:"isArchived,omitempty"`
	Description      *string    `json:"description,omitempty"`
	DateTimeOriginal *time.Time `json:"dateTimeOriginal,omitempty"`
	Latitude         *float64   `json:"latitude,omitempty"`
	Longitude        *float64   `json:"longitude,omitempty"`
}

type AssetStatsResponse struct {
	Images int `json:"images"`
	Videos int `json:"videos"`
	Total  int `json:"total"`
}

type AssetSearchOptions struct {
	UserID       uuid.UUID
	Type         *string
	IsFavorite   *bool
	IsArchived   *bool
	IsTrashed    *bool
	City         *string
	State        *string
	Country      *string
	Make         *string
	Model        *string
	TakenAfter   *time.Time
	TakenBefore  *time.Time
	OriginalPath *string
	LibraryId    *uuid.UUID
	Page         int
	Size         int
}

func (s *AssetService) GetAllAssets(userID uuid.UUID, options AssetSearchOptions) ([]*immichv1.Asset, error) {
	var assets []models.Asset
	query := s.db.Where("owner_id = ?", userID)

	// Apply filters
	if options.Type != nil {
		query = query.Where("type = ?", *options.Type)
	}
	if options.IsFavorite != nil {
		query = query.Where("is_favorite = ?", *options.IsFavorite)
	}
	if options.IsArchived != nil {
		query = query.Where("is_archived = ?", *options.IsArchived)
	}
	if options.IsTrashed != nil {
		query = query.Where("is_trashed = ?", *options.IsTrashed)
	}
	if options.LibraryId != nil {
		query = query.Where("library_id = ?", *options.LibraryId)
	}
	if options.TakenAfter != nil {
		query = query.Where("file_created_at >= ?", *options.TakenAfter)
	}
	if options.TakenBefore != nil {
		query = query.Where("file_created_at <= ?", *options.TakenBefore)
	}
	if options.OriginalPath != nil {
		query = query.Where("original_path LIKE ?", "%"+*options.OriginalPath+"%")
	}

	// Apply pagination
	if options.Size > 0 {
		offset := options.Page * options.Size
		query = query.Offset(offset).Limit(options.Size)
	}

	// Order by creation date (newest first)
	query = query.Order("created_at DESC")

	if err := query.Find(&assets).Error; err != nil {
		return nil, err
	}

	responses := make([]*immichv1.Asset, len(assets))
	for i, asset := range assets {
		responses[i] = toAssetProto(asset)
	}

	return responses, nil
}

func (s *AssetService) GetAssetByID(assetID uuid.UUID, userID uuid.UUID) (*immichv1.Asset, error) {
	var asset models.Asset
	if err := s.db.Where("id = ? AND owner_id = ?", assetID, userID).First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("asset not found")
		}
		return nil, err
	}

	response := toAssetProto(asset)
	return response, nil
}

func (s *AssetService) CreateAsset(userID uuid.UUID, req CreateAssetRequest) (*immichv1.Asset, error) {
	// Check if asset already exists
	var existingAsset models.Asset
	if err := s.db.Where("device_asset_id = ? AND device_id = ? AND owner_id = ?",
		req.DeviceAssetId, req.DeviceID, userID).First(&existingAsset).Error; err == nil {
		return nil, errors.New("asset already exists")
	}

	asset := models.Asset{
		DeviceAssetID:    req.DeviceAssetId,
		OwnerID:          userID,
		DeviceID:         req.DeviceID,
		Type:             req.Type,
		OriginalPath:     req.OriginalPath,
		OriginalFileName: req.OriginalFileName,
		IsVisible:        true,
		IsFavorite:       false,
		IsArchived:       false,
		IsTrashed:        false,
	}
	
	// Set duration if provided
	if req.Duration != nil {
		asset.Duration = *req.Duration
	}
	
	// Set file timestamps if provided
	if req.FileCreatedAt != nil {
		asset.FileCreatedAt = *req.FileCreatedAt
	}
	if req.FileModifiedAt != nil {
		asset.FileModifiedAt = *req.FileModifiedAt
	}

	if req.IsVisible != nil {
		asset.IsVisible = *req.IsVisible
	}
	if req.IsFavorite != nil {
		asset.IsFavorite = *req.IsFavorite
	}
	if req.IsArchived != nil {
		asset.IsArchived = *req.IsArchived
	}

	if err := s.db.Create(&asset).Error; err != nil {
		return nil, err
	}

	response := toAssetProto(asset)
	return response, nil
}

func (s *AssetService) UpdateAsset(assetID uuid.UUID, userID uuid.UUID, req UpdateAssetRequest) (*immichv1.Asset, error) {
	var asset models.Asset
	if err := s.db.Where("id = ? AND owner_id = ?", assetID, userID).First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("asset not found")
		}
		return nil, err
	}

	if req.IsFavorite != nil {
		asset.IsFavorite = *req.IsFavorite
	}
	if req.IsArchived != nil {
		asset.IsArchived = *req.IsArchived
	}

	if err := s.db.Save(&asset).Error; err != nil {
		return nil, err
	}

	response := toAssetProto(asset)
	return response, nil
}

func (s *AssetService) DeleteAsset(assetID uuid.UUID, userID uuid.UUID) error {
	var asset models.Asset
	if err := s.db.Where("id = ? AND owner_id = ?", assetID, userID).First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("asset not found")
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

	// Remove from albums
	if err := tx.Where("asset_id = ?", assetID).Delete(&models.AlbumAsset{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete asset
	if err := tx.Delete(&asset).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (s *AssetService) TrashAssets(userID uuid.UUID, assetIDs []uuid.UUID) error {
	return s.db.Model(&models.Asset{}).
		Where("id IN ? AND owner_id = ?", assetIDs, userID).
		Update("is_trashed", true).Error
}

func (s *AssetService) RestoreAssets(userID uuid.UUID, assetIDs []uuid.UUID) error {
	return s.db.Model(&models.Asset{}).
		Where("id IN ? AND owner_id = ?", assetIDs, userID).
		Update("is_trashed", false).Error
}

func (s *AssetService) GetAssetStatistics(userID uuid.UUID) (*AssetStatsResponse, error) {
	var stats struct {
		TotalImages int64
		TotalVideos int64
		Total       int64
	}

	// Count images
	if err := s.db.Model(&models.Asset{}).
		Where("owner_id = ? AND type = ? AND is_trashed = ?", userID, "IMAGE", false).
		Count(&stats.TotalImages).Error; err != nil {
		return nil, err
	}

	// Count videos
	if err := s.db.Model(&models.Asset{}).
		Where("owner_id = ? AND type = ? AND is_trashed = ?", userID, "VIDEO", false).
		Count(&stats.TotalVideos).Error; err != nil {
		return nil, err
	}

	stats.Total = stats.TotalImages + stats.TotalVideos

	return &AssetStatsResponse{
		Images: int(stats.TotalImages),
		Videos: int(stats.TotalVideos),
		Total:  int(stats.Total),
	}, nil
}

func (s *AssetService) GetMemoryLane(userID uuid.UUID, day int, month int) ([]*immichv1.Asset, error) {
	var assets []models.Asset

	// Get assets from previous years on this day
	query := s.db.Where("owner_id = ? AND is_trashed = ? AND is_archived = ?", userID, false, false)

	// Filter by month and day from file_created_at or created_at
	query = query.Where(
		"(EXTRACT(MONTH FROM COALESCE(file_created_at, created_at)) = ? AND EXTRACT(DAY FROM COALESCE(file_created_at, created_at)) = ?) AND EXTRACT(YEAR FROM COALESCE(file_created_at, created_at)) < ?",
		month, day, time.Now().Year(),
	)

	query = query.Order("COALESCE(file_created_at, created_at) DESC").Limit(20)

	if err := query.Find(&assets).Error; err != nil {
		return nil, err
	}

	responses := make([]*immichv1.Asset, len(assets))
	for i, asset := range assets {
		responses[i] = toAssetProto(asset)
	}

	return responses, nil
}

func (s *AssetService) GetAssetThumbnail(assetID uuid.UUID, userID uuid.UUID, format string) (string, error) {
	var asset models.Asset
	if err := s.db.Where("id = ? AND owner_id = ?", assetID, userID).First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("asset not found")
		}
		return "", err
	}

	// TODO: Add thumbnail path fields to Asset model
	// For now, return original path
	return asset.OriginalPath, nil
}

func (s *AssetService) CheckDuplicateAsset(userID uuid.UUID, deviceAssetId string, deviceId string) (bool, error) {
	var count int64
	if err := s.db.Model(&models.Asset{}).
		Where("device_asset_id = ? AND device_id = ? AND owner_id = ?", deviceAssetId, deviceId, userID).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (s *AssetService) CheckExistingAssets(userID uuid.UUID, deviceAssetIds []string, deviceId string) (map[string]bool, error) {
	var assets []models.Asset
	if err := s.db.Select("device_asset_id").
		Where("device_asset_id IN ? AND device_id = ? AND owner_id = ?", deviceAssetIds, deviceId, userID).
		Find(&assets).Error; err != nil {
		return nil, err
	}

	existing := make(map[string]bool)
	for _, deviceAssetId := range deviceAssetIds {
		existing[deviceAssetId] = false
	}

	for _, asset := range assets {
		existing[asset.DeviceAssetID] = true
	}

	return existing, nil
}

func (s *AssetService) BulkUploadCheck(userID uuid.UUID, assetData []CreateAssetRequest) ([]*immichv1.Asset, error) {
	var newAssets []models.Asset

	for _, req := range assetData {
		// Check if asset already exists
		var existingAsset models.Asset
		if err := s.db.Where("device_asset_id = ? AND device_id = ? AND owner_id = ?",
			req.DeviceAssetId, req.DeviceID, userID).First(&existingAsset).Error; err == nil {
			continue // Skip existing asset
		}

		asset := models.Asset{
			DeviceAssetID:    req.DeviceAssetId,
			OwnerID:          userID,
			DeviceID:         req.DeviceID,
			Type:             req.Type,
			OriginalPath:     req.OriginalPath,
			OriginalFileName: req.OriginalFileName,
			IsVisible:        true,
			IsFavorite:       false,
			IsArchived:       false,
			IsTrashed:        false,
		}
		
		// Set duration if provided
		if req.Duration != nil {
			asset.Duration = *req.Duration
		}
		
		// Set file timestamps if provided
		if req.FileCreatedAt != nil {
			asset.FileCreatedAt = *req.FileCreatedAt
		}
		if req.FileModifiedAt != nil {
			asset.FileModifiedAt = *req.FileModifiedAt
		}

		if req.IsVisible != nil {
			asset.IsVisible = *req.IsVisible
		}
		if req.IsFavorite != nil {
			asset.IsFavorite = *req.IsFavorite
		}
		if req.IsArchived != nil {
			asset.IsArchived = *req.IsArchived
		}

		newAssets = append(newAssets, asset)
	}

	// Bulk create new assets
	if len(newAssets) > 0 {
		if err := s.db.Create(&newAssets).Error; err != nil {
			return nil, err
		}
	}

	responses := make([]*immichv1.Asset, len(newAssets))
	for i, asset := range newAssets {
		responses[i] = toAssetProto(asset)
	}

	return responses, nil
}