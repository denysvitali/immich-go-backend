package sqlc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// SharedLink represents a shared link in the database
type SharedLink struct {
	ID            uuid.UUID         `db:"id"`
	UserID        pgtype.UUID       `db:"userId"`
	Key           string            `db:"key"`
	Type          string            `db:"type"`
	Description   pgtype.Text       `db:"description"`
	Password      []byte            `db:"password"`
	ExpiresAt     pgtype.Timestamptz `db:"expiresAt"`
	AllowDownload bool              `db:"allowDownload"`
	AllowUpload   bool              `db:"allowUpload"`
	ShowMetadata  bool              `db:"showMetadata"`
	AlbumID       pgtype.UUID       `db:"albumId"`
	CreatedAt     pgtype.Timestamptz `db:"createdAt"`
	UpdatedAt     pgtype.Timestamptz `db:"updatedAt"`
	AssetCount    int64             `db:"asset_count"`
}

// CreateSharedLinkParams contains parameters for creating a shared link
type CreateSharedLinkParams struct {
	ID            uuid.UUID
	UserID        pgtype.UUID
	Key           string
	Type          string
	Description   pgtype.Text
	Password      []byte
	ExpiresAt     pgtype.Timestamptz
	AllowDownload bool
	AllowUpload   bool
	ShowMetadata  bool
	AlbumID       pgtype.UUID
}

// CreateSharedLink creates a new shared link
func (q *Queries) CreateSharedLink(ctx context.Context, arg CreateSharedLinkParams) (SharedLink, error) {
	query := `
	INSERT INTO shared_links (
		id, "userId", key, type, description, password,
		"expiresAt", "allowDownload", "allowUpload", "showMetadata",
		"albumId", "createdAt", "updatedAt"
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
	) RETURNING id, "userId", key, type, description, password, "expiresAt", 
	           "allowDownload", "allowUpload", "showMetadata", "albumId", 
	           "createdAt", "updatedAt", 0 as asset_count`

	now := pgtype.Timestamptz{
		Time:  time.Now(),
		Valid: true,
	}

	var link SharedLink
	err := q.db.QueryRow(ctx, query,
		arg.ID,
		arg.UserID,
		arg.Key,
		arg.Type,
		arg.Description,
		arg.Password,
		arg.ExpiresAt,
		arg.AllowDownload,
		arg.AllowUpload,
		arg.ShowMetadata,
		arg.AlbumID,
		now,
		now,
	).Scan(
		&link.ID,
		&link.UserID,
		&link.Key,
		&link.Type,
		&link.Description,
		&link.Password,
		&link.ExpiresAt,
		&link.AllowDownload,
		&link.AllowUpload,
		&link.ShowMetadata,
		&link.AlbumID,
		&link.CreatedAt,
		&link.UpdatedAt,
		&link.AssetCount,
	)

	return link, err
}

// GetSharedLink retrieves a shared link by ID
func (q *Queries) GetSharedLink(ctx context.Context, id uuid.UUID) (SharedLink, error) {
	query := `
	SELECT sl.*, COUNT(sla."assetId") as asset_count
	FROM shared_links sl
	LEFT JOIN shared_link_assets sla ON sl.id = sla."sharedLinkId"
	WHERE sl.id = $1
	GROUP BY sl.id`

	var link SharedLink
	err := q.db.QueryRow(ctx, query, id).Scan(
		&link.ID,
		&link.UserID,
		&link.Key,
		&link.Type,
		&link.Description,
		&link.Password,
		&link.ExpiresAt,
		&link.AllowDownload,
		&link.AllowUpload,
		&link.ShowMetadata,
		&link.AlbumID,
		&link.CreatedAt,
		&link.UpdatedAt,
		&link.AssetCount,
	)

	return link, err
}

// GetSharedLinkByKey retrieves a shared link by its key
func (q *Queries) GetSharedLinkByKey(ctx context.Context, key string) (SharedLink, error) {
	query := `
	SELECT sl.*, COUNT(sla."assetId") as asset_count
	FROM shared_links sl
	LEFT JOIN shared_link_assets sla ON sl.id = sla."sharedLinkId"
	WHERE sl.key = $1
	GROUP BY sl.id`

	var link SharedLink
	err := q.db.QueryRow(ctx, query, key).Scan(
		&link.ID,
		&link.UserID,
		&link.Key,
		&link.Type,
		&link.Description,
		&link.Password,
		&link.ExpiresAt,
		&link.AllowDownload,
		&link.AllowUpload,
		&link.ShowMetadata,
		&link.AlbumID,
		&link.CreatedAt,
		&link.UpdatedAt,
		&link.AssetCount,
	)

	return link, err
}

