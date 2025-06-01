package albums

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/telemetry"
)

var tracer = telemetry.GetTracer("albums")

// Service handles album-related operations
type Service struct {
	db *sqlc.Queries
}

// NewService creates a new album service
func NewService(db *sqlc.Queries) *Service {
	return &Service{
		db: db,
	}
}

// CreateAlbum creates a new album
func (s *Service) CreateAlbum(ctx context.Context, req *CreateAlbumRequest) (*AlbumInfo, error) {
	ctx, span := tracer.Start(ctx, "albums.create_album",
		trace.WithAttributes(
			attribute.String("album_name", req.Name),
			attribute.String("owner_id", req.OwnerID.String()),
		),
	)
	defer span.End()

	// Convert owner ID to pgtype.UUID
	ownerUUID, err := stringToUUID(req.OwnerID.String())
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("invalid owner ID: %w", err)
	}

	// Create album in database
	album, err := s.db.CreateAlbum(ctx, sqlc.CreateAlbumParams{
		OwnerId:     ownerUUID,
		AlbumName:   req.Name,
		Description: req.Description,
	})
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to create album: %w", err)
	}

	// Convert to response format
	albumInfo := s.convertToAlbumInfo(album, nil, nil)
	
	span.SetAttributes(
		attribute.String("album_id", uuidToString(album.ID)),
	)

	return albumInfo, nil
}

// GetAlbum retrieves an album by ID
func (s *Service) GetAlbum(ctx context.Context, albumID uuid.UUID, userID uuid.UUID) (*AlbumInfo, error) {
	ctx, span := tracer.Start(ctx, "albums.get_album",
		trace.WithAttributes(
			attribute.String("album_id", albumID.String()),
			attribute.String("user_id", userID.String()),
		),
	)
	defer span.End()

	// Convert album ID to pgtype.UUID
	albumUUID, err := stringToUUID(albumID.String())
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("invalid album ID: %w", err)
	}

	// Get album
	album, err := s.db.GetAlbum(ctx, albumUUID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get album: %w", err)
	}

	// Check if user has access to this album
	if !s.userHasAlbumAccess(ctx, userID, album) {
		return nil, fmt.Errorf("access denied")
	}

	// Get album assets
	assets, err := s.db.GetAlbumAssets(ctx, albumUUID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get album assets: %w", err)
	}

	// Get shared users
	sharedUsers, err := s.db.GetAlbumSharedUsers(ctx, albumUUID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get shared users: %w", err)
	}

	// Convert to response format
	albumInfo := s.convertToAlbumInfo(album, assets, sharedUsers)

	return albumInfo, nil
}

// GetUserAlbums retrieves all albums for a user
func (s *Service) GetUserAlbums(ctx context.Context, userID uuid.UUID) ([]*AlbumInfo, error) {
	ctx, span := tracer.Start(ctx, "albums.get_user_albums",
		trace.WithAttributes(
			attribute.String("user_id", userID.String()),
		),
	)
	defer span.End()

	// Convert user ID to pgtype.UUID
	userUUID, err := stringToUUID(userID.String())
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Get albums owned by user
	albums, err := s.db.GetAlbumsByOwner(ctx, userUUID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get user albums: %w", err)
	}

	// Convert to response format
	albumInfos := make([]*AlbumInfo, len(albums))
	for i, album := range albums {
		// For list view, we don't need assets and shared users
		albumInfos[i] = s.convertToAlbumInfo(album, nil, nil)
	}

	span.SetAttributes(
		attribute.Int("album_count", len(albumInfos)),
	)

	return albumInfos, nil
}

// UpdateAlbum updates an album
func (s *Service) UpdateAlbum(ctx context.Context, albumID uuid.UUID, userID uuid.UUID, req *UpdateAlbumRequest) (*AlbumInfo, error) {
	ctx, span := tracer.Start(ctx, "albums.update_album",
		trace.WithAttributes(
			attribute.String("album_id", albumID.String()),
			attribute.String("user_id", userID.String()),
		),
	)
	defer span.End()

	// Convert album ID to pgtype.UUID
	albumUUID, err := stringToUUID(albumID.String())
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("invalid album ID: %w", err)
	}

	// Get existing album to check ownership
	album, err := s.db.GetAlbum(ctx, albumUUID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get album: %w", err)
	}

	// Check if user owns this album
	if uuidToString(album.OwnerId) != userID.String() {
		return nil, fmt.Errorf("access denied: user does not own this album")
	}

	// Prepare update parameters
	updateParams := sqlc.UpdateAlbumParams{
		ID:          albumUUID,
		AlbumName:   pgtype.Text{String: req.Name, Valid: true},
		Description: pgtype.Text{String: req.Description, Valid: true},
	}

	// Set thumbnail if provided
	if req.ThumbnailAssetID != nil {
		thumbnailUUID, err := stringToUUID(req.ThumbnailAssetID.String())
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("invalid thumbnail asset ID: %w", err)
		}
		updateParams.AlbumThumbnailAssetID = thumbnailUUID
	}

	// Update album
	updatedAlbum, err := s.db.UpdateAlbum(ctx, updateParams)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to update album: %w", err)
	}

	// Convert to response format
	albumInfo := s.convertToAlbumInfo(updatedAlbum, nil, nil)

	return albumInfo, nil
}

