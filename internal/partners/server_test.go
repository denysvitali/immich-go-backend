package partners

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

func TestCurrentUserUUIDFromContext(t *testing.T) {
	userID := uuid.New()

	got, err := currentUserUUIDFromContext(auth.WithClaims(context.Background(), &auth.Claims{UserID: userID.String()}))
	require.NoError(t, err)
	assert.Equal(t, pgUUID(userID), got)

	_, err = currentUserUUIDFromContext(context.Background())
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = currentUserUUIDFromContext(auth.WithClaims(context.Background(), &auth.Claims{UserID: "not-a-uuid"}))
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestParseUUIDParam(t *testing.T) {
	partnerID := uuid.New()

	got, err := parseUUIDParam(partnerID.String(), "invalid partner ID")
	require.NoError(t, err)
	assert.Equal(t, pgUUID(partnerID), got)

	_, err = parseUUIDParam("not-a-uuid", "invalid partner ID")
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestPartnerResponseFromRowUsesPartnershipFields(t *testing.T) {
	partnerID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	createdAt := time.Date(2026, 7, 5, 10, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 7, 5, 11, 45, 0, 0, time.UTC)
	row := sqlc.GetPartnersRow{
		ID:                   pgUUID(partnerID),
		Email:                "ada@example.com",
		Name:                 "Ada",
		InTimeline:           true,
		PartnershipCreatedAt: pgtype.Timestamptz{Time: createdAt, Valid: true},
		PartnershipUpdatedAt: pgtype.Timestamptz{Time: updatedAt, Valid: true},
	}

	resp := partnerResponseFromRow(row)
	require.NotNil(t, resp)
	assert.Equal(t, partnerID.String(), resp.GetId())
	assert.Equal(t, partnerID.String(), resp.GetUser().GetId())
	assert.Equal(t, "ada@example.com", resp.GetUser().GetEmail())
	assert.Equal(t, "Ada", resp.GetUser().GetName())
	assert.True(t, resp.GetInTimeline())
	assert.Equal(t, createdAt, resp.GetCreatedAt().AsTime())
	assert.Equal(t, updatedAt, resp.GetUpdatedAt().AsTime())
}

func TestPartnerResponseFromUser(t *testing.T) {
	partnerID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	createdAt := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 7, 5, 12, 5, 0, 0, time.UTC)
	user := sqlc.User{
		ID:    pgUUID(partnerID),
		Email: "grace@example.com",
		Name:  "Grace",
	}

	resp := partnerResponseFromUser(
		user,
		false,
		pgtype.Timestamptz{Time: createdAt, Valid: true},
		pgtype.Timestamptz{Time: updatedAt, Valid: true},
	)
	require.NotNil(t, resp)
	assert.Equal(t, partnerID.String(), resp.GetId())
	assert.Equal(t, "grace@example.com", resp.GetUser().GetEmail())
	assert.False(t, resp.GetInTimeline())
	assert.Equal(t, createdAt, resp.GetCreatedAt().AsTime())
	assert.Equal(t, updatedAt, resp.GetUpdatedAt().AsTime())
}
