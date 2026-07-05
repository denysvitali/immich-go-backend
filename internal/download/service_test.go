package download

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
)

func TestGenerateArchivePathUsesUUIDFallback(t *testing.T) {
	assetID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	service := &Service{}

	assert.Equal(t, assetID.String()+".jpg", service.generateArchivePath(&sqlc.Asset{
		ID:   pgtype.UUID{Bytes: assetID, Valid: true},
		Type: "IMAGE",
	}))
	assert.Equal(t, assetID.String()+".mp4", service.generateArchivePath(&sqlc.Asset{
		ID:   pgtype.UUID{Bytes: assetID, Valid: true},
		Type: "VIDEO",
	}))
}
