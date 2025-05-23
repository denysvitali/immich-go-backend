package services

import (
	"errors"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LibraryService struct {
	db *gorm.DB
}

func NewLibraryService(db *gorm.DB) *LibraryService {
	return &LibraryService{db: db}
}

type CreateLibraryRequest struct {
	Name        string   `json:"name" binding:"required"`
	ImportPaths []string `json:"importPaths,omitempty"`
	Type        string   `json:"type" binding:"required"` // UPLOAD, EXTERNAL
}

type UpdateLibraryRequest struct {
	Name        *string  `json:"name,omitempty"`
	ImportPaths []string `json:"importPaths,omitempty"`
}

type LibraryResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	ImportPaths []string  `json:"importPaths"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	OwnerID     uuid.UUID `json:"ownerId"`
	AssetCount  int       `json:"assetCount"`
}

type LibraryStatsResponse struct {
	Photos int `json:"photos"`
	Videos int `json:"videos"`
	Total  int `json:"total"`
	Usage  int64 `json:"usage"`
}

func (s *LibraryService) GetAllLibraries(userID uuid.UUID) ([]LibraryResponse, error) {
	var libraries []models.Library
	if err := s.db.Where("owner_id = ?", userID).Find(&libraries).Error; err != nil {
		return nil, err
	}

	responses := make([]LibraryResponse, len(libraries))
	for i, library := range libraries {
		responses[i] = s.toLibraryResponse(library)
	}

	return responses, nil
}

func (s *LibraryService) GetLibraryByID(libraryID uuid.UUID, userID uuid.UUID) (*LibraryResponse, error) {
	var library models.Library
	if err := s.db.Where("id = ? AND owner_id = ?", libraryID, userID).First(&library).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("library not found")
		}
		return nil, err
	}

	response := s.toLibraryResponse(library)
	return &response, nil
}

func (s *LibraryService) CreateLibrary(userID uuid.UUID, req CreateLibraryRequest) (*LibraryResponse, error) {
	library := models.Library{
		ID:          uuid.New(),
		Name:        req.Name,
		Type:        req.Type,
		ImportPaths: req.ImportPaths,
		OwnerID:     userID,
	}

	if err := s.db.Create(&library).Error; err != nil {
		return nil, err
	}

	response := s.toLibraryResponse(library)
	return &response, nil
}

func (s *LibraryService) UpdateLibrary(libraryID uuid.UUID, userID uuid.UUID, req UpdateLibraryRequest) (*LibraryResponse, error) {
	var library models.Library
	if err := s.db.Where("id = ? AND owner_id = ?", libraryID, userID).First(&library).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("library not found")
		}
		return nil, err
	}

	if req.Name != nil {
		library.Name = *req.Name
	}
	if req.ImportPaths != nil {
		library.ImportPaths = req.ImportPaths
	}

	if err := s.db.Save(&library).Error; err != nil {
		return nil, err
	}

	response := s.toLibraryResponse(library)
	return &response, nil
}

func (s *LibraryService) DeleteLibrary(libraryID uuid.UUID, userID uuid.UUID) error {
	var library models.Library
	if err := s.db.Where("id = ? AND owner_id = ?", libraryID, userID).First(&library).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("library not found")
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

	// Delete associated assets
	if err := tx.Where("library_id = ?", libraryID).Delete(&models.Asset{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete library
	if err := tx.Delete(&library).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (s *LibraryService) GetLibraryStatistics(libraryID uuid.UUID, userID uuid.UUID) (*LibraryStatsResponse, error) {
	// Check if user has access to library
	var library models.Library
	if err := s.db.Where("id = ? AND owner_id = ?", libraryID, userID).First(&library).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("library not found")
		}
		return nil, err
	}

	var stats struct {
		TotalPhotos int
		TotalVideos int
		TotalSize   int64
	}

	// Count photos
	if err := s.db.Model(&models.Asset{}).
		Where("library_id = ? AND type = ?", libraryID, "IMAGE").
		Count(&stats.TotalPhotos).Error; err != nil {
		return nil, err
	}

	// Count videos
	if err := s.db.Model(&models.Asset{}).
		Where("library_id = ? AND type = ?", libraryID, "VIDEO").
		Count(&stats.TotalVideos).Error; err != nil {
		return nil, err
	}

	// TODO: Calculate total size from file system or store in database
	stats.TotalSize = 0

	return &LibraryStatsResponse{
		Photos: stats.TotalPhotos,
		Videos: stats.TotalVideos,
		Total:  stats.TotalPhotos + stats.TotalVideos,
		Usage:  stats.TotalSize,
	}, nil
}

func (s *LibraryService) ScanLibrary(libraryID uuid.UUID, userID uuid.UUID) error {
	// Check if user has access to library
	var library models.Library
	if err := s.db.Where("id = ? AND owner_id = ?", libraryID, userID).First(&library).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("library not found")
		}
		return err
	}

	// TODO: Implement library scanning logic
	// This would scan the import paths and create/update assets
	return nil
}

func (s *LibraryService) toLibraryResponse(library models.Library) LibraryResponse {
	// Get asset count
	var assetCount int64
	s.db.Model(&models.Asset{}).Where("library_id = ?", library.ID).Count(&assetCount)

	return LibraryResponse{
		ID:          library.ID,
		Name:        library.Name,
		Type:        library.Type,
		ImportPaths: library.ImportPaths,
		CreatedAt:   library.CreatedAt,
		UpdatedAt:   library.UpdatedAt,
		OwnerID:     library.OwnerID,
		AssetCount:  int(assetCount),
	}
}