// DeleteAlbum deletes an album
func (s *Service) DeleteAlbum(ctx context.Context, albumID uuid.UUID, userID uuid.UUID) error {
	ctx, span := tracer.Start(ctx, "albums.delete_album",
		trace.WithAttributes(
			attribute.String("album_id", albumID.String()),
			attribute.String("user_id", userID.String()),
		),
	)
	defer span.End()

	// Convert album ID to pgtype.UUID
	albumUUID, err := stringToUUID(albumID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid album ID: %w", err)
	}

	// Get existing album to check ownership
	album, err := s.db.GetAlbum(ctx, albumUUID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get album: %w", err)
	}

	// Check if user owns this album
	if uuidToString(album.OwnerId) != userID.String() {
		return fmt.Errorf("access denied: user does not own this album")
	}

	// Delete album
	err = s.db.DeleteAlbum(ctx, albumUUID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to delete album: %w", err)
	}

	return nil
}

// AddAssetToAlbum adds an asset to an album
func (s *Service) AddAssetToAlbum(ctx context.Context, albumID uuid.UUID, assetID uuid.UUID, userID uuid.UUID) error {
	ctx, span := tracer.Start(ctx, "albums.add_asset_to_album",
		trace.WithAttributes(
			attribute.String("album_id", albumID.String()),
			attribute.String("asset_id", assetID.String()),
			attribute.String("user_id", userID.String()),
		),
	)
	defer span.End()

	// Convert IDs to pgtype.UUID
	albumUUID, err := stringToUUID(albumID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid album ID: %w", err)
	}

	assetUUID, err := stringToUUID(assetID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid asset ID: %w", err)
	}

	// Get album to check access
	album, err := s.db.GetAlbum(ctx, albumUUID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get album: %w", err)
	}

	// Check if user has access to this album
	if !s.userHasAlbumAccess(ctx, userID, album) {
		return fmt.Errorf("access denied")
	}

	// Add asset to album
	err = s.db.AddAssetToAlbum(ctx, sqlc.AddAssetToAlbumParams{
		AlbumsId: albumUUID,
		AssetsId: assetUUID,
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to add asset to album: %w", err)
	}

	return nil
}

// RemoveAssetFromAlbum removes an asset from an album
func (s *Service) RemoveAssetFromAlbum(ctx context.Context, albumID uuid.UUID, assetID uuid.UUID, userID uuid.UUID) error {
	ctx, span := tracer.Start(ctx, "albums.remove_asset_from_album",
		trace.WithAttributes(
			attribute.String("album_id", albumID.String()),
			attribute.String("asset_id", assetID.String()),
			attribute.String("user_id", userID.String()),
		),
	)
	defer span.End()

	// Convert IDs to pgtype.UUID
	albumUUID, err := stringToUUID(albumID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid album ID: %w", err)
	}

	assetUUID, err := stringToUUID(assetID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid asset ID: %w", err)
	}

	// Get album to check access
	album, err := s.db.GetAlbum(ctx, albumUUID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get album: %w", err)
	}

	// Check if user has access to this album
	if !s.userHasAlbumAccess(ctx, userID, album) {
		return fmt.Errorf("access denied")
	}

	// Remove asset from album
	err = s.db.RemoveAssetFromAlbum(ctx, sqlc.RemoveAssetFromAlbumParams{
		AlbumsId: albumUUID,
		AssetsId: assetUUID,
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to remove asset from album: %w", err)
	}

	return nil
}

// ShareAlbum shares an album with a user
func (s *Service) ShareAlbum(ctx context.Context, albumID uuid.UUID, targetUserID uuid.UUID, role string, ownerID uuid.UUID) error {
	ctx, span := tracer.Start(ctx, "albums.share_album",
		trace.WithAttributes(
			attribute.String("album_id", albumID.String()),
			attribute.String("target_user_id", targetUserID.String()),
			attribute.String("role", role),
			attribute.String("owner_id", ownerID.String()),
		),
	)
	defer span.End()

	// Convert IDs to pgtype.UUID
	albumUUID, err := stringToUUID(albumID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid album ID: %w", err)
	}

	targetUserUUID, err := stringToUUID(targetUserID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid target user ID: %w", err)
	}

	// Get album to check ownership
	album, err := s.db.GetAlbum(ctx, albumUUID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get album: %w", err)
	}

	// Check if user owns this album
	if uuidToString(album.OwnerId) != ownerID.String() {
		return fmt.Errorf("access denied: user does not own this album")
	}

	// Add user to album
	err = s.db.AddUserToAlbum(ctx, sqlc.AddUserToAlbumParams{
		AlbumsId: albumUUID,
		UsersId:  targetUserUUID,
		Role:     role,
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to share album: %w", err)
	}

	return nil
}

// UnshareAlbum removes a user from an album
func (s *Service) UnshareAlbum(ctx context.Context, albumID uuid.UUID, targetUserID uuid.UUID, ownerID uuid.UUID) error {
	ctx, span := tracer.Start(ctx, "albums.unshare_album",
		trace.WithAttributes(
			attribute.String("album_id", albumID.String()),
			attribute.String("target_user_id", targetUserID.String()),
			attribute.String("owner_id", ownerID.String()),
		),
	)
	defer span.End()

	// Convert IDs to pgtype.UUID
	albumUUID, err := stringToUUID(albumID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid album ID: %w", err)
	}

	targetUserUUID, err := stringToUUID(targetUserID.String())
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("invalid target user ID: %w", err)
	}

	// Get album to check ownership
	album, err := s.db.GetAlbum(ctx, albumUUID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get album: %w", err)
	}

	// Check if user owns this album
	if uuidToString(album.OwnerId) != ownerID.String() {
		return fmt.Errorf("access denied: user does not own this album")
	}

	// Remove user from album
	err = s.db.RemoveUserFromAlbum(ctx, sqlc.RemoveUserFromAlbumParams{
		AlbumsId: albumUUID,
		UsersId:  targetUserUUID,
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to unshare album: %w", err)
	}

	return nil
}

// Helper functions

// userHasAlbumAccess checks if a user has access to an album
func (s *Service) userHasAlbumAccess(ctx context.Context, userID uuid.UUID, album sqlc.Album) bool {
	// Owner always has access
	if uuidToString(album.OwnerId) == userID.String() {
		return true
	}

	// TODO: Check if user is in shared users list
	// For now, we'll implement a simple check
	albumUUID, err := stringToUUID(uuidToString(album.ID))
	if err != nil {
		return false
	}

	sharedUsers, err := s.db.GetAlbumSharedUsers(ctx, albumUUID)
	if err != nil {
		return false
	}

	for _, sharedUser := range sharedUsers {
		if uuidToString(sharedUser.ID) == userID.String() {
			return true
		}
	}

	return false
}

// convertToAlbumInfo converts a database album to AlbumInfo
func (s *Service) convertToAlbumInfo(album sqlc.Album, assets []sqlc.Asset, sharedUsers []sqlc.GetAlbumSharedUsersRow) *AlbumInfo {
	info := &AlbumInfo{
		ID:          uuid.MustParse(uuidToString(album.ID)),
		OwnerID:     uuid.MustParse(uuidToString(album.OwnerId)),
		Name:        album.AlbumName,
		Description: album.Description,
		CreatedAt:   timestamptzToTime(album.CreatedAt),
		UpdatedAt:   timestamptzToTime(album.UpdatedAt),
		AssetCount:  len(assets),
	}

	// Set thumbnail if available
	if album.AlbumThumbnailAssetId.Valid {
		thumbnailID := uuid.MustParse(uuidToString(album.AlbumThumbnailAssetId))
		info.ThumbnailAssetID = &thumbnailID
	}

	// Add assets if provided
	if assets != nil {
		info.Assets = make([]uuid.UUID, len(assets))
		for i, asset := range assets {
			info.Assets[i] = uuid.MustParse(uuidToString(asset.ID))
		}
	}

	// Add shared users if provided
	if sharedUsers != nil {
		info.SharedUsers = make([]SharedUser, len(sharedUsers))
		for i, sharedUser := range sharedUsers {
			info.SharedUsers[i] = SharedUser{
				UserID: uuid.MustParse(uuidToString(sharedUser.ID)),
				Role:   sharedUser.Role,
			}
		}
	}

	return info
}

// Helper functions for type conversion (same as in assets service)
func stringToUUID(s string) (pgtype.UUID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: u, Valid: true}, nil
}

func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return string(u.Bytes)
}

func timestamptzToTime(ts pgtype.Timestamptz) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time
}