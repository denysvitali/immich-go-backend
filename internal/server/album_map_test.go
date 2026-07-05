package server

import (
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

func TestAlbumMapMarkersIDFromPath(t *testing.T) {
	albumID := "00000000-0000-4000-8000-000000000000"

	got, ok := albumMapMarkersIDFromPath("/api/albums/" + albumID + "/map-markers")
	assert.True(t, ok)
	assert.Equal(t, albumID, got)

	_, ok = albumMapMarkersIDFromPath("/api/albums/" + albumID)
	assert.False(t, ok)

	_, ok = albumMapMarkersIDFromPath("/api/albums/" + albumID + "/assets/map-markers")
	assert.False(t, ok)
}

func TestAlbumMapMarkerToProto(t *testing.T) {
	assetID := uuid.MustParse("00000000-0000-4000-8000-000000000001")

	marker := albumMapMarkerToProto(sqlc.GetAlbumMapMarkersRow{
		ID:            pgtype.UUID{Bytes: assetID, Valid: true},
		ExifLatitude:  pgtype.Float8{Float64: 37.7749, Valid: true},
		ExifLongitude: pgtype.Float8{Float64: -122.4194, Valid: true},
		City:          pgtype.Text{String: "San Francisco", Valid: true},
		State:         pgtype.Text{String: "California", Valid: true},
		Country:       pgtype.Text{String: "United States", Valid: true},
	})

	assert.Equal(t, assetID.String(), marker.Id)
	assert.Equal(t, 37.7749, marker.Lat)
	assert.Equal(t, -122.4194, marker.Lon)
	assert.Equal(t, 37.7749, marker.Latitude)
	assert.Equal(t, -122.4194, marker.Longitude)
	assert.Equal(t, "San Francisco", marker.City)
	assert.Equal(t, "California", marker.State)
	assert.Equal(t, "United States", marker.Country)
}
