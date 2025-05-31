package albums

import (
	"time"

	"github.com/google/uuid"
)

// CreateAlbumRequest represents a request to create a new album
type CreateAlbumRequest struct {
	OwnerID     uuid.UUID `json:"ownerId" binding:"required"`
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description"`
}

// UpdateAlbumRequest represents a request to update an album
type UpdateAlbumRequest struct {
	Name              string     `json:"name" binding:"required"`
	Description       string     `json:"description"`
	ThumbnailAssetID  *uuid.UUID `json:"thumbnailAssetId,omitempty"`
}

// AlbumInfo represents detailed information about an album
type AlbumInfo struct {
	ID                uuid.UUID    `json:"id"`
	OwnerID           uuid.UUID    `json:"ownerId"`
	Name              string       `json:"name"`
	Description       string       `json:"description"`
	CreatedAt         time.Time    `json:"createdAt"`
	UpdatedAt         time.Time    `json:"updatedAt"`
	ThumbnailAssetID  *uuid.UUID   `json:"thumbnailAssetId,omitempty"`
	AssetCount        int          `json:"assetCount"`
	Assets            []uuid.UUID  `json:"assets,omitempty"`
	SharedUsers       []SharedUser `json:"sharedUsers,omitempty"`
	IsActivityEnabled bool         `json:"isActivityEnabled"`
}

// SharedUser represents a user that has access to an album
type SharedUser struct {
	UserID uuid.UUID `json:"userId"`
	Role   string    `json:"role"`
}

// AlbumRole represents the different roles a user can have in an album
type AlbumRole string

const (
	AlbumRoleViewer AlbumRole = "viewer"
	AlbumRoleEditor AlbumRole = "editor"
)

// AddAssetRequest represents a request to add an asset to an album
type AddAssetRequest struct {
	AssetID uuid.UUID `json:"assetId" binding:"required"`
}

// RemoveAssetRequest represents a request to remove an asset from an album
type RemoveAssetRequest struct {
	AssetID uuid.UUID `json:"assetId" binding:"required"`
}

// ShareAlbumRequest represents a request to share an album with a user
type ShareAlbumRequest struct {
	UserID uuid.UUID `json:"userId" binding:"required"`
	Role   string    `json:"role" binding:"required"`
}

// UnshareAlbumRequest represents a request to unshare an album from a user
type UnshareAlbumRequest struct {
	UserID uuid.UUID `json:"userId" binding:"required"`
}

// AlbumListResponse represents a list of albums
type AlbumListResponse struct {
	Albums []AlbumInfo `json:"albums"`
	Total  int64       `json:"total"`
}

// AlbumActivity represents activity in an album
type AlbumActivity struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"userId"`
	AlbumID   uuid.UUID  `json:"albumId"`
	AssetID   *uuid.UUID `json:"assetId,omitempty"`
	Comment   string     `json:"comment"`
	IsLiked   bool       `json:"isLiked"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// AlbumStatistics represents statistics about albums
type AlbumStatistics struct {
	TotalAlbums      int64 `json:"totalAlbums"`
	TotalSharedAlbums int64 `json:"totalSharedAlbums"`
	TotalAssets      int64 `json:"totalAssets"`
}