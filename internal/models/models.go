package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Base model with common fields
type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt,omitempty"`
}

// User represents a user in the system
type User struct {
	BaseModel
	Email              string     `gorm:"uniqueIndex;not null" json:"email"`
	Name               string     `gorm:"not null" json:"name"`
	PasswordHash       string     `gorm:"not null" json:"-"`
	IsAdmin            bool       `gorm:"default:false" json:"isAdmin"`
	AvatarColor        string     `json:"avatarColor"`
	ProfileImagePath   string     `json:"profileImagePath"`
	ProfileChangedAt   time.Time  `json:"profileChangedAt"`
	ShouldChangePassword bool     `gorm:"default:false" json:"shouldChangePassword"`
	QuotaSizeInBytes   int64      `json:"quotaSizeInBytes"`
	StorageLabel       string     `json:"storageLabel"`
	OAuthID            string     `json:"oauthId"`
	License            *UserLicense `gorm:"embedded" json:"license,omitempty"`
	Status             string     `gorm:"default:'active'" json:"status"`
}

type UserLicense struct {
	ActivatedAt   time.Time `json:"activatedAt"`
	ActivationKey string    `json:"activationKey"`
	LicenseKey    string    `json:"licenseKey"`
}

// Album represents a photo album
type Album struct {
	BaseModel
	Name                    string    `gorm:"not null" json:"albumName"`
	Description             string    `json:"description"`
	OwnerID                 uuid.UUID `gorm:"type:uuid;not null" json:"ownerId"`
	Owner                   User      `gorm:"foreignKey:OwnerID" json:"owner"`
	AlbumThumbnailAssetID   *uuid.UUID `gorm:"type:uuid" json:"albumThumbnailAssetId,omitempty"`
	IsActivityEnabled       bool      `gorm:"default:true" json:"isActivityEnabled"`
	Assets                  []Asset   `gorm:"many2many:album_assets" json:"assets,omitempty"`
	AssetCount              int       `json:"assetCount"`
	StartDate               *time.Time `json:"startDate,omitempty"`
	EndDate                 *time.Time `json:"endDate,omitempty"`
}

// Asset represents a photo or video asset
type Asset struct {
	BaseModel
	OriginalPath        string     `gorm:"not null" json:"originalPath"`
	OriginalFileName    string     `gorm:"not null" json:"originalFileName"`
	FileCreatedAt       time.Time  `json:"fileCreatedAt"`
	FileModifiedAt      time.Time  `json:"fileModifiedAt"`
	LocalDateTime       time.Time  `json:"localDateTime"`
	OwnerID             uuid.UUID  `gorm:"type:uuid;not null" json:"ownerId"`
	Owner               User       `gorm:"foreignKey:OwnerID" json:"owner"`
	DeviceAssetID       string     `json:"deviceAssetId"`
	DeviceID            string     `json:"deviceId"`
	Type                string     `gorm:"not null" json:"type"` // PHOTO, VIDEO
	IsFavorite          bool       `gorm:"default:false" json:"isFavorite"`
	IsArchived          bool       `gorm:"default:false" json:"isArchived"`
	IsTrashed           bool       `gorm:"default:false" json:"isTrashed"`
	TrashedAt           *time.Time `json:"trashedAt,omitempty"`
	Duration            string     `json:"duration,omitempty"`
	IsVisible           bool       `gorm:"default:true" json:"isVisible"`
	LivePhotoVideoID    *uuid.UUID `gorm:"type:uuid" json:"livePhotoVideoId,omitempty"`
	Checksum            string     `gorm:"index" json:"checksum"`
	StackID             *uuid.UUID `gorm:"type:uuid" json:"stackId,omitempty"`
	Stack               *Stack     `gorm:"foreignKey:StackID" json:"stack,omitempty"`
}

// Library represents an external library
type Library struct {
	BaseModel
	Name              string   `gorm:"not null" json:"name"`
	OwnerID           uuid.UUID `gorm:"type:uuid;not null" json:"ownerId"`
	Owner             User     `gorm:"foreignKey:OwnerID" json:"owner"`
	ImportPaths       []string `gorm:"type:text[]" json:"importPaths"`
	ExclusionPatterns []string `gorm:"type:text[]" json:"exclusionPatterns"`
	RefreshedAt       *time.Time `json:"refreshedAt,omitempty"`
	AssetCount        int      `json:"assetCount"`
}

// APIKey represents an API key for authentication
type APIKey struct {
	BaseModel
	Name      string    `gorm:"not null" json:"name"`
	Key       string    `gorm:"uniqueIndex;not null" json:"key"`
	UserID    uuid.UUID `gorm:"type:uuid;not null" json:"userId"`
	User      User      `gorm:"foreignKey:UserID" json:"user"`
	CreatedBy uuid.UUID `gorm:"type:uuid" json:"createdBy"`
}

