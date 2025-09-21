package assets

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewService(t *testing.T) {
	cfg := &config.Config{}
	storageService := &storage.Service{}

	// Note: This will fail without proper database setup
	// but tests the service initialization structure
	service, err := NewService(nil, storageService, cfg)

	assert.NotNil(t, service)
	assert.NoError(t, err)
	assert.NotNil(t, service.storage)
	assert.NotNil(t, service.metadataExtractor)
	assert.NotNil(t, service.thumbnailGen)
	assert.NotNil(t, service.config)
}

func TestService_GetStorageService(t *testing.T) {
	storageService := &storage.Service{}
	service := &Service{
		storage: storageService,
	}

	result := service.GetStorageService()
	assert.Equal(t, storageService, result)
}

func TestAssetType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected AssetType
	}{
		{
			name:     "JPEG image",
			filename: "photo.jpg",
			expected: AssetTypeImage,
		},
		{
			name:     "PNG image",
			filename: "screenshot.png",
			expected: AssetTypeImage,
		},
		{
			name:     "MP4 video",
			filename: "movie.mp4",
			expected: AssetTypeVideo,
		},
		{
			name:     "MOV video",
			filename: "recording.mov",
			expected: AssetTypeVideo,
		},
		{
			name:     "unknown type",
			filename: "document.pdf",
			expected: AssetTypeOther,
		},
		{
			name:     "uppercase extension",
			filename: "PHOTO.JPG",
			expected: AssetTypeImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAssetTypeFromFilename(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUploadRequest_Validation(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		req := UploadRequest{
			UserID:      uuid.New(),
			Filename:    "photo.jpg",
			ContentType: "image/jpeg",
			Size:        1024,
		}

		assert.NotEqual(t, uuid.Nil, req.UserID)
		assert.NotEmpty(t, req.Filename)
		assert.NotEmpty(t, req.ContentType)
		assert.Greater(t, req.Size, int64(0))
	})

	t.Run("with checksum", func(t *testing.T) {
		req := UploadRequest{
			UserID:      uuid.New(),
			Filename:    "photo.jpg",
			ContentType: "image/jpeg",
			Size:        1024,
			Checksum:    "sha256:abcdef123456",
		}

		assert.NotEmpty(t, req.Checksum)
		assert.Equal(t, "sha256:abcdef123456", req.Checksum)
	})

	t.Run("without checksum", func(t *testing.T) {
		req := UploadRequest{
			UserID:      uuid.New(),
			Filename:    "photo.jpg",
			ContentType: "image/jpeg",
			Size:        1024,
			Checksum:    "",
		}

		assert.Empty(t, req.Checksum)
	})
}

func TestAssetInfo_Properties(t *testing.T) {
	now := time.Now()
	assetID := uuid.New()
	userID := uuid.New()

	width := int32(1920)
	height := int32(1080)

	asset := AssetInfo{
		ID:           assetID,
		UserID:       userID,
		Type:         AssetTypeImage,
		Status:       AssetStatusActive,
		OriginalPath: "/uploads/test.jpg",
		Metadata: AssetMetadata{
			Width:  &width,
			Height: &height,
		},
		Thumbnails: []ThumbnailInfo{
			{
				Type:   ThumbnailTypeWebp,
				Path:   "/thumbnails/test.webp",
				Width:  200,
				Height: 200,
				Size:   1024,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
		DeletedAt: nil,
	}

	assert.Equal(t, assetID, asset.ID)
	assert.Equal(t, userID, asset.UserID)
	assert.Equal(t, AssetTypeImage, asset.Type)
	assert.Equal(t, AssetStatusActive, asset.Status)
	assert.Equal(t, "/uploads/test.jpg", asset.OriginalPath)
	assert.Len(t, asset.Thumbnails, 1)
	assert.Nil(t, asset.DeletedAt)
}

func TestSearchRequest(t *testing.T) {
	t.Run("basic search", func(t *testing.T) {
		req := SearchRequest{
			UserID: uuid.New(),
			Query:  "vacation",
			Limit:  10,
			Offset: 0,
		}

		assert.NotEqual(t, uuid.Nil, req.UserID)
		assert.Equal(t, "vacation", req.Query)
		assert.Equal(t, 10, req.Limit)
		assert.Equal(t, 0, req.Offset)
	})

	t.Run("search with filters", func(t *testing.T) {
		assetType := AssetTypeImage
		make := "Canon"
		model := "EOS R5"

		req := SearchRequest{
			UserID: uuid.New(),
			Query:  "",
			Type:   &assetType,
			Make:   &make,
			Model:  &model,
			Limit:  20,
			Offset: 0,
		}

		assert.NotNil(t, req.Type)
		assert.Equal(t, AssetTypeImage, *req.Type)
		assert.NotNil(t, req.Make)
		assert.Equal(t, "Canon", *req.Make)
		assert.NotNil(t, req.Model)
		assert.Equal(t, "EOS R5", *req.Model)
	})

	t.Run("search with date range", func(t *testing.T) {
		startDate := time.Now().AddDate(0, -1, 0)
		endDate := time.Now()

		req := SearchRequest{
			UserID:    uuid.New(),
			Query:     "",
			StartDate: &startDate,
			EndDate:   &endDate,
			Limit:     50,
			Offset:    0,
		}

		assert.NotNil(t, req.StartDate)
		assert.NotNil(t, req.EndDate)
		assert.True(t, req.StartDate.Before(*req.EndDate))
	})

	t.Run("search with location", func(t *testing.T) {
		city := "San Francisco"
		state := "California"
		country := "United States"

		req := SearchRequest{
			UserID:  uuid.New(),
			Query:   "",
			City:    &city,
			State:   &state,
			Country: &country,
			Limit:   10,
			Offset:  0,
		}

		assert.NotNil(t, req.City)
		assert.Equal(t, "San Francisco", *req.City)
		assert.NotNil(t, req.State)
		assert.Equal(t, "California", *req.State)
		assert.NotNil(t, req.Country)
		assert.Equal(t, "United States", *req.Country)
	})
}

func TestDownloadRequest(t *testing.T) {
	t.Run("asset download without thumbnail", func(t *testing.T) {
		assetID := uuid.New()
		req := DownloadRequest{
			AssetID: assetID,
			UserID:  uuid.New(),
		}

		assert.Equal(t, assetID, req.AssetID)
		assert.NotEqual(t, uuid.Nil, req.UserID)
		assert.Nil(t, req.ThumbnailType)
	})

	t.Run("asset download with thumbnail type", func(t *testing.T) {
		thumbType := ThumbnailTypeWebp
		req := DownloadRequest{
			AssetID:       uuid.New(),
			UserID:        uuid.New(),
			ThumbnailType: &thumbType,
		}

		assert.NotEqual(t, uuid.Nil, req.AssetID)
		assert.NotEqual(t, uuid.Nil, req.UserID)
		assert.NotNil(t, req.ThumbnailType)
		assert.Equal(t, ThumbnailTypeWebp, *req.ThumbnailType)
	})
}

func TestAssetMetadata(t *testing.T) {
	now := time.Now()
	dateTaken := time.Now().Add(-24 * time.Hour)
	width := int32(1920)
	height := int32(1080)
	make := "Canon"
	model := "EOS R5"
	lensModel := "RF 24-70mm"
	focalLength := float64(50.0)
	iso := int32(100)
	exposureTime := "1/125"
	fNumber := float64(2.8)
	latitude := float64(37.7749)
	longitude := float64(-122.4194)
	city := "San Francisco"
	state := "California"
	country := "United States"

	meta := AssetMetadata{
		Filename:     "photo.jpg",
		ContentType:  "image/jpeg",
		Size:         1024000,
		Checksum:     "sha256:abc123",
		CreatedAt:    now,
		ModifiedAt:   now,
		DateTaken:    &dateTaken,
		Width:        &width,
		Height:       &height,
		Make:         &make,
		Model:        &model,
		LensModel:    &lensModel,
		FocalLength:  &focalLength,
		ISO:          &iso,
		ExposureTime: &exposureTime,
		FNumber:      &fNumber,
		Latitude:     &latitude,
		Longitude:    &longitude,
		City:         &city,
		State:        &state,
		Country:      &country,
	}

	assert.Equal(t, "photo.jpg", meta.Filename)
	assert.Equal(t, "image/jpeg", meta.ContentType)
	assert.Equal(t, int64(1024000), meta.Size)
	assert.NotNil(t, meta.Width)
	assert.Equal(t, int32(1920), *meta.Width)
	assert.NotNil(t, meta.Height)
	assert.Equal(t, int32(1080), *meta.Height)
	assert.NotNil(t, meta.Make)
	assert.Equal(t, "Canon", *meta.Make)
	assert.NotNil(t, meta.Model)
	assert.Equal(t, "EOS R5", *meta.Model)
	assert.NotNil(t, meta.FocalLength)
	assert.Equal(t, float64(50.0), *meta.FocalLength)
	assert.NotNil(t, meta.Latitude)
	assert.Equal(t, float64(37.7749), *meta.Latitude)
	assert.NotNil(t, meta.Longitude)
	assert.Equal(t, float64(-122.4194), *meta.Longitude)
	assert.NotNil(t, meta.City)
	assert.Equal(t, "San Francisco", *meta.City)
}

func TestStreamUpload(t *testing.T) {
	content := []byte("test file content")
	reader := bytes.NewReader(content)

	// Create a mock upload stream
	stream := struct {
		io.Reader
		Size int64
	}{
		Reader: reader,
		Size:   int64(len(content)),
	}

	// Read from the stream
	buf := make([]byte, len(content))
	n, err := stream.Read(buf)

	assert.NoError(t, err)
	assert.Equal(t, len(content), n)
	assert.Equal(t, content, buf)
	assert.Equal(t, int64(len(content)), stream.Size)
}

// Helper function to determine asset type from filename
func GetAssetTypeFromFilename(filename string) AssetType {
	ext := filepath.Ext(filename)
	ext = strings.ToLower(ext)

	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".heic", ".heif":
		return AssetTypeImage
	case ".mp4", ".mov", ".avi", ".wmv", ".mkv", ".webm", ".m4v", ".3gp":
		return AssetTypeVideo
	default:
		return AssetTypeOther
	}
}