-- name: GetAlbum :one
SELECT * FROM albums
WHERE id = $1;

-- name: GetAlbums :many
SELECT * FROM albums;
