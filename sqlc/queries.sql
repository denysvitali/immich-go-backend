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

-- name: SearchAlbums :many
SELECT * FROM albums
WHERE "ownerId" = $1
  AND ("albumName" ILIKE '%' || $2 || '%'
       OR description ILIKE '%' || $2 || '%')
ORDER BY "createdAt" DESC
LIMIT $3 OFFSET $4;

-- name: GetAlbumSharedUsers :many
SELECT u.*, asu.role FROM users u
JOIN albums_shared_users_users asu ON u.id = asu."usersId"
WHERE asu."albumsId" = $1;

-- name: CheckAssetSharedWithUser :one
SELECT EXISTS(
    SELECT 1 FROM albums_assets_assets aaa
    JOIN albums_shared_users_users asuu ON aaa."albumsId" = asuu."albumsId"
    JOIN albums a ON a.id = aaa."albumsId"
    WHERE aaa."assetsId" = $1
    AND asuu."usersId" = $2
    AND a."deletedAt" IS NULL
) AS is_shared;

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

-- name: CreateLibraryAsset :one
INSERT INTO assets (
    "deviceAssetId", "ownerId", "libraryId", "deviceId", type, "originalPath",
    "fileCreatedAt", "fileModifiedAt", "localDateTime", "originalFileName",
    checksum, "isFavorite", visibility, status, "isExternal"
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, true)
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

-- name: UpdateAssetStatus :one
UPDATE assets
SET status = $2,
    "updatedAt" = now(),
    "updateId" = immich_uuid_v7()
WHERE id = $1 AND "deletedAt" IS NULL
RETURNING *;

-- name: GetAssetByID :one
SELECT * FROM assets
WHERE id = $1 AND "deletedAt" IS NULL;

-- name: GetAssetByIDAndUser :one
SELECT * FROM assets
WHERE id = $1 AND "ownerId" = $2 AND "deletedAt" IS NULL;

-- name: GetUserAssets :many
SELECT * FROM assets
WHERE "ownerId" = $1 AND "deletedAt" IS NULL
AND (sqlc.narg('status')::assets_status_enum IS NULL OR status = sqlc.narg('status')::assets_status_enum)
ORDER BY "fileCreatedAt" DESC
LIMIT sqlc.narg('limit')
OFFSET sqlc.narg('offset');

