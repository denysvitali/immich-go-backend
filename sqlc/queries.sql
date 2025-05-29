-- Album queries
-- name: GetAlbum :one
SELECT * FROM albums
WHERE id = $1 AND "deletedAt" IS NULL;

-- name: GetAlbums :many
SELECT * FROM albums
WHERE "deletedAt" IS NULL
ORDER BY "createdAt" DESC;

-- name: GetAlbumsByOwner :many
SELECT * FROM albums
WHERE "ownerId" = $1 AND "deletedAt" IS NULL
ORDER BY "createdAt" DESC;

-- name: CreateAlbum :one
INSERT INTO albums ("ownerId", "albumName", description)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateAlbum :one
UPDATE albums
SET "albumName" = COALESCE(sqlc.narg('album_name'), "albumName"),
    description = COALESCE(sqlc.narg('description'), description),
    "albumThumbnailAssetId" = COALESCE(sqlc.narg('album_thumbnail_asset_id'), "albumThumbnailAssetId"),
    "isActivityEnabled" = COALESCE(sqlc.narg('is_activity_enabled'), "isActivityEnabled"),
    "updatedAt" = now(),
    "updateId" = immich_uuid_v7()
WHERE id = $1 AND "deletedAt" IS NULL
RETURNING *;

-- name: DeleteAlbum :exec
UPDATE albums
SET "deletedAt" = now(),
    "updatedAt" = now(),
    "updateId" = immich_uuid_v7()
WHERE id = $1;

-- name: GetAlbumAssets :many
SELECT a.* FROM assets a
JOIN albums_assets_assets aaa ON a.id = aaa."assetsId"
WHERE aaa."albumsId" = $1 AND a."deletedAt" IS NULL
ORDER BY aaa."createdAt" DESC;

-- name: AddAssetToAlbum :exec
INSERT INTO albums_assets_assets ("albumsId", "assetsId")
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveAssetFromAlbum :exec
DELETE FROM albums_assets_assets
WHERE "albumsId" = $1 AND "assetsId" = $2;

-- name: GetAlbumSharedUsers :many
SELECT u.*, asu.role FROM users u
JOIN albums_shared_users_users asu ON u.id = asu."usersId"
WHERE asu."albumsId" = $1;

-- name: AddUserToAlbum :exec
INSERT INTO albums_shared_users_users ("albumsId", "usersId", role)
VALUES ($1, $2, $3)
ON CONFLICT ("albumsId", "usersId") DO UPDATE SET role = $3;

-- name: RemoveUserFromAlbum :exec
DELETE FROM albums_shared_users_users
WHERE "albumsId" = $1 AND "usersId" = $2;

-- name: GetAlbumStatistics :one
SELECT 
    COUNT(CASE WHEN "ownerId" = $1 THEN 1 END) as owned,
    COUNT(CASE WHEN "ownerId" != $1 THEN 1 END) as shared,
    COUNT(CASE WHEN "ownerId" = $1 AND NOT EXISTS(SELECT 1 FROM albums_shared_users_users WHERE "albumsId" = albums.id) THEN 1 END) as not_shared
FROM albums
WHERE ("ownerId" = $1 OR EXISTS(SELECT 1 FROM albums_shared_users_users WHERE "albumsId" = albums.id AND "usersId" = $1))
AND "deletedAt" IS NULL;

-- Asset queries
-- name: GetAsset :one
SELECT * FROM assets
WHERE id = $1 AND "deletedAt" IS NULL;

-- name: GetAssets :many
SELECT * FROM assets
WHERE "ownerId" = $1 
AND "deletedAt" IS NULL
AND (sqlc.narg('type')::text IS NULL OR type = sqlc.narg('type'))
AND (sqlc.narg('is_favorite')::boolean IS NULL OR "isFavorite" = sqlc.narg('is_favorite'))
AND (sqlc.narg('is_archived')::boolean IS NULL OR visibility = CASE WHEN sqlc.narg('is_archived')::boolean THEN 'archive' ELSE 'timeline' END)
AND (sqlc.narg('is_trashed')::boolean IS NULL OR status = CASE WHEN sqlc.narg('is_trashed')::boolean THEN 'trashed' ELSE 'active' END)
ORDER BY "localDateTime" DESC
LIMIT $2 OFFSET $3;

