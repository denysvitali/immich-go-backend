// Code generated manually for missing search queries - TODO: Replace with SQLC generated code

package sqlc

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// SearchAssetsParams represents parameters for SearchAssets query
type SearchAssetsParams struct {
	OwnerID pgtype.UUID
	Query   string
	Limit   int32
	Offset  int32
}

// SearchAssets searches for assets by query string
func (q *Queries) SearchAssets(ctx context.Context, arg SearchAssetsParams) ([]Asset, error) {
	rows, err := q.db.Query(ctx, `
		SELECT * FROM assets
		WHERE "ownerId" = $1 
		  AND "deletedAt" IS NULL
		  AND (
		    "originalFileName" ILIKE '%' || $2 || '%' OR
		    description ILIKE '%' || $2 || '%'
		  )
		ORDER BY "createdAt" DESC
		LIMIT $3 OFFSET $4`,
		arg.OwnerID, arg.Query, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Asset
	for rows.Next() {
		var i Asset
		if err := rows.Scan(
			&i.ID,
			&i.DeviceAssetId,
			&i.OwnerId,
			&i.DeviceId,
			&i.Type,
			&i.OriginalPath,
			&i.FileCreatedAt,
			&i.FileModifiedAt,
			&i.IsFavorite,
			&i.Duration,
			&i.EncodedVideoPath,
			&i.Checksum,
			&i.LivePhotoVideoId,
			&i.UpdatedAt,
			&i.CreatedAt,
			&i.OriginalFileName,
			&i.SidecarPath,
			&i.Thumbhash,
			&i.IsOffline,
			&i.LibraryId,
			&i.IsExternal,
			&i.DeletedAt,
			&i.LocalDateTime,
			&i.StackId,
			&i.DuplicateId,
			&i.Status,
			&i.UpdateId,
			&i.Visibility,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

// CountSearchAssetsParams represents parameters for CountSearchAssets query
type CountSearchAssetsParams struct {
	OwnerID pgtype.UUID
	Query   string
}

// CountSearchAssets counts assets matching search query
func (q *Queries) CountSearchAssets(ctx context.Context, arg CountSearchAssetsParams) (int64, error) {
	row := q.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM assets
		WHERE "ownerId" = $1 
		  AND "deletedAt" IS NULL
		  AND (
		    "originalFileName" ILIKE '%' || $2 || '%' OR
		    description ILIKE '%' || $2 || '%'
		  )`,
		arg.OwnerID, arg.Query)
	var count int64
	err := row.Scan(&count)
	return count, err
}

// SearchPeopleParams represents parameters for SearchPeople query
type SearchPeopleParams struct {
	OwnerID pgtype.UUID
	Query   string
	Limit   int32
	Offset  int32
}

// SearchPeople searches for people by name
func (q *Queries) SearchPeople(ctx context.Context, arg SearchPeopleParams) ([]Person, error) {
	rows, err := q.db.Query(ctx, `
		SELECT * FROM person
		WHERE "ownerId" = $1
		  AND name ILIKE '%' || $2 || '%'
		ORDER BY name
		LIMIT $3 OFFSET $4`,
		arg.OwnerID, arg.Query, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Person
	for rows.Next() {
		var i Person
		if err := rows.Scan(
			&i.ID,
			&i.OwnerId,
			&i.Name,
			&i.BirthDate,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

// SearchPlacesParams represents parameters for SearchPlaces query
type SearchPlacesParams struct {
	OwnerID pgtype.UUID
	Query   string
	Limit   int32
	Offset  int32
}

// SearchPlacesRow represents a row from SearchPlaces query
type SearchPlacesRow struct {
	City    *string
	State   *string
	Country *string
}

// SearchPlaces searches for places
func (q *Queries) SearchPlaces(ctx context.Context, arg SearchPlacesParams) ([]SearchPlacesRow, error) {
	rows, err := q.db.Query(ctx, `
		SELECT DISTINCT city, state, country FROM exif
		WHERE "assetId" IN (
		  SELECT id FROM assets WHERE "ownerId" = $1 AND "deletedAt" IS NULL
		)
		  AND (
		    city ILIKE '%' || $2 || '%' OR
		    state ILIKE '%' || $2 || '%' OR
		    country ILIKE '%' || $2 || '%'
		  )
		LIMIT $3 OFFSET $4`,
		arg.OwnerID, arg.Query, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []SearchPlacesRow
	for rows.Next() {
		var i SearchPlacesRow
		if err := rows.Scan(&i.City, &i.State, &i.Country); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

// GetDistinctCitiesParams represents parameters for GetDistinctCities query
type GetDistinctCitiesParams struct {
	OwnerID pgtype.UUID
	Limit   int32
}

// GetDistinctCities gets distinct cities
func (q *Queries) GetDistinctCities(ctx context.Context, arg GetDistinctCitiesParams) ([]string, error) {
	rows, err := q.db.Query(ctx, `
		SELECT DISTINCT city FROM exif
		WHERE "assetId" IN (
		  SELECT id FROM assets WHERE "ownerId" = $1 AND "deletedAt" IS NULL
		)
		  AND city IS NOT NULL
		  AND city != ''
		ORDER BY city
		LIMIT $2`,
		arg.OwnerID, arg.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var city string
		if err := rows.Scan(&city); err != nil {
			return nil, err
		}
		items = append(items, city)
	}
	return items, nil
}

// GetTopPeopleRow represents a row from GetTopPeople query
type GetTopPeopleRow struct {
	Person
	FaceCount int64
}

// GetTopPeople gets top people by face count
func (q *Queries) GetTopPeople(ctx context.Context, ownerID pgtype.UUID, limit int32) ([]GetTopPeopleRow, error) {
	rows, err := q.db.Query(ctx, `
		SELECT p.*, COUNT(f."personId") as face_count
		FROM person p
		LEFT JOIN asset_faces f ON p.id = f."personId"
		WHERE p."ownerId" = $1
		GROUP BY p.id
		ORDER BY face_count DESC
		LIMIT $2`,
		ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []GetTopPeopleRow
	for rows.Next() {
		var i GetTopPeopleRow
		if err := rows.Scan(
			&i.ID,
			&i.OwnerId,
			&i.Name,
			&i.BirthDate,
			&i.FaceCount,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

// CheckAssetExistsByPathParams represents parameters for CheckAssetExistsByPath query
type CheckAssetExistsByPathParams struct {
	Path string
}

// CheckAssetExistsByPath checks if an asset exists by path
func (q *Queries) CheckAssetExistsByPath(ctx context.Context, arg CheckAssetExistsByPathParams) (bool, error) {
	row := q.db.QueryRow(ctx, `
		SELECT EXISTS(
		  SELECT 1 FROM assets
		  WHERE "originalPath" = $1
		    AND "deletedAt" IS NULL
		)`,
		arg.Path)
	var exists bool
	err := row.Scan(&exists)
	return exists, err
}

// GetLibraryAssetCount gets the count of assets in a library
func (q *Queries) GetLibraryAssetCount(ctx context.Context, libraryID pgtype.UUID) (int64, error) {
	row := q.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM assets
		WHERE "libraryId" = $1
		  AND "deletedAt" IS NULL`,
		libraryID)
	var count int64
	err := row.Scan(&count)
	return count, err
}
