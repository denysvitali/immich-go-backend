-- name: GetAlbum :one
SELECT * FROM albums
WHERE id = $1;
