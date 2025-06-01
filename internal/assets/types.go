package assets

import (
	"time"

	"github.com/google/uuid"
)

// AssetType represents the type of asset
type AssetType string

const (
	AssetTypeImage AssetType = "IMAGE"
	AssetTypeVideo AssetType = "VIDEO"
	AssetTypeAudio AssetType = "AUDIO"
	AssetTypeOther AssetType = "OTHER"
)

// ThumbnailType represents different thumbnail sizes
type ThumbnailType string

const (
	ThumbnailTypePreview ThumbnailType = "preview" // 1440px
	ThumbnailTypeWebp    ThumbnailType = "webp"    // 250px
	ThumbnailTypeThumb   ThumbnailType = "thumb"   // 160px
)

// AssetStatus represents the processing status of an asset
type AssetStatus string

const (
	AssetStatusUploading  AssetStatus = "uploading"
	AssetStatusProcessing AssetStatus = "processing"
	AssetStatusActive     AssetStatus = "active"
	AssetStatusTrash      AssetStatus = "trash"
	AssetStatusDeleted    AssetStatus = "deleted"
)

// UploadRequest represents a request to upload an asset
type UploadRequest struct {
	UserID      uuid.UUID `json:"userId" binding:"required"`
	Filename    string    `json:"filename" binding:"required"`
	ContentType string    `json:"contentType" binding:"required"`
	Size        int64     `json:"size" binding:"required"`
	Checksum    string    `json:"checksum,omitempty"`
}

// UploadResponse represents the response for an upload request
type UploadResponse struct {
	AssetID      uuid.UUID         `json:"assetId"`
	UploadURL    string            `json:"uploadUrl,omitempty"`    // Pre-signed URL for S3
	UploadFields map[string]string `json:"uploadFields,omitempty"` // Additional fields for S3 POST
	DirectUpload bool              `json:"directUpload"`           // Whether to upload directly to storage
}

// AssetMetadata represents extracted metadata from an asset
type AssetMetadata struct {
	// Basic file information
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Checksum    string `json:"checksum"`

	// Timestamps
	CreatedAt  time.Time  `json:"createdAt"`
	ModifiedAt time.Time  `json:"modifiedAt"`
	DateTaken  *time.Time `json:"dateTaken,omitempty"`

	// Image/Video specific
	Width    *int32   `json:"width,omitempty"`
	Height   *int32   `json:"height,omitempty"`
	Duration *float64 `json:"duration,omitempty"` // Video duration in seconds

	// Camera/Device information
	Make      *string `json:"make,omitempty"`
	Model     *string `json:"model,omitempty"`
	LensModel *string `json:"lensModel,omitempty"`

	// Camera settings
	FNumber      *float64 `json:"fNumber,omitempty"`
	FocalLength  *float64 `json:"focalLength,omitempty"`
	ISO          *int32   `json:"iso,omitempty"`
	ExposureTime *string  `json:"exposureTime,omitempty"`

	// Location
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	City      *string  `json:"city,omitempty"`
	State     *string  `json:"state,omitempty"`
	Country   *string  `json:"country,omitempty"`

	// Additional metadata
	Description *string  `json:"description,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

// AssetInfo represents complete asset information
type AssetInfo struct {
	ID           uuid.UUID       `json:"id"`
	UserID       uuid.UUID       `json:"userId"`
	Type         AssetType       `json:"type"`
	Status       AssetStatus     `json:"status"`
	OriginalPath string          `json:"originalPath"`
	Metadata     AssetMetadata   `json:"metadata"`
	Thumbnails   []ThumbnailInfo `json:"thumbnails,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	DeletedAt    *time.Time      `json:"deletedAt,omitempty"`
}

// ThumbnailInfo represents thumbnail information
type ThumbnailInfo struct {
	Type   ThumbnailType `json:"type"`
	Path   string        `json:"path"`
	Width  int32         `json:"width"`
	Height int32         `json:"height"`
	Size   int64         `json:"size"`
}

// AssetThumbnail represents a database thumbnail record (placeholder for now)
type AssetThumbnail struct {
	AssetID uuid.UUID `json:"assetId"`
	Type    string    `json:"type"`
	Path    string    `json:"path"`
	Width   int32     `json:"width"`
	Height  int32     `json:"height"`
	Size    int64     `json:"size"`
}

// SearchRequest represents asset search parameters
type SearchRequest struct {
	UserID    uuid.UUID  `json:"userId"`
	Query     string     `json:"query,omitempty"`
	Type      *AssetType `json:"type,omitempty"`
	StartDate *time.Time `json:"startDate,omitempty"`
	EndDate   *time.Time `json:"endDate,omitempty"`
	City      *string    `json:"city,omitempty"`
	State     *string    `json:"state,omitempty"`
	Country   *string    `json:"country,omitempty"`
	Make      *string    `json:"make,omitempty"`
	Model     *string    `json:"model,omitempty"`
	Limit     int        `json:"limit,omitempty"`
	Offset    int        `json:"offset,omitempty"`
	SortBy    string     `json:"sortBy,omitempty"`    // "created_at", "date_taken", "filename"
	SortOrder string     `json:"sortOrder,omitempty"` // "asc", "desc"
}

// SearchResponse represents search results
type SearchResponse struct {
	Assets []AssetInfo `json:"assets"`
	Total  int64       `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

// DownloadRequest represents a request to download an asset
type DownloadRequest struct {
	AssetID       uuid.UUID      `json:"assetId" binding:"required"`
	ThumbnailType *ThumbnailType `json:"thumbnailType,omitempty"`
	UserID        uuid.UUID      `json:"userId"`
}

// DownloadResponse represents the response for a download request
type DownloadResponse struct {
	URL       string            `json:"url"`                 // Pre-signed URL for S3 or direct URL
	Headers   map[string]string `json:"headers,omitempty"`   // Additional headers if needed
	ExpiresAt *time.Time        `json:"expiresAt,omitempty"` // When the URL expires
}

// ProcessingJob represents a background job for asset processing
type ProcessingJob struct {
	ID        uuid.UUID `json:"id"`
	AssetID   uuid.UUID `json:"assetId"`
	Type      string    `json:"type"` // "metadata", "thumbnail", "transcode"
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	Error     *string   `json:"error,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