// Session represents a user session
type Session struct {
	BaseModel
	UserID    uuid.UUID `gorm:"type:uuid;not null" json:"userId"`
	User      User      `gorm:"foreignKey:UserID" json:"user"`
	DeviceOS  string    `json:"deviceOS"`
	DeviceType string   `json:"deviceType"`
	Current   bool      `gorm:"default:false" json:"current"`
}

// Tag represents a tag that can be applied to assets
type Tag struct {
	BaseModel
	Name     string     `gorm:"not null" json:"name"`
	Color    string     `json:"color"`
	ParentID *uuid.UUID `gorm:"type:uuid" json:"parentId,omitempty"`
	Parent   *Tag       `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Value    string     `json:"value"`
	Assets   []Asset    `gorm:"many2many:tag_assets" json:"assets,omitempty"`
}

// Activity represents user activity/comments
type Activity struct {
	BaseModel
	Type     string     `gorm:"not null" json:"type"`
	UserID   uuid.UUID  `gorm:"type:uuid;not null" json:"userId"`
	User     User       `gorm:"foreignKey:UserID" json:"user"`
	AssetID  *uuid.UUID `gorm:"type:uuid" json:"assetId,omitempty"`
	Asset    *Asset     `gorm:"foreignKey:AssetID" json:"asset,omitempty"`
	AlbumID  *uuid.UUID `gorm:"type:uuid" json:"albumId,omitempty"`
	Album    *Album     `gorm:"foreignKey:AlbumID" json:"album,omitempty"`
	Comment  string     `json:"comment,omitempty"`
}

// Notification represents system notifications
type Notification struct {
	BaseModel
	UserID  uuid.UUID `gorm:"type:uuid;not null" json:"userId"`
	User    User      `gorm:"foreignKey:UserID" json:"user"`
	Type    string    `gorm:"not null" json:"type"`
	Level   string    `gorm:"not null" json:"level"`
	Title   string    `gorm:"not null" json:"title"`
	Message string    `json:"message"`
	IsRead  bool      `gorm:"default:false" json:"isRead"`
}

// Partner represents sharing partnerships between users
type Partner struct {
	BaseModel
	SharedByID   uuid.UUID `gorm:"type:uuid;not null" json:"sharedById"`
	SharedBy     User      `gorm:"foreignKey:SharedByID" json:"sharedBy"`
	SharedWithID uuid.UUID `gorm:"type:uuid;not null" json:"sharedWithId"`
	SharedWith   User      `gorm:"foreignKey:SharedWithID" json:"sharedWith"`
	InTimeline   bool      `gorm:"default:false" json:"inTimeline"`
}

// Person represents a detected person in photos
type Person struct {
	BaseModel
	Name               string  `json:"name"`
	BirthDate          *time.Time `json:"birthDate,omitempty"`
	ThumbnailPath      string  `json:"thumbnailPath"`
	FaceAssetID        *uuid.UUID `gorm:"type:uuid" json:"faceAssetId,omitempty"`
	FaceAsset          *Asset  `gorm:"foreignKey:FaceAssetID" json:"faceAsset,omitempty"`
	IsHidden           bool    `gorm:"default:false" json:"isHidden"`
}

// SharedLink represents a shared album or asset link
type SharedLink struct {
	BaseModel
	Type            string     `gorm:"not null" json:"type"` // ALBUM, INDIVIDUAL
	Key             string     `gorm:"uniqueIndex;not null" json:"key"`
	UserID          uuid.UUID  `gorm:"type:uuid;not null" json:"userId"`
	User            User       `gorm:"foreignKey:UserID" json:"user"`
	AlbumID         *uuid.UUID `gorm:"type:uuid" json:"albumId,omitempty"`
	Album           *Album     `gorm:"foreignKey:AlbumID" json:"album,omitempty"`
	Description     string     `json:"description"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
	AllowUpload     bool       `gorm:"default:false" json:"allowUpload"`
	AllowDownload   bool       `gorm:"default:true" json:"allowDownload"`
	ShowMetadata    bool       `gorm:"default:true" json:"showMetadata"`
	Password        string     `json:"-"`
	Assets          []Asset    `gorm:"many2many:shared_link_assets" json:"assets,omitempty"`
}

// Stack represents a stack of related assets
type Stack struct {
	BaseModel
	PrimaryAssetID uuid.UUID `gorm:"type:uuid;not null" json:"primaryAssetId"`
	PrimaryAsset   Asset     `gorm:"foreignKey:PrimaryAssetID" json:"primaryAsset"`
	Assets         []Asset   `gorm:"foreignKey:StackID" json:"assets"`
	AssetCount     int       `json:"assetCount"`
}
