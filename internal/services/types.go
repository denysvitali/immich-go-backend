package services

import (
	"time"

	"github.com/denysvitali/immich-go-backend/internal/models"
	"github.com/google/uuid"
)

type AssetResponse struct {
	ID               uuid.UUID       `json:"id"`
	DeviceAssetId    string          `json:"deviceAssetId"`
	OwnerID          uuid.UUID       `json:"ownerId"`
	DeviceID         string          `json:"deviceId"`
	Type             string          `json:"type"`
	OriginalPath     string          `json:"originalPath"`
	OriginalFileName string          `json:"originalFileName"`
	ResizePath       *string         `json:"resizePath"`
	WebpPath         *string         `json:"webpPath"`
	ThumbhashPath    *string         `json:"thumbhashPath"`
	EncodedVideoPath *string         `json:"encodedVideoPath"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	IsFavorite       bool            `json:"isFavorite"`
	IsArchived       bool            `json:"isArchived"`
	IsTrashed        bool            `json:"isTrashed"`
	Duration         *string         `json:"duration"`
	ExifInfo         *ExifInfo       `json:"exifInfo"`
	SmartInfo        *SmartInfo      `json:"smartInfo"`
	LivePhotoVideoId *uuid.UUID      `json:"livePhotoVideoId"`
	Tags             []string        `json:"tags"`
	People           []string        `json:"people"`
	Checksum         string          `json:"checksum"`
	StackParentId    *uuid.UUID      `json:"stackParentId"`
	Stack            []AssetResponse `json:"stack,omitempty"`
}

type ExifInfo struct {
	Make             *string    `json:"make"`
	Model            *string    `json:"model"`
	ExifImageWidth   *int       `json:"exifImageWidth"`
	ExifImageHeight  *int       `json:"exifImageHeight"`
	FileSizeInByte   *int       `json:"fileSizeInByte"`
	Orientation      *string    `json:"orientation"`
	DateTimeOriginal *time.Time `json:"dateTimeOriginal"`
	ModifyDate       *time.Time `json:"modifyDate"`
	TimeZone         *string    `json:"timeZone"`
	LensModel        *string    `json:"lensModel"`
	FNumber          *float64   `json:"fNumber"`
	FocalLength      *float64   `json:"focalLength"`
	ISO              *int       `json:"iso"`
	ExposureTime     *string    `json:"exposureTime"`
	Latitude         *float64   `json:"latitude"`
	Longitude        *float64   `json:"longitude"`
	City             *string    `json:"city"`
	State            *string    `json:"state"`
	Country          *string    `json:"country"`
	Description      *string    `json:"description"`
	ProjectionType   *string    `json:"projectionType"`
}

type SmartInfo struct {
	Objects []string `json:"objects"`
	Tags    []string `json:"tags"`
}

func toAssetResponse(asset models.Asset) AssetResponse {
	var duration *string
	if asset.Duration != "" {
		duration = &asset.Duration
	}

	return AssetResponse{
		ID:               asset.ID,
		DeviceAssetId:    asset.DeviceAssetID,
		OwnerID:          asset.OwnerID,
		DeviceID:         asset.DeviceID,
		Type:             asset.Type,
		OriginalPath:     asset.OriginalPath,
		OriginalFileName: asset.OriginalFileName,
		ResizePath:       nil, // TODO: implement resize path
		WebpPath:         nil, // TODO: implement webp path
		ThumbhashPath:    nil, // TODO: implement thumbhash path
		EncodedVideoPath: nil, // TODO: implement encoded video path
		CreatedAt:        asset.CreatedAt,
		UpdatedAt:        asset.UpdatedAt,
		IsFavorite:       asset.IsFavorite,
		IsArchived:       asset.IsArchived,
		IsTrashed:        asset.IsTrashed,
		Duration:         duration,
		Checksum:         asset.Checksum,
		StackParentId:    asset.StackID,
		// TODO: Add ExifInfo, SmartInfo, Tags, People conversion
	}
}