-- name: CountAssets :one
SELECT COUNT(*) FROM assets
WHERE "ownerId" = $1 
AND "deletedAt" IS NULL
AND (sqlc.narg('type')::text IS NULL OR type = sqlc.narg('type'))
AND (sqlc.narg('is_favorite')::boolean IS NULL OR "isFavorite" = sqlc.narg('is_favorite'))
AND (sqlc.narg('is_archived')::boolean IS NULL OR visibility = CASE WHEN sqlc.narg('is_archived')::boolean THEN 'archive' ELSE 'timeline' END)
AND (sqlc.narg('is_trashed')::boolean IS NULL OR status = CASE WHEN sqlc.narg('is_trashed')::boolean THEN 'trashed' ELSE 'active' END);

-- name: CreateAsset :one
INSERT INTO assets (
    "deviceAssetId", "ownerId", "deviceId", type, "originalPath", 
    "fileCreatedAt", "fileModifiedAt", "localDateTime", "originalFileName", 
    checksum, "isFavorite", visibility, status
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING *;

-- name: UpdateAsset :one
UPDATE assets
SET "isFavorite" = COALESCE(sqlc.narg('is_favorite'), "isFavorite"),
    visibility = COALESCE(
        CASE 
            WHEN sqlc.narg('is_archived')::boolean = true THEN 'archive'::asset_visibility_enum
            WHEN sqlc.narg('is_archived')::boolean = false THEN 'timeline'::asset_visibility_enum
            ELSE visibility
        END, 
        visibility
    ),
    "updatedAt" = now(),
    "updateId" = immich_uuid_v7()
WHERE id = $1 AND "deletedAt" IS NULL
RETURNING *;

-- name: DeleteAssets :exec
UPDATE assets
SET status = CASE WHEN $2::boolean THEN 'deleted'::assets_status_enum ELSE 'trashed'::assets_status_enum END,
    "deletedAt" = CASE WHEN $2::boolean THEN now() ELSE NULL END,
    "updatedAt" = now(),
    "updateId" = immich_uuid_v7()
WHERE id = ANY($1::uuid[]);

-- name: GetAssetsByDeviceId :many
SELECT id FROM assets
WHERE "ownerId" = $1 AND "deviceId" = $2 AND "deletedAt" IS NULL;

-- name: CheckExistingAssets :many
SELECT "deviceAssetId" FROM assets
WHERE "ownerId" = $1 AND "deviceId" = $2 AND "deviceAssetId" = ANY($3::text[]) AND "deletedAt" IS NULL;

-- name: GetAssetStatistics :one
SELECT 
    COUNT(CASE WHEN type = 'IMAGE' THEN 1 END) as images,
    COUNT(CASE WHEN type = 'VIDEO' THEN 1 END) as videos,
    COUNT(*) as total
FROM assets
WHERE "ownerId" = $1 AND "deletedAt" IS NULL AND status = 'active';

-- name: GetRandomAssets :many
SELECT * FROM assets
WHERE "ownerId" = $1 AND "deletedAt" IS NULL AND status = 'active'
ORDER BY RANDOM()
LIMIT $2;

-- User queries
-- name: GetUser :one
SELECT * FROM users
WHERE id = $1 AND "deletedAt" IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 AND "deletedAt" IS NULL;

-- name: GetUsers :many
SELECT * FROM users
WHERE "deletedAt" IS NULL
ORDER BY "createdAt" DESC;

-- name: CreateUser :one
INSERT INTO users (email, name, password, "isAdmin")
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET name = COALESCE(sqlc.narg('name'), name),
    email = COALESCE(sqlc.narg('email'), email),
    "isAdmin" = COALESCE(sqlc.narg('is_admin'), "isAdmin"),
    "avatarColor" = COALESCE(sqlc.narg('avatar_color'), "avatarColor"),
    "profileImagePath" = COALESCE(sqlc.narg('profile_image_path'), "profileImagePath"),
    "shouldChangePassword" = COALESCE(sqlc.narg('should_change_password'), "shouldChangePassword"),
    "quotaSizeInBytes" = COALESCE(sqlc.narg('quota_size_in_bytes'), "quotaSizeInBytes"),
    "storageLabel" = COALESCE(sqlc.narg('storage_label'), "storageLabel"),
    "updatedAt" = now()
WHERE id = $1 AND "deletedAt" IS NULL
RETURNING *;

-- name: DeleteUser :exec
UPDATE users
SET "deletedAt" = now(),
    "updatedAt" = now()
WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users
SET password = $2,
    "shouldChangePassword" = false,
    "updatedAt" = now()
WHERE id = $1;

-- EXIF queries
-- name: GetAssetExif :one
SELECT * FROM exif
WHERE "assetId" = $1;

-- name: CreateOrUpdateExif :one
INSERT INTO exif (
    "assetId", make, model, "exifImageWidth", "exifImageHeight", 
    "fileSizeInByte", orientation, "dateTimeOriginal", "modifyDate",
    "lensModel", "fNumber", "focalLength", iso, latitude, longitude,
    city, state, country, description
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
ON CONFLICT ("assetId") DO UPDATE SET
    make = EXCLUDED.make,
    model = EXCLUDED.model,
    "exifImageWidth" = EXCLUDED."exifImageWidth",
    "exifImageHeight" = EXCLUDED."exifImageHeight",
    "fileSizeInByte" = EXCLUDED."fileSizeInByte",
    orientation = EXCLUDED.orientation,
    "dateTimeOriginal" = EXCLUDED."dateTimeOriginal",
    "modifyDate" = EXCLUDED."modifyDate",
    "lensModel" = EXCLUDED."lensModel",
    "fNumber" = EXCLUDED."fNumber",
    "focalLength" = EXCLUDED."focalLength",
    iso = EXCLUDED.iso,
    latitude = EXCLUDED.latitude,
    longitude = EXCLUDED.longitude,
    city = EXCLUDED.city,
    state = EXCLUDED.state,
    country = EXCLUDED.country,
    description = EXCLUDED.description,
    "updatedAt" = now(),
    "updateId" = immich_uuid_v7()
RETURNING *;

-- API Key queries
-- name: GetApiKey :one
SELECT * FROM api_keys
WHERE key = $1;

-- name: GetApiKeysByUser :many
SELECT * FROM api_keys
WHERE "userId" = $1
ORDER BY "createdAt" DESC;

-- name: CreateApiKey :one
INSERT INTO api_keys (name, key, "userId", permissions)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteApiKey :exec
DELETE FROM api_keys
WHERE id = $1 AND "userId" = $2;

-- Memory queries
-- name: GetMemories :many
SELECT * FROM memories
WHERE "ownerId" = $1 AND "deletedAt" IS NULL
ORDER BY "createdAt" DESC;

-- name: GetMemory :one
SELECT * FROM memories
WHERE id = $1 AND "deletedAt" IS NULL;

-- name: CreateMemory :one
INSERT INTO memories ("ownerId", type, data)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateMemory :one
UPDATE memories
SET type = COALESCE(sqlc.narg('type'), type),
    data = COALESCE(sqlc.narg('data'), data),
    "updatedAt" = now()
WHERE id = $1 AND "deletedAt" IS NULL
RETURNING *;

-- name: DeleteMemory :exec
UPDATE memories
SET "deletedAt" = now(),
    "updatedAt" = now()
WHERE id = $1;