// ListSharedLinks lists all shared links for a user
func (q *Queries) ListSharedLinks(ctx context.Context, userID pgtype.UUID) ([]SharedLink, error) {
	query := `
	SELECT sl.*, COUNT(sla."assetId") as asset_count
	FROM shared_links sl
	LEFT JOIN shared_link_assets sla ON sl.id = sla."sharedLinkId"
	WHERE sl."userId" = $1
	GROUP BY sl.id
	ORDER BY sl."createdAt" DESC`

	rows, err := q.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []SharedLink
	for rows.Next() {
		var link SharedLink
		err := rows.Scan(
			&link.ID,
			&link.UserID,
			&link.Key,
			&link.Type,
			&link.Description,
			&link.Password,
			&link.ExpiresAt,
			&link.AllowDownload,
			&link.AllowUpload,
			&link.ShowMetadata,
			&link.AlbumID,
			&link.CreatedAt,
			&link.UpdatedAt,
			&link.AssetCount,
		)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}

	return links, rows.Err()
}

// UpdateSharedLinkParams contains parameters for updating a shared link
type UpdateSharedLinkParams struct {
	ID            uuid.UUID
	Description   pgtype.Text
	Password      []byte
	ExpiresAt     pgtype.Timestamptz
	AllowDownload bool
	AllowUpload   bool
	ShowMetadata  bool
}

// UpdateSharedLink updates an existing shared link
func (q *Queries) UpdateSharedLink(ctx context.Context, arg UpdateSharedLinkParams) (SharedLink, error) {
	query := `
	UPDATE shared_links
	SET description = $2,
	    password = $3,
	    "expiresAt" = $4,
	    "allowDownload" = $5,
	    "allowUpload" = $6,
	    "showMetadata" = $7,
	    "updatedAt" = $8
	WHERE id = $1
	RETURNING id, "userId", key, type, description, password, "expiresAt",
	          "allowDownload", "allowUpload", "showMetadata", "albumId",
	          "createdAt", "updatedAt", 0 as asset_count`

	now := pgtype.Timestamptz{
		Time:  time.Now(),
		Valid: true,
	}

	var link SharedLink
	err := q.db.QueryRow(ctx, query,
		arg.ID,
		arg.Description,
		arg.Password,
		arg.ExpiresAt,
		arg.AllowDownload,
		arg.AllowUpload,
		arg.ShowMetadata,
		now,
	).Scan(
		&link.ID,
		&link.UserID,
		&link.Key,
		&link.Type,
		&link.Description,
		&link.Password,
		&link.ExpiresAt,
		&link.AllowDownload,
		&link.AllowUpload,
		&link.ShowMetadata,
		&link.AlbumID,
		&link.CreatedAt,
		&link.UpdatedAt,
		&link.AssetCount,
	)

	return link, err
}

// DeleteSharedLink deletes a shared link
func (q *Queries) DeleteSharedLink(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM shared_links WHERE id = $1`
	_, err := q.db.Exec(ctx, query, id)
	return err
}

// AddAssetToSharedLinkParams contains parameters for adding an asset to a shared link
type AddAssetToSharedLinkParams struct {
	SharedLinkID uuid.UUID
	AssetID      pgtype.UUID
}

// AddAssetToSharedLink adds an asset to a shared link
func (q *Queries) AddAssetToSharedLink(ctx context.Context, arg AddAssetToSharedLinkParams) error {
	query := `
	INSERT INTO shared_link_assets ("sharedLinkId", "assetId")
	VALUES ($1, $2)
	ON CONFLICT DO NOTHING`
	
	_, err := q.db.Exec(ctx, query, arg.SharedLinkID, arg.AssetID)
	return err
}

// RemoveAssetFromSharedLinkParams contains parameters for removing an asset from a shared link
type RemoveAssetFromSharedLinkParams struct {
	SharedLinkID uuid.UUID
	AssetID      pgtype.UUID
}

// RemoveAssetFromSharedLink removes an asset from a shared link
func (q *Queries) RemoveAssetFromSharedLink(ctx context.Context, arg RemoveAssetFromSharedLinkParams) error {
	query := `
	DELETE FROM shared_link_assets
	WHERE "sharedLinkId" = $1 AND "assetId" = $2`
	
	_, err := q.db.Exec(ctx, query, arg.SharedLinkID, arg.AssetID)
	return err
}

// RemoveAllAssetsFromSharedLink removes all assets from a shared link
func (q *Queries) RemoveAllAssetsFromSharedLink(ctx context.Context, sharedLinkID uuid.UUID) error {
	query := `DELETE FROM shared_link_assets WHERE "sharedLinkId" = $1`
	_, err := q.db.Exec(ctx, query, sharedLinkID)
	return err
}

// GetSharedLinkAssets retrieves all asset IDs for a shared link
func (q *Queries) GetSharedLinkAssets(ctx context.Context, sharedLinkID uuid.UUID) ([]pgtype.UUID, error) {
	query := `SELECT "assetId" FROM shared_link_assets WHERE "sharedLinkId" = $1`
	
	rows, err := q.db.Query(ctx, query, sharedLinkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assetIDs []pgtype.UUID
	for rows.Next() {
		var assetID pgtype.UUID
		if err := rows.Scan(&assetID); err != nil {
			return nil, err
		}
		assetIDs = append(assetIDs, assetID)
	}

	return assetIDs, rows.Err()
}