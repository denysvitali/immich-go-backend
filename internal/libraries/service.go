package libraries

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"
)

// LibraryType represents the type of library
type LibraryType string

const (
	LibraryTypeUpload   LibraryType = "UPLOAD"
	LibraryTypeExternal LibraryType = "EXTERNAL"
)

// Library represents a library configuration
type Library struct {
	ID                uuid.UUID
	OwnerID           uuid.UUID
	Name              string
	Type              LibraryType
	ImportPaths       []string
	ExclusionPatterns []string
	IsWatched         bool
	IsVisible         bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	RefreshedAt       *time.Time
	AssetCount        int64
}

// Service manages libraries
type Service struct {
	db             *sqlc.Queries
	config         *config.Config
	storageService *storage.Service
	scanners       map[uuid.UUID]*LibraryScanner
}

// NewService creates a new library service
func NewService(db *sqlc.Queries, config *config.Config, storageService *storage.Service) *Service {
	return &Service{
		db:             db,
		config:         config,
		storageService: storageService,
		scanners:       make(map[uuid.UUID]*LibraryScanner),
	}
}

// CreateLibrary creates a new library
func (s *Service) CreateLibrary(ctx context.Context, userID uuid.UUID, req CreateLibraryRequest) (*Library, error) {
	// Validate request
	if req.Name == "" {
		return nil, fmt.Errorf("library name is required")
	}

	if req.Type == "" {
		req.Type = LibraryTypeExternal
	}

	// Create library in database
	library, err := s.db.CreateLibrary(ctx, sqlc.CreateLibraryParams{
		OwnerId:           UUIDToPgtype(userID),
		Name:              req.Name,
		ImportPaths:       req.ImportPaths,
		ExclusionPatterns: req.ExclusionPatterns,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create library: %w", err)
	}

	return &Library{
		ID:                PgtypeToUUID(library.ID),
		OwnerID:           PgtypeToUUID(library.OwnerId),
		Name:              library.Name,
		Type:              req.Type,
		ImportPaths:       library.ImportPaths,
		ExclusionPatterns: library.ExclusionPatterns,
		IsWatched:         req.IsWatched,
		IsVisible:         req.IsVisible,
		CreatedAt:         PgtypeToTime(library.CreatedAt),
		UpdatedAt:         PgtypeToTime(library.UpdatedAt),
		RefreshedAt: func() *time.Time {
			t := PgtypeToTime(library.RefreshedAt)
			if t.IsZero() {
				return nil
			}
			return &t
		}(),
		AssetCount: 0,
	}, nil
}

// GetLibrary retrieves a library by ID
func (s *Service) GetLibrary(ctx context.Context, userID, libraryID uuid.UUID) (*Library, error) {
	library, err := s.db.GetLibrary(ctx, UUIDToPgtype(libraryID))
	if err != nil {
		return nil, fmt.Errorf("failed to get library: %w", err)
	}

	// Get asset count
	var count int64
	if countResult, err := s.db.CountLibraryAssets(ctx, UUIDToPgtype(libraryID)); err == nil {
		count = countResult
	} else {
		logrus.WithError(err).Warn("Failed to get library asset count")
		count = 0
	}

	return &Library{
		ID:                PgtypeToUUID(library.ID),
		OwnerID:           PgtypeToUUID(library.OwnerId),
		Name:              library.Name,
		Type:              LibraryTypeExternal,
		ImportPaths:       library.ImportPaths,
		ExclusionPatterns: library.ExclusionPatterns,
		IsWatched:         false,
		IsVisible:         true,
		CreatedAt:         PgtypeToTime(library.CreatedAt),
		UpdatedAt:         PgtypeToTime(library.UpdatedAt),
		RefreshedAt: func() *time.Time {
			t := PgtypeToTime(library.RefreshedAt)
			if t.IsZero() {
				return nil
			}
			return &t
		}(),
		AssetCount: count,
	}, nil
}

// GetLibraries retrieves all libraries for a user
func (s *Service) GetLibraries(ctx context.Context, userID uuid.UUID) ([]*Library, error) {
	dbLibraries, err := s.db.GetLibraries(ctx, UUIDToPgtype(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to get libraries: %w", err)
	}

	libraries := make([]*Library, len(dbLibraries))
	for i, dbLib := range dbLibraries {
		// Get asset count for each library
		count, err := s.db.GetLibraryAssetCount(ctx, dbLib.ID)
		if err != nil {
			logrus.WithError(err).Warn("Failed to get library asset count")
			count = 0
		}

		libraries[i] = &Library{
			ID:                PgtypeToUUID(dbLib.ID),
			OwnerID:           PgtypeToUUID(dbLib.OwnerId),
			Name:              dbLib.Name,
			Type:              LibraryTypeExternal, // Default type
			ImportPaths:       dbLib.ImportPaths,
			ExclusionPatterns: dbLib.ExclusionPatterns,
			IsWatched:         false, // Default value
			IsVisible:         true,  // Default value
			CreatedAt:         PgtypeToTime(dbLib.CreatedAt),
			UpdatedAt:         PgtypeToTime(dbLib.UpdatedAt),
			RefreshedAt: func() *time.Time {
				t := PgtypeToTime(dbLib.RefreshedAt)
				if t.IsZero() {
					return nil
				}
				return &t
			}(),
			AssetCount: count,
		}
	}

	return libraries, nil
}

// UpdateLibrary updates a library
func (s *Service) UpdateLibrary(ctx context.Context, userID, libraryID uuid.UUID, req *UpdateLibraryRequest) (*Library, error) {
	// Get existing library
	library, err := s.db.GetLibrary(ctx, UUIDToPgtype(libraryID))
	if err != nil {
		return nil, fmt.Errorf("failed to get library: %w", err)
	}

	// Update fields if provided
	name := library.Name
	if req.Name != nil {
		name = *req.Name
	}
	importPaths := library.ImportPaths
	if req.ImportPaths != nil {
		importPaths = req.ImportPaths
	}
	exclusionPatterns := library.ExclusionPatterns
	if req.ExclusionPatterns != nil {
		exclusionPatterns = req.ExclusionPatterns
	}

	// Update in database
	updatedLibrary, err := s.db.UpdateLibrary(ctx, sqlc.UpdateLibraryParams{
		ID:                UUIDToPgtype(libraryID),
		Name:              pgtype.Text{String: name, Valid: true},
		ImportPaths:       importPaths,
		ExclusionPatterns: exclusionPatterns,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update library: %w", err)
	}

	// Get asset count
	var count int64
	if countResult, err := s.db.CountLibraryAssets(ctx, UUIDToPgtype(libraryID)); err == nil {
		count = countResult
	} else {
		logrus.WithError(err).Warn("Failed to get library asset count")
		count = 0
	}

	return &Library{
		ID:                PgtypeToUUID(updatedLibrary.ID),
		OwnerID:           PgtypeToUUID(updatedLibrary.OwnerId),
		Name:              updatedLibrary.Name,
		Type:              LibraryTypeExternal, // Default type
		ImportPaths:       updatedLibrary.ImportPaths,
		ExclusionPatterns: updatedLibrary.ExclusionPatterns,
		IsWatched:         false, // Default value
		IsVisible:         true,  // Default value
		CreatedAt:         PgtypeToTime(updatedLibrary.CreatedAt),
		UpdatedAt:         PgtypeToTime(updatedLibrary.UpdatedAt),
		RefreshedAt: func() *time.Time {
			t := PgtypeToTime(updatedLibrary.RefreshedAt)
			if t.IsZero() {
				return nil
			}
			return &t
		}(),
		AssetCount: count,
	}, nil
}

// DeleteLibrary deletes a library
func (s *Service) DeleteLibrary(ctx context.Context, userID, libraryID uuid.UUID) error {
	// Stop any active scanning
	if scanner, exists := s.scanners[libraryID]; exists {
		scanner.Stop()
		delete(s.scanners, libraryID)
	}

	// Delete library and associated assets
	if err := s.db.DeleteLibrary(ctx, UUIDToPgtype(libraryID)); err != nil {
		return fmt.Errorf("failed to delete library: %w", err)
	}

	return nil
}

// ScanLibrary starts scanning a library for assets
func (s *Service) ScanLibrary(ctx context.Context, userID, libraryID uuid.UUID, forceRefresh, refreshAllFiles bool) (uuid.UUID, error) {
	// Check if already scanning
	if _, exists := s.scanners[libraryID]; exists {
		return uuid.Nil, fmt.Errorf("library is already being scanned")
	}

	// Get library
	library, err := s.GetLibrary(ctx, userID, libraryID)
	if err != nil {
		return uuid.Nil, err
	}

	// Create and start scanner
	scanner := NewLibraryScanner(library, s.db, s.storageService)
	s.scanners[libraryID] = scanner

	// Start scanning in background
	go func() {
		defer func() {
			delete(s.scanners, libraryID)
		}()

		if err := scanner.Scan(ctx, forceRefresh); err != nil {
			logrus.WithError(err).Error("Library scan failed")
		}

		// Update refresh timestamp
		if err := s.db.UpdateLibraryRefreshedAt(ctx, UUIDToPgtype(libraryID)); err != nil {
			logrus.WithError(err).Error("Failed to update library refresh timestamp")
		}
	}()

	// Return a job ID (simplified for now)
	jobID := uuid.New()
	return jobID, nil
}

// GetLibraryStatistics retrieves statistics for a library
func (s *Service) GetLibraryStatistics(ctx context.Context, userID, libraryID uuid.UUID) (*LibraryStatistics, error) {
	// Get total asset count from database
	totalCount, err := s.db.CountLibraryAssets(ctx, UUIDToPgtype(libraryID))
	if err != nil {
		return nil, fmt.Errorf("failed to get library statistics: %w", err)
	}

	// Return actual count from database
	// A more complete implementation would count by asset type
	return &LibraryStatistics{
		Photos:    totalCount, // Would need separate count by type
		Videos:    0,          // Would need separate count by type
		TotalSize: 0,          // Would need SUM(file_size) query
		Usage:     0,          // Would need calculation against quota
	}, nil
}

// ValidateImportPath validates an import path
func (s *Service) ValidateImportPath(path string) (bool, string) {
	// Basic validation
	if path == "" {
		return false, "Path cannot be empty"
	}

	// Check if path exists
	if _, err := filepath.Abs(path); err != nil {
		return false, "Invalid path"
	}

	return true, ""
}

// ValidateLibrary validates library import paths
func (s *Service) ValidateLibrary(ctx context.Context, req ValidateLibraryRequest) (*ValidateLibraryResponse, error) {
	results := make(map[string]PathValidation)

	for _, path := range req.ImportPaths {
		validation := PathValidation{
			Path:       path,
			IsValid:    false,
			IsReadable: false,
		}

		// Check if path exists
		info, err := filepath.Glob(path)
		if err != nil || len(info) == 0 {
			validation.Message = "Path does not exist"
		} else {
			validation.IsValid = true
			// Check read permissions by attempting to read the directory
			_, readErr := filepath.Glob(filepath.Join(path, "*"))
			if readErr != nil {
				validation.IsReadable = false
				validation.Message = "Path exists but is not readable"
			} else {
				validation.IsReadable = true
				validation.Message = "Path is valid and readable"
			}
		}

		results[path] = validation
	}

	return &ValidateLibraryResponse{
		Results: results,
	}, nil
}

// Request/Response types

type CreateLibraryRequest struct {
	Name              string      `json:"name"`
	Type              LibraryType `json:"type"`
	ImportPaths       []string    `json:"importPaths"`
	ExclusionPatterns []string    `json:"exclusionPatterns"`
	IsWatched         bool        `json:"isWatched"`
	IsVisible         bool        `json:"isVisible"`
}

type UpdateLibraryRequest struct {
	Name              *string  `json:"name,omitempty"`
	ImportPaths       []string `json:"importPaths,omitempty"`
	ExclusionPatterns []string `json:"exclusionPatterns,omitempty"`
	IsWatched         *bool    `json:"isWatched,omitempty"`
}

type LibraryStatistics struct {
	AssetCount int64   `json:"assetCount"`
	Photos     int64   `json:"photos"`
	Videos     int64   `json:"videos"`
	TotalSize  int64   `json:"totalSize"`
	Usage      float32 `json:"usage"`
}

type ValidateLibraryRequest struct {
	ImportPaths []string `json:"importPaths"`
}

type ValidateLibraryResponse struct {
	Results map[string]PathValidation `json:"results"`
}

type PathValidation struct {
	Path       string `json:"path"`
	IsValid    bool   `json:"isValid"`
	IsReadable bool   `json:"isReadable"`
	Message    string `json:"message"`
}

// LibraryScanner handles scanning library directories for assets
type LibraryScanner struct {
	library        *Library
	db             *sqlc.Queries
	storageService *storage.Service
	stopCh         chan struct{}
}

// NewLibraryScanner creates a new library scanner
func NewLibraryScanner(library *Library, db *sqlc.Queries, storageService *storage.Service) *LibraryScanner {
	return &LibraryScanner{
		library:        library,
		db:             db,
		storageService: storageService,
		stopCh:         make(chan struct{}),
	}
}

// Scan scans the library for assets
func (ls *LibraryScanner) Scan(ctx context.Context, forceRefresh bool) error {
	logrus.Infof("Starting scan of library %s", ls.library.Name)

	for _, importPath := range ls.library.ImportPaths {
		if err := ls.scanPath(ctx, importPath, forceRefresh); err != nil {
			logrus.WithError(err).Errorf("Failed to scan path %s", importPath)
			continue
		}
	}

	logrus.Infof("Completed scan of library %s", ls.library.Name)
	return nil
}

// scanPath scans a specific path for assets
func (ls *LibraryScanner) scanPath(ctx context.Context, path string, forceRefresh bool) error {
	return filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		// Check if stopped
		select {
		case <-ls.stopCh:
			return fmt.Errorf("scan stopped")
		default:
		}

		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			// Check exclusion patterns
			for _, pattern := range ls.library.ExclusionPatterns {
				matched, _ := filepath.Match(pattern, d.Name())
				if matched {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check if file should be excluded
		for _, pattern := range ls.library.ExclusionPatterns {
			matched, _ := filepath.Match(pattern, path)
			if matched {
				return nil
			}
		}

		// Check if file is a supported media type
		if !ls.isSupportedMediaType(path) {
			return nil
		}

		// Check if asset already exists (by path)
		result, err := ls.db.CheckAssetExistsByPath(ctx, path)
		if err != nil {
			logrus.WithError(err).Errorf("Failed to check if asset exists: %s", path)
			return nil // Continue with other files even if this check fails
		}

		if result && !forceRefresh {
			return nil
		}

		// Import the asset
		if err := ls.importAsset(ctx, path); err != nil {
			logrus.WithError(err).Errorf("Failed to import asset: %s", path)
			// Continue with other files even if this import fails
		}

		return nil
	})
}

// importAsset creates an asset record in the database for the given file
func (ls *LibraryScanner) importAsset(ctx context.Context, filePath string) error {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Calculate file checksum
	checksum, err := ls.calculateChecksum(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Determine asset type based on file extension
	assetType := ls.getAssetType(filePath)

	// Get file timestamps
	modTime := fileInfo.ModTime()
	modTimePg := TimeToPgtype(modTime)

	// Create asset record in database
	_, err = ls.db.CreateLibraryAsset(ctx, sqlc.CreateLibraryAssetParams{
		DeviceAssetId:    filepath.Base(filePath),
		OwnerId:          UUIDToPgtype(ls.library.OwnerID),
		LibraryId:        UUIDToPgtype(ls.library.ID),
		DeviceId:         "library-scanner",
		Type:             assetType,
		OriginalPath:     filePath,
		FileCreatedAt:    modTimePg,
		FileModifiedAt:   modTimePg,
		LocalDateTime:    modTimePg,
		OriginalFileName: filepath.Base(filePath),
		Checksum:         checksum,
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	if err != nil {
		return fmt.Errorf("failed to create asset record: %w", err)
	}

	logrus.Debugf("Imported asset: %s", filePath)
	return nil
}

// calculateChecksum calculates the SHA-256 checksum of a file
func (ls *LibraryScanner) calculateChecksum(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}

// getAssetType determines the asset type based on file extension
func (ls *LibraryScanner) getAssetType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	// Video extensions
	videoExts := map[string]bool{
		".mp4": true, ".avi": true, ".mov": true, ".mkv": true, ".webm": true,
		".m4v": true, ".3gp": true, ".wmv": true, ".flv": true, ".mts": true,
		".m2ts": true, ".mpg": true, ".mpeg": true,
	}

	if videoExts[ext] {
		return "VIDEO"
	}
	return "IMAGE"
}

// isSupportedMediaType checks if a file is a supported media type
func (ls *LibraryScanner) isSupportedMediaType(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	// Supported image formats
	imageExts := []string{
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif",
		".webp", ".heic", ".heif", ".raw", ".cr2", ".nef", ".arw",
		".dng", ".orf", ".rw2", ".raf", ".srw",
	}

	// Supported video formats
	videoExts := []string{
		".mp4", ".avi", ".mov", ".mkv", ".webm", ".m4v",
		".3gp", ".wmv", ".flv", ".mts", ".m2ts", ".mpg", ".mpeg",
	}

	for _, supportedExt := range append(imageExts, videoExts...) {
		if ext == supportedExt {
			return true
		}
	}

	return false
}

// Stop stops the scanner
func (ls *LibraryScanner) Stop() {
	close(ls.stopCh)
}
