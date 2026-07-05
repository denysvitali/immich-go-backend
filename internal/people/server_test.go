package people

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
)

func TestCurrentUserIDFromContext(t *testing.T) {
	userID := uuid.New()

	got, err := currentUserIDFromContext(auth.WithClaims(context.Background(), &auth.Claims{UserID: userID.String()}))
	require.NoError(t, err)
	assert.Equal(t, userID, got)

	_, err = currentUserIDFromContext(context.Background())
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = currentUserIDFromContext(auth.WithClaims(context.Background(), &auth.Claims{UserID: "not-a-uuid"}))
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestBuildPersonResponse(t *testing.T) {
	personID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	updatedAt := time.Date(2026, 7, 5, 12, 30, 0, 0, time.UTC)
	person := sqlc.Person{
		ID:            pgUUID(personID),
		UpdatedAt:     pgtype.Timestamptz{Time: updatedAt, Valid: true},
		Name:          "Ada",
		ThumbnailPath: "thumbs/ada.webp",
		IsHidden:      true,
		BirthDate:     pgtype.Date{Time: time.Date(1815, 12, 10, 0, 0, 0, 0, time.UTC), Valid: true},
	}

	resp := buildPersonResponse(person, 7)
	require.NotNil(t, resp)
	assert.Equal(t, personID.String(), resp.GetId())
	assert.Equal(t, "Ada", resp.GetName())
	assert.Equal(t, "1815-12-10", resp.GetBirthDate())
	assert.Equal(t, "thumbs/ada.webp", resp.GetThumbnailPath())
	assert.Equal(t, int32(7), resp.GetFaces())
	assert.True(t, resp.GetIsHidden())
	assert.Equal(t, updatedAt, resp.GetUpdatedAt().AsTime())
}