-- EXIF queries
-- name: CreateExif :one
INSERT INTO exif (
    "assetId", make, model, "exifImageWidth", "exifImageHeight", 
    "fileSizeInByte", orientation, "dateTimeOriginal", "modifyDate",
    "lensModel", "fNumber", "focalLength", iso, latitude, longitude,
    city, state, country, description, fps
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
RETURNING *;

-- name: GetExifByAssetId :one
SELECT * FROM exif
WHERE "assetId" = $1;

-- name: UpdateExif :one
UPDATE exif
SET make = $2, model = $3, "exifImageWidth" = $4, "exifImageHeight" = $5,
    "fileSizeInByte" = $6, orientation = $7, "dateTimeOriginal" = $8, "modifyDate" = $9,
    "lensModel" = $10, "fNumber" = $11, "focalLength" = $12, iso = $13, 
    latitude = $14, longitude = $15, city = $16, state = $17, country = $18,
    description = $19, fps = $20
WHERE "assetId" = $1
RETURNING *;

-- name: DeleteExif :exec
DELETE FROM exif
WHERE "assetId" = $1;

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
INSERT INTO users (id, email, name, password, "isAdmin")
VALUES ($1, $2, $3, $4, $5)
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

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 AND "deletedAt" IS NULL;

-- name: UpdateUserLastLogin :exec
UPDATE users
SET "updatedAt" = now()
WHERE id = $1;

-- Session/Refresh Token queries
-- name: CreateRefreshToken :exec
INSERT INTO sessions (token, "userId", "expiresAt")
VALUES ($1, $2, $3);

-- name: GetRefreshToken :one
SELECT * FROM sessions
WHERE token = $1 AND ("expiresAt" IS NULL OR "expiresAt" > now());

-- name: DeleteRefreshToken :exec
DELETE FROM sessions
WHERE token = $1;

-- name: DeleteUserRefreshTokens :exec
DELETE FROM sessions
WHERE "userId" = $1;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM sessions
WHERE "expiresAt" IS NOT NULL AND "expiresAt" <= now();

-- Additional User Management queries
-- name: ListUsers :many
SELECT * FROM users
WHERE "deletedAt" IS NULL
AND (sqlc.narg('include_deleted')::boolean IS NULL OR sqlc.narg('include_deleted')::boolean = false OR "deletedAt" IS NOT NULL)
ORDER BY "createdAt" DESC
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT COUNT(*) FROM users
WHERE "deletedAt" IS NULL
AND (sqlc.narg('include_deleted')::boolean IS NULL OR sqlc.narg('include_deleted')::boolean = false OR "deletedAt" IS NOT NULL);

-- name: UpdateUserAdmin :one
UPDATE users
SET "isAdmin" = $2,
    "updatedAt" = now()
WHERE id = $1 AND "deletedAt" IS NULL
RETURNING *;

-- name: SoftDeleteUser :exec
UPDATE users
SET "deletedAt" = now(),
    "updatedAt" = now()
WHERE id = $1;

-- name: HardDeleteUser :exec
DELETE FROM users
WHERE id = $1;

-- name: RestoreUser :one
UPDATE users
SET "deletedAt" = NULL,
    "updatedAt" = now()
WHERE id = $1
RETURNING *;

-- name: ClearAllOAuthIds :exec
UPDATE users
SET "oauthId" = '',
    "updatedAt" = now()
WHERE "oauthId" != '';

-- name: UpdateUserOAuthId :one
UPDATE users
SET "oauthId" = $2,
    "updatedAt" = now()
WHERE id = $1 AND "deletedAt" IS NULL
RETURNING *;

-- name: GetUserByOAuthId :one
SELECT * FROM users
WHERE "oauthId" = $1 AND "deletedAt" IS NULL;

-- name: GetAllUsers :many
SELECT * FROM users
WHERE "deletedAt" IS NULL
ORDER BY "createdAt" DESC;

-- name: GetAllUsersWithDeleted :many
SELECT * FROM users
ORDER BY "createdAt" DESC;

-- name: SearchUsersAdmin :many
SELECT * FROM users
WHERE (sqlc.narg('with_deleted')::boolean IS NULL OR sqlc.narg('with_deleted')::boolean = true OR "deletedAt" IS NULL)
AND (sqlc.narg('email')::text IS NULL OR email ILIKE '%' || sqlc.narg('email') || '%')
AND (sqlc.narg('name')::text IS NULL OR name ILIKE '%' || sqlc.narg('name') || '%')
ORDER BY "createdAt" DESC
LIMIT sqlc.narg('limit')
OFFSET sqlc.narg('offset');

-- name: GetUserOnboarding :one
SELECT value FROM user_metadata
WHERE "userId" = $1 AND key = 'onboarding';

-- name: UpdateUserOnboarding :exec
INSERT INTO user_metadata ("userId", key, value)
VALUES ($1, 'onboarding', $2)
ON CONFLICT ("userId", key) DO UPDATE SET value = $2;

-- User Preferences queries using user_metadata table
-- name: GetUserPreferencesData :one
SELECT value FROM user_metadata
WHERE "userId" = $1 AND key = 'preferences';

-- name: UpdateUserPreferencesData :one
INSERT INTO user_metadata ("userId", key, value)
VALUES ($1, 'preferences', $2)
ON CONFLICT ("userId", key) DO UPDATE SET value = $2
RETURNING value;

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
    "isSaved" = COALESCE(sqlc.narg('is_saved'), "isSaved"),
    "updatedAt" = now()
WHERE id = $1 AND "deletedAt" IS NULL
RETURNING *;

-- name: DeleteMemory :exec
UPDATE memories
SET "deletedAt" = now(),
    "updatedAt" = now()
WHERE id = $1;

-- name: AddAssetsToMemory :exec
INSERT INTO memories_assets_assets ("memoriesId", "assetsId")
SELECT $1, unnest($2::uuid[])
ON CONFLICT ("memoriesId", "assetsId") DO NOTHING;

-- name: RemoveAssetsFromMemory :exec
DELETE FROM memories_assets_assets
WHERE "memoriesId" = $1 AND "assetsId" = ANY($2::uuid[]);

-- name: GetMemoryAssets :many
SELECT "assetsId" FROM memories_assets_assets
WHERE "memoriesId" = $1;

-- ============================================================================
-- PEOPLE & FACES QUERIES
-- ============================================================================

-- name: GetPerson :one
SELECT * FROM person
WHERE id = $1;

-- name: GetPeople :many
SELECT * FROM person
WHERE "ownerId" = $1
ORDER BY "updatedAt" DESC;

-- name: CreatePerson :one
INSERT INTO person ("ownerId", name, "birthDate", "thumbnailPath", "faceAssetId", "isHidden")
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdatePerson :one
UPDATE person
SET name = COALESCE(sqlc.narg('name'), name),
    "birthDate" = COALESCE(sqlc.narg('birth_date'), "birthDate"),
    "thumbnailPath" = COALESCE(sqlc.narg('thumbnail_path'), "thumbnailPath"),
    "faceAssetId" = COALESCE(sqlc.narg('face_asset_id'), "faceAssetId"),
    "isHidden" = COALESCE(sqlc.narg('is_hidden'), "isHidden"),
    "updatedAt" = now()
WHERE id = $1
RETURNING *;

-- name: DeletePerson :exec
DELETE FROM person
WHERE id = $1;

-- name: GetPersonAssets :many
SELECT DISTINCT a.* FROM assets a
JOIN asset_faces af ON a.id = af."assetId"
WHERE af."personId" = $1 AND a."deletedAt" IS NULL
ORDER BY a."localDateTime" DESC
LIMIT $2 OFFSET $3;

-- name: CountPersonAssets :one
SELECT COUNT(DISTINCT a.id) FROM assets a
JOIN asset_faces af ON a.id = af."assetId"
WHERE af."personId" = $1 AND a."deletedAt" IS NULL;

-- name: GetAssetFaces :many
SELECT af.*, p.name as person_name FROM asset_faces af
LEFT JOIN person p ON af."personId" = p.id
WHERE af."assetId" = $1;

-- name: CreateAssetFace :one
INSERT INTO asset_faces ("assetId", "personId", "imageWidth", "imageHeight", "boundingBoxX1", "boundingBoxY1", "boundingBoxX2", "boundingBoxY2")
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateAssetFace :one
UPDATE asset_faces
SET "personId" = COALESCE(sqlc.narg('person_id'), "personId"),
    "boundingBoxX1" = COALESCE(sqlc.narg('bounding_box_x1'), "boundingBoxX1"),
    "boundingBoxY1" = COALESCE(sqlc.narg('bounding_box_y1'), "boundingBoxY1"),
    "boundingBoxX2" = COALESCE(sqlc.narg('bounding_box_x2'), "boundingBoxX2"),
    "boundingBoxY2" = COALESCE(sqlc.narg('bounding_box_y2'), "boundingBoxY2")
WHERE id = $1
RETURNING *;

-- name: DeleteAssetFace :exec
DELETE FROM asset_faces
WHERE id = $1;

-- name: GetFaceSearch :many
SELECT * FROM face_search
WHERE "faceId" = $1;

-- name: CreateFaceSearch :one
INSERT INTO face_search ("faceId", embedding)
VALUES ($1, $2)
RETURNING *;

-- name: SearchFacesByEmbedding :many
SELECT fs.*, p.name as person_name
FROM face_search fs
JOIN person p ON fs."personId" = p.id
WHERE fs.embedding <-> $1 < $2
ORDER BY fs.embedding <-> $1
LIMIT $3;

-- ============================================================================
-- LIBRARIES QUERIES
-- ============================================================================

-- name: GetLibrary :one
SELECT * FROM libraries
WHERE id = $1 AND "deletedAt" IS NULL;

-- name: GetLibraries :many
SELECT * FROM libraries
WHERE "ownerId" = $1 AND "deletedAt" IS NULL
ORDER BY "createdAt" DESC;

-- name: CreateLibrary :one
INSERT INTO libraries ("ownerId", name, "importPaths", "exclusionPatterns")
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateLibrary :one
UPDATE libraries
SET name = COALESCE(sqlc.narg('name'), name),
    "importPaths" = COALESCE(sqlc.narg('import_paths'), "importPaths"),
    "exclusionPatterns" = COALESCE(sqlc.narg('exclusion_patterns'), "exclusionPatterns"),
    "updatedAt" = now()
WHERE id = $1 AND "deletedAt" IS NULL
RETURNING *;

-- name: DeleteLibrary :exec
UPDATE libraries
SET "deletedAt" = now(),
    "updatedAt" = now()
WHERE id = $1;

-- name: UpdateLibraryRefreshedAt :exec
UPDATE libraries
SET "refreshedAt" = now(),
    "updatedAt" = now()
WHERE id = $1 AND "deletedAt" IS NULL;

-- name: GetLibraryAssets :many
SELECT * FROM assets
WHERE "libraryId" = $1 AND "deletedAt" IS NULL
ORDER BY "localDateTime" DESC
LIMIT $2 OFFSET $3;

-- name: CountLibraryAssets :one
SELECT COUNT(*) FROM assets
WHERE "libraryId" = $1 AND "deletedAt" IS NULL;

-- ============================================================================
-- JOBS & PROCESSING QUERIES
-- ============================================================================

-- name: GetAssetJobStatus :one
SELECT * FROM asset_job_status
WHERE "assetId" = $1;

-- name: CreateAssetJobStatus :one
INSERT INTO asset_job_status ("assetId", "facesRecognizedAt", "metadataExtractedAt", "duplicatesDetectedAt", "previewAt", "thumbnailAt")
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateAssetJobStatus :one
UPDATE asset_job_status
SET "facesRecognizedAt" = COALESCE(sqlc.narg('faces_recognized_at'), "facesRecognizedAt"),
    "metadataExtractedAt" = COALESCE(sqlc.narg('metadata_extracted_at'), "metadataExtractedAt"),
    "duplicatesDetectedAt" = COALESCE(sqlc.narg('duplicates_detected_at'), "duplicatesDetectedAt"),
    "previewAt" = COALESCE(sqlc.narg('preview_at'), "previewAt"),
    "thumbnailAt" = COALESCE(sqlc.narg('thumbnail_at'), "thumbnailAt")
WHERE "assetId" = $1
RETURNING *;

-- name: GetAssetsNeedingThumbnails :many
SELECT a.* FROM assets a
LEFT JOIN asset_job_status ajs ON a.id = ajs."assetId"
WHERE a."deletedAt" IS NULL 
AND (ajs."thumbnailGeneratedAt" IS NULL OR ajs."thumbnailGeneratedAt" < a."updatedAt")
ORDER BY a."createdAt" DESC
LIMIT $1;

-- name: GetAssetsNeedingMetadata :many
SELECT a.* FROM assets a
LEFT JOIN asset_job_status ajs ON a.id = ajs."assetId"
WHERE a."deletedAt" IS NULL 
AND (ajs."metadataExtractedAt" IS NULL OR ajs."metadataExtractedAt" < a."updatedAt")
ORDER BY a."createdAt" DESC
LIMIT $1;

-- name: GetAssetsNeedingFaceDetection :many
SELECT a.* FROM assets a
LEFT JOIN asset_job_status ajs ON a.id = ajs."assetId"
WHERE a."deletedAt" IS NULL 
AND a.type = 'IMAGE'
AND (ajs."facesRecognizedAt" IS NULL OR ajs."facesRecognizedAt" < a."updatedAt")
ORDER BY a."createdAt" DESC
LIMIT $1;

-- ============================================================================
-- SEARCH & SMART SEARCH QUERIES
-- ============================================================================

-- name: GetSmartSearch :many
SELECT * FROM smart_search
WHERE "assetId" = $1;

-- name: CreateSmartSearch :one
INSERT INTO smart_search ("assetId", embedding)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateSmartSearch :one
UPDATE smart_search
SET embedding = $2,
    "updatedAt" = now()
WHERE "assetId" = $1
RETURNING *;

-- name: SearchAssetsByEmbedding :many
SELECT ss.*, a.* FROM smart_search ss
JOIN assets a ON ss."assetId" = a.id
WHERE a."ownerId" = $1 
AND a."deletedAt" IS NULL
AND ss.embedding <-> $2 < $3
ORDER BY ss.embedding <-> $2
LIMIT $4;

-- name: SearchAssetsByText :many
SELECT DISTINCT a.* FROM assets a
LEFT JOIN exif e ON a.id = e."assetId"
WHERE a."ownerId" = $1 
AND a."deletedAt" IS NULL
AND (
    a."originalFileName" ILIKE '%' || $2 || '%'
    OR e.description ILIKE '%' || $2 || '%'
    OR e."imageName" ILIKE '%' || $2 || '%'
    OR e.city ILIKE '%' || $2 || '%'
    OR e.state ILIKE '%' || $2 || '%'
    OR e.country ILIKE '%' || $2 || '%'
)
ORDER BY a."localDateTime" DESC
LIMIT $3 OFFSET $4;

-- ============================================================================
-- TAGS QUERIES
-- ============================================================================

-- name: GetTag :one
SELECT * FROM tags
WHERE id = $1;

-- name: GetTags :many
SELECT * FROM tags
WHERE "userId" = $1
ORDER BY value ASC;

-- name: CreateTag :one
INSERT INTO tags ("userId", value, color)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateTag :one
UPDATE tags
SET value = COALESCE(sqlc.narg('value'), value),
    color = COALESCE(sqlc.narg('color'), color),
    "updatedAt" = now()
WHERE id = $1
RETURNING *;

-- name: DeleteTag :exec
DELETE FROM tags
WHERE id = $1;

-- name: GetAssetTags :many
SELECT t.* FROM tags t
JOIN tag_asset ta ON t.id = ta."tagsId"
WHERE ta."assetsId" = $1;

-- name: AddTagToAsset :exec
INSERT INTO tag_asset ("tagsId", "assetsId")
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveTagFromAsset :exec
DELETE FROM tag_asset
WHERE "tagsId" = $1 AND "assetsId" = $2;

-- ============================================================================
-- SHARED LINKS QUERIES
-- ============================================================================

-- name: GetSharedLink :one
SELECT * FROM shared_links
WHERE id = $1 AND "deletedAt" IS NULL;

-- name: GetSharedLinkByKey :one
SELECT * FROM shared_links
WHERE key = $1 AND "deletedAt" IS NULL;

-- name: GetSharedLinks :many
SELECT * FROM shared_links
WHERE "userId" = $1 AND "deletedAt" IS NULL
ORDER BY "createdAt" DESC;

-- name: CreateSharedLink :one
INSERT INTO shared_links ("userId", key, type, "albumId", "expiresAt", "allowUpload", "allowDownload", description, password, "showExif")
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateSharedLink :one
UPDATE shared_links
SET "expiresAt" = COALESCE(sqlc.narg('expires_at'), "expiresAt"),
    "allowUpload" = COALESCE(sqlc.narg('allow_upload'), "allowUpload"),
    "allowDownload" = COALESCE(sqlc.narg('allow_download'), "allowDownload"),
    description = COALESCE(sqlc.narg('description'), description),
    password = COALESCE(sqlc.narg('password'), password),
    "showExif" = COALESCE(sqlc.narg('show_exif'), "showExif")
WHERE id = $1
RETURNING *;

-- name: DeleteSharedLink :exec
DELETE FROM shared_links
WHERE id = $1;

-- name: GetSharedLinkAssets :many
SELECT a.* FROM assets a
JOIN shared_link__asset sla ON a.id = sla."assetsId"
WHERE sla."sharedLinksId" = $1 AND a."deletedAt" IS NULL
ORDER BY a."localDateTime" DESC;

-- name: AddAssetToSharedLink :exec
INSERT INTO shared_link__asset ("sharedLinksId", "assetsId")
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveAssetFromSharedLink :exec
DELETE FROM shared_link__asset
WHERE "sharedLinksId" = $1 AND "assetsId" = $2;

-- ============================================================================
-- ACTIVITY QUERIES
-- ============================================================================

-- name: GetActivity :one
SELECT * FROM activity
WHERE id = $1;

-- name: GetAlbumActivity :many
SELECT a.*, u.name as user_name, u.email as user_email FROM activity a
JOIN users u ON a."userId" = u.id
WHERE a."albumId" = $1
ORDER BY a."createdAt" DESC
LIMIT $2 OFFSET $3;

-- name: CreateActivity :one
INSERT INTO activity ("userId", "albumId", "assetId", comment, "isLiked")
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: DeleteActivity :exec
DELETE FROM activity
WHERE id = $1;

-- ============================================================================
-- PARTNERS QUERIES
-- ============================================================================

-- name: GetPartners :many
SELECT u.*, p."sharedById", p."sharedWithId", p."inTimeline", p."createdAt" as partnership_created_at, p."updatedAt" as partnership_updated_at FROM partners p
JOIN users u ON (u.id = p."sharedById" OR u.id = p."sharedWithId")
WHERE (p."sharedById" = $1 OR p."sharedWithId" = $1) AND u.id != $1;

-- name: CreatePartnership :one
INSERT INTO partners ("sharedById", "sharedWithId")
VALUES ($1, $2)
RETURNING *;

-- name: DeletePartnership :exec
DELETE FROM partners
WHERE ("sharedById" = $1 AND "sharedWithId" = $2) OR ("sharedById" = $2 AND "sharedWithId" = $1);

-- name: UpdatePartnership :one
UPDATE partners
SET "inTimeline" = $3,
    "updatedAt" = now()
WHERE ("sharedById" = $1 AND "sharedWithId" = $2) OR ("sharedById" = $2 AND "sharedWithId" = $1)
RETURNING *;

-- ============================================================================
-- SYSTEM & METADATA QUERIES
-- ============================================================================

-- name: GetSystemMetadata :one
SELECT * FROM system_metadata
WHERE key = $1;

-- name: SetSystemMetadata :one
INSERT INTO system_metadata (key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = $2, "updatedAt" = now()
RETURNING *;

-- name: GetUserMetadata :one
SELECT * FROM user_metadata
WHERE "userId" = $1 AND key = $2;

-- name: SetUserMetadata :one
INSERT INTO user_metadata ("userId", key, value)
VALUES ($1, $2, $3)
ON CONFLICT ("userId", key) DO UPDATE SET value = $3, "updatedAt" = now()
RETURNING *;

-- name: GetUserPreferences :many
SELECT * FROM user_metadata
WHERE "userId" = $1;

-- ============================================================================
-- NOTIFICATIONS QUERIES
-- ============================================================================

-- name: GetNotifications :many
SELECT * FROM notifications
WHERE "userId" = $1 AND "deletedAt" IS NULL
ORDER BY "createdAt" DESC
LIMIT $2 OFFSET $3;

-- name: GetNotification :one
SELECT * FROM notifications
WHERE id = $1 AND "deletedAt" IS NULL;

-- name: CreateNotification :one
INSERT INTO notifications ("userId", level, type, data, title, description)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateNotification :one
UPDATE notifications
SET "readAt" = $2,
    "updatedAt" = now()
WHERE id = $1 AND "deletedAt" IS NULL
RETURNING *;

-- name: DeleteNotification :exec
UPDATE notifications
SET "deletedAt" = now(),
    "updatedAt" = now()
WHERE id = $1;

-- name: MarkNotificationAsRead :one
UPDATE notifications
SET "readAt" = now(),
    "updatedAt" = now()
WHERE id = $1 AND "deletedAt" IS NULL
RETURNING *;

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM notifications
WHERE "userId" = $1 AND "readAt" IS NULL AND "deletedAt" IS NULL;

-- ============================================================================
-- TIMELINE & STATISTICS QUERIES
-- ============================================================================

-- name: GetTimelineBuckets :many
SELECT 
    date_trunc($2, "localDateTime") as time_bucket,
    COUNT(*) as count
FROM assets
WHERE "ownerId" = $1 
AND "deletedAt" IS NULL
AND status = 'active'
AND visibility = 'timeline'
GROUP BY date_trunc($2, "localDateTime")
ORDER BY time_bucket DESC;

-- name: GetAssetsByTimeBucket :many
SELECT * FROM assets
WHERE "ownerId" = $1 
AND "deletedAt" IS NULL
AND status = 'active'
AND visibility = 'timeline'
AND date_trunc($2, "localDateTime") = $3
ORDER BY "localDateTime" DESC
LIMIT $4 OFFSET $5;

-- name: GetAssetStatsByUser :one
SELECT 
    COUNT(*) as total,
    COUNT(CASE WHEN type = 'IMAGE' THEN 1 END) as images,
    COUNT(CASE WHEN type = 'VIDEO' THEN 1 END) as videos,
    COUNT(CASE WHEN "isFavorite" = true THEN 1 END) as favorites,
    COUNT(CASE WHEN visibility = 'archive' THEN 1 END) as archived,
    COUNT(CASE WHEN status = 'trashed' THEN 1 END) as trashed,
    SUM("originalFileSize") as total_size
FROM assets
WHERE "ownerId" = $1 AND "deletedAt" IS NULL;

-- name: GetStorageUsageByUser :one
SELECT 
    "ownerId",
    COUNT(*) as asset_count,
    SUM("originalFileSize") as total_size,
    SUM(CASE WHEN type = 'IMAGE' THEN "originalFileSize" ELSE 0 END) as image_size,
    SUM(CASE WHEN type = 'VIDEO' THEN "originalFileSize" ELSE 0 END) as video_size
FROM assets
WHERE "ownerId" = $1 AND "deletedAt" IS NULL
GROUP BY "ownerId";

-- ============================================================================
-- ADVANCED ASSET QUERIES
-- ============================================================================

-- name: GetAssetsByLocation :many
SELECT a.* FROM assets a
JOIN exif e ON a.id = e."assetId"
WHERE a."ownerId" = $1 
AND a."deletedAt" IS NULL
AND e.latitude IS NOT NULL 
AND e.longitude IS NOT NULL
AND e.latitude BETWEEN $2 AND $3
AND e.longitude BETWEEN $4 AND $5
ORDER BY a."localDateTime" DESC
LIMIT $6 OFFSET $7;

-- name: GetAssetsByDateRange :many
SELECT * FROM assets
WHERE "ownerId" = $1 
AND "deletedAt" IS NULL
AND "localDateTime" BETWEEN $2 AND $3
ORDER BY "localDateTime" DESC
LIMIT $4 OFFSET $5;

-- name: GetDuplicateAssets :many
SELECT a1.*, a2.id as duplicate_id FROM assets a1
JOIN assets a2 ON a1.checksum = a2.checksum AND a1.id < a2.id
WHERE a1."ownerId" = $1 AND a1."deletedAt" IS NULL AND a2."deletedAt" IS NULL
ORDER BY a1."localDateTime" DESC;

-- name: GetAssetsByChecksum :many
SELECT * FROM assets
WHERE checksum = $1 AND "deletedAt" IS NULL;

-- name: GetRecentAssets :many
SELECT * FROM assets
WHERE "ownerId" = $1 
AND "deletedAt" IS NULL
AND status = 'active'
AND visibility = 'timeline'
ORDER BY "createdAt" DESC
LIMIT $2;

-- ============================================================================
-- TRASH QUERIES
-- ============================================================================

-- name: GetTrashedAssetsByUser :many
SELECT * FROM assets
WHERE "ownerId" = $1 
AND "deletedAt" IS NULL
AND status = 'trashed'
ORDER BY "updatedAt" DESC;

-- name: RestoreAssetFromTrash :exec
UPDATE assets
SET status = 'active',
    "updatedAt" = now()
WHERE id = $1 AND status = 'trashed';

-- name: PermanentlyDeleteAsset :exec
UPDATE assets
SET "deletedAt" = now(),
    "updatedAt" = now()
WHERE id = $1;

-- name: MoveAssetToTrash :exec
UPDATE assets
SET status = 'trashed',
    "updatedAt" = now()
WHERE id = $1 AND "deletedAt" IS NULL;

-- ============================================================================
-- TAG QUERIES
-- ============================================================================

-- name: GetTagsByUser :many
SELECT * FROM tags
WHERE "userId" = $1
ORDER BY value ASC;

-- name: GetTagByID :one
SELECT * FROM tags
WHERE id = $1;

-- name: GetTagByValue :one
SELECT * FROM tags
WHERE "userId" = $1 AND value = $2;

-- name: GetTagAssets :many
SELECT a.* FROM assets a
JOIN tag_asset ta ON a.id = ta."assetsId"
WHERE ta."tagsId" = $1 AND a."deletedAt" IS NULL;

-- name: GetFavoriteAssets :many
SELECT * FROM assets
WHERE "ownerId" = $1 
AND "deletedAt" IS NULL
AND "isFavorite" = true
ORDER BY "localDateTime" DESC
LIMIT $2 OFFSET $3;

-- name: GetArchivedAssets :many
SELECT * FROM assets
WHERE "ownerId" = $1 
AND "deletedAt" IS NULL
AND visibility = 'archive'
ORDER BY "localDateTime" DESC
LIMIT $2 OFFSET $3;

-- name: GetTrashedAssets :many
SELECT * FROM assets
WHERE "ownerId" = $1 
AND "deletedAt" IS NULL
AND status = 'trashed'
ORDER BY "updatedAt" DESC
LIMIT $2 OFFSET $3;

-- name: RestoreAssets :exec
UPDATE assets
SET status = 'active',
    "updatedAt" = now()
WHERE id = ANY($1::uuid[]) AND "ownerId" = $2;

-- name: PermanentlyDeleteAssets :exec
UPDATE assets
SET "deletedAt" = now(),
    "updatedAt" = now()
WHERE id = ANY($1::uuid[]) AND "ownerId" = $2;

-- Asset Files queries
-- name: CreateAssetFile :one
INSERT INTO asset_files ("assetId", "type", "path")
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetAssetFiles :many
SELECT * FROM asset_files
WHERE "assetId" = $1
ORDER BY "createdAt" ASC;

-- name: GetAssetFilesByType :many
SELECT * FROM asset_files
WHERE "assetId" = $1 AND "type" = $2
ORDER BY "createdAt" ASC;

-- name: GetAssetFile :one
SELECT * FROM asset_files
WHERE "assetId" = $1 AND "type" = $2
LIMIT 1;

-- name: DeleteAssetFile :exec
DELETE FROM asset_files
WHERE "assetId" = $1 AND "type" = $2;

-- name: DeleteAssetFiles :exec
DELETE FROM asset_files
WHERE "assetId" = $1;

-- Search queries for basic functionality
-- name: SearchAssets :many
SELECT * FROM assets
WHERE "ownerId" = $1 
  AND "deletedAt" IS NULL
  AND (
    "originalFileName" ILIKE '%' || $2 || '%' OR
    description ILIKE '%' || $2 || '%'
  )
ORDER BY "createdAt" DESC
LIMIT $3 OFFSET $4;

-- name: CountSearchAssets :one
SELECT COUNT(*) FROM assets
WHERE "ownerId" = $1 
  AND "deletedAt" IS NULL
  AND (
    "originalFileName" ILIKE '%' || $2 || '%' OR
    description ILIKE '%' || $2 || '%'
  );

-- name: SearchPeople :many
SELECT * FROM person
WHERE "ownerId" = $1
  AND name ILIKE '%' || $2 || '%'
ORDER BY name
LIMIT $3 OFFSET $4;

-- name: SearchPlaces :many
SELECT DISTINCT city, state, country FROM exif
WHERE "assetId" IN (
  SELECT id FROM assets WHERE "ownerId" = $1 AND "deletedAt" IS NULL
)
  AND (
    city ILIKE '%' || $2 || '%' OR
    state ILIKE '%' || $2 || '%' OR
    country ILIKE '%' || $2 || '%'
  )
LIMIT $3 OFFSET $4;

-- name: GetDistinctCities :many
SELECT DISTINCT city FROM exif
WHERE "assetId" IN (
  SELECT id FROM assets WHERE "ownerId" = $1 AND "deletedAt" IS NULL
)
  AND city IS NOT NULL
  AND city != ''
ORDER BY city
LIMIT $2;

-- name: GetDistinctCameras :many
SELECT DISTINCT make, model FROM exif
WHERE "assetId" IN (
  SELECT id FROM assets WHERE "ownerId" = $1 AND "deletedAt" IS NULL
)
  AND make IS NOT NULL
  AND make != ''
  AND model IS NOT NULL
  AND model != ''
ORDER BY make, model
LIMIT $2;

-- name: GetTopPeople :many
SELECT p.*, COUNT(f."personId") as face_count
FROM person p
LEFT JOIN asset_faces f ON p.id = f."personId"
WHERE p."ownerId" = $1
GROUP BY p.id
ORDER BY face_count DESC
LIMIT $2;

-- name: CheckAssetExistsByPath :one
SELECT EXISTS(
  SELECT 1 FROM assets
  WHERE "originalPath" = $1
    AND "deletedAt" IS NULL
);

-- name: GetLibraryAssetCount :one
SELECT COUNT(*) FROM assets
WHERE "libraryId" = $1
  AND "deletedAt" IS NULL;


-- ================== SYSTEM CONFIGURATION ==================
-- Note: system_config table doesn't exist in current schema
-- These queries are commented until the table is added

-- -- name: GetAllSystemConfig :many
-- SELECT key, value FROM system_config
-- ORDER BY key;

-- -- name: GetSystemConfig :one
-- SELECT value FROM system_config
-- WHERE key = $1;

-- -- name: UpsertSystemConfig :exec
-- INSERT INTO system_config (key, value)
-- VALUES ($1, $2)
-- ON CONFLICT (key) DO UPDATE
-- SET value = $2, "updatedAt" = NOW();

-- -- name: DeleteSystemConfig :exec
-- DELETE FROM system_config WHERE key = $1;

-- ================== ADDITIONAL ASSET PATH QUERIES ==================

-- name: GetAssetByPath :one
SELECT * FROM assets
WHERE "originalPath" = $1
AND "deletedAt" IS NULL
LIMIT 1;

-- ================== VIEW TRACKING QUERIES ==================

-- name: RecordAssetView :exec
INSERT INTO asset_views (asset_id, user_id, viewed_at)
VALUES ($1, $2, NOW())
ON CONFLICT (asset_id, user_id) DO UPDATE
SET viewed_at = NOW();

-- name: GetAssetViewCount :one
SELECT COUNT(DISTINCT user_id) as view_count
FROM asset_views
WHERE asset_id = $1;

-- name: GetUserRecentViews :many
SELECT DISTINCT ON (asset_id)
    asset_id,
    viewed_at
FROM asset_views
WHERE user_id = $1
ORDER BY asset_id, viewed_at DESC
LIMIT $2;

-- ================== LOCATION/PLACE QUERIES ==================

-- name: GetTopPlaces :many
SELECT
    e.city,
    e.state,
    e.country,
    COUNT(*) as asset_count
FROM exif e
INNER JOIN assets a ON e."assetId" = a.id
WHERE a."deletedAt" IS NULL
AND e.city IS NOT NULL
GROUP BY e.city, e.state, e.country
ORDER BY asset_count DESC
LIMIT $1;

-- ================== FACE RECOGNITION QUERIES ==================

-- name: CreateFace :one
INSERT INTO asset_faces (
    id, "assetId", "personId",
    "boundingBoxX1", "boundingBoxY1",
    "boundingBoxX2", "boundingBoxY2",
    "imageWidth", "imageHeight"
) VALUES (
    gen_uuid_v7(), $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetFacesByAsset :many
SELECT * FROM asset_faces
WHERE "assetId" = $1
AND "deletedAt" IS NULL;

-- name: GetFacesByPerson :many
SELECT * FROM asset_faces
WHERE "personId" = $1
AND "deletedAt" IS NULL;

-- name: DeleteFace :exec
UPDATE asset_faces
SET "deletedAt" = NOW()
WHERE id = $1;

-- ================== ASSET FILE SIZE/DIMENSIONS ==================

-- name: UpdateAssetFileInfo :exec
UPDATE exif
SET
    "fileSizeInByte" = $2,
    "exifImageWidth" = $3,
    "exifImageHeight" = $4,
    "updatedAt" = NOW()
WHERE "assetId" = $1;

-- ================== SESSION MANAGEMENT ==================

-- name: CreateSession :one
INSERT INTO sessions (
    id, token, "userId", "deviceType", "deviceOS",
    "expiresAt", "createdAt", "updatedAt"
) VALUES (
    gen_uuid_v7(), $1, $2, $3, $4, $5, NOW(), NOW()
) RETURNING *;

-- name: GetSession :one
SELECT * FROM sessions
WHERE id = $1;

-- name: GetSessionByToken :one
SELECT * FROM sessions
WHERE token = $1;

-- name: GetUserSessions :many
SELECT * FROM sessions
WHERE "userId" = $1
ORDER BY "createdAt" DESC;

-- name: UpdateSessionActivity :exec
UPDATE sessions
SET "updatedAt" = NOW()
WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions
WHERE "userId" = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE "expiresAt" < NOW();

-- name: CountUserSessions :one
SELECT COUNT(*) FROM sessions
WHERE "userId" = $1;

-- PIN Code queries

-- name: GetUserPinCode :one
SELECT "pinCode" FROM users
WHERE id = $1;

-- name: SetUserPinCode :exec
UPDATE users
SET "pinCode" = $1, "updatedAt" = NOW()
WHERE id = $2;

-- name: ClearUserPinCode :exec
UPDATE users
SET "pinCode" = NULL, "updatedAt" = NOW()
WHERE id = $1;

-- name: HasUserPinCode :one
SELECT EXISTS(
    SELECT 1 FROM users
    WHERE id = $1 AND "pinCode" IS NOT NULL
) AS has_pin_code;

-- Session PIN elevation queries

-- name: SetSessionPinElevation :exec
UPDATE sessions
SET "pinExpiresAt" = $1, "updatedAt" = NOW()
WHERE id = $2;

-- name: ClearSessionPinElevation :exec
UPDATE sessions
SET "pinExpiresAt" = NULL, "updatedAt" = NOW()
WHERE id = $1;

-- name: IsSessionElevated :one
SELECT
    CASE
        WHEN "pinExpiresAt" IS NULL THEN false
        WHEN "pinExpiresAt" > NOW() THEN true
        ELSE false
    END AS is_elevated
FROM sessions
WHERE id = $1;

-- ================== VIEW SERVICE QUERIES ==================

-- name: GetAssetsByOriginalPathPrefix :many
SELECT * FROM assets
WHERE "ownerId" = $1
AND "deletedAt" IS NULL
AND "originalPath" LIKE $2 || '%'
AND (sqlc.narg('is_archived')::boolean IS NULL OR visibility = CASE WHEN sqlc.narg('is_archived')::boolean THEN 'archive' ELSE 'timeline' END)
AND (sqlc.narg('is_favorite')::boolean IS NULL OR "isFavorite" = sqlc.narg('is_favorite'))
ORDER BY "localDateTime" DESC
LIMIT $3 OFFSET $4;

-- name: CountAssetsByOriginalPathPrefix :one
SELECT COUNT(*) FROM assets
WHERE "ownerId" = $1
AND "deletedAt" IS NULL
AND "originalPath" LIKE $2 || '%'
AND (sqlc.narg('is_archived')::boolean IS NULL OR visibility = CASE WHEN sqlc.narg('is_archived')::boolean THEN 'archive' ELSE 'timeline' END)
AND (sqlc.narg('is_favorite')::boolean IS NULL OR "isFavorite" = sqlc.narg('is_favorite'));

-- name: GetUniqueOriginalPathPrefixes :many
SELECT DISTINCT
    COALESCE(
        CASE
            WHEN position('/' IN "originalPath") > 0 THEN
                substring("originalPath" FROM 1 FOR length("originalPath") - position('/' IN reverse("originalPath")) + 1)
            ELSE
                "originalPath"
        END,
        ''
    )::text AS path_prefix
FROM assets
WHERE "ownerId" = $1
AND "deletedAt" IS NULL
ORDER BY path_prefix;

-- ============================================================================
-- ASSET STACK QUERIES
-- ============================================================================

-- name: CreateStack :one
INSERT INTO asset_stack (id, "primaryAssetId", "ownerId")
VALUES (gen_uuid_v7(), $1, $2)
RETURNING *;

-- name: GetStack :one
SELECT * FROM asset_stack
WHERE id = $1;

-- name: GetStackByPrimaryAsset :one
SELECT * FROM asset_stack
WHERE "primaryAssetId" = $1;

-- name: GetStackWithAssets :one
SELECT
    s.*,
    COUNT(a.id) as asset_count
FROM asset_stack s
LEFT JOIN assets a ON a."stackId" = s.id AND a."deletedAt" IS NULL
WHERE s.id = $1
GROUP BY s.id;

-- name: GetStackAssets :many
SELECT * FROM assets
WHERE "stackId" = $1 AND "deletedAt" IS NULL
ORDER BY "localDateTime" DESC;

-- name: GetUserStacks :many
SELECT
    s.*,
    COUNT(a.id) as asset_count
FROM asset_stack s
LEFT JOIN assets a ON a."stackId" = s.id AND a."deletedAt" IS NULL
WHERE s."ownerId" = $1
GROUP BY s.id
ORDER BY s.id DESC
LIMIT $2 OFFSET $3;

-- name: CountUserStacks :one
SELECT COUNT(*) FROM asset_stack
WHERE "ownerId" = $1;

-- name: UpdateStackPrimaryAsset :one
UPDATE asset_stack
SET "primaryAssetId" = $2
WHERE id = $1
RETURNING *;

-- name: DeleteStack :exec
DELETE FROM asset_stack
WHERE id = $1;

-- name: DeleteStacksByIds :exec
DELETE FROM asset_stack
WHERE id = ANY($1::uuid[]);

-- name: AddAssetsToStack :exec
UPDATE assets
SET "stackId" = $1,
    "updatedAt" = now(),
    "updateId" = immich_uuid_v7()
WHERE id = ANY($2::uuid[]) AND "deletedAt" IS NULL;

-- name: RemoveAssetsFromStack :exec
UPDATE assets
SET "stackId" = NULL,
    "updatedAt" = now(),
    "updateId" = immich_uuid_v7()
WHERE id = ANY($1::uuid[]) AND "deletedAt" IS NULL;

-- name: ClearStackAssets :exec
UPDATE assets
SET "stackId" = NULL,
    "updatedAt" = now(),
    "updateId" = immich_uuid_v7()
WHERE "stackId" = $1 AND "deletedAt" IS NULL;

-- name: SearchStacks :many
SELECT
    s.*,
    COUNT(a.id) as asset_count
FROM asset_stack s
LEFT JOIN assets a ON a."stackId" = s.id AND a."deletedAt" IS NULL
WHERE s."ownerId" = $1
AND (sqlc.narg('primary_asset_id')::uuid IS NULL OR s."primaryAssetId" = sqlc.narg('primary_asset_id'))
GROUP BY s.id
ORDER BY s.id DESC
LIMIT $2 OFFSET $3;
