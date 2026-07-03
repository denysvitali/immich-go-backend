package sessions

import (
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

func TestSessionFromDB(t *testing.T) {
	sessionID := uuid.New()
	userID := uuid.New()
	createdAt := time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	expiresAt := createdAt.Add(24 * time.Hour)

	got := sessionFromDB(sqlc.Session{
		ID:         pgutil.UUIDToPgtype(sessionID),
		UserId:     pgutil.UUIDToPgtype(userID),
		DeviceType: "mobile",
		DeviceOS:   "iOS",
		Token:      "session-token",
		CreatedAt:  pgutil.TimeToTimestamptz(createdAt),
		UpdatedAt:  pgutil.TimeToTimestamptz(updatedAt),
		ExpiresAt:  pgutil.TimeToTimestamptz(expiresAt),
	})

	assert.Equal(t, sessionID.String(), got.ID)
	assert.Equal(t, userID.String(), got.UserID)
	assert.Equal(t, "mobile", got.DeviceType)
	assert.Equal(t, "iOS", got.DeviceOS)
	assert.Equal(t, "session-token", got.Token)
	assert.Equal(t, createdAt, got.CreatedAt)
	assert.Equal(t, updatedAt, got.UpdatedAt)
	assert.Equal(t, expiresAt, got.ExpiresAt)
}

func TestSessionFromDBHandlesNullValues(t *testing.T) {
	got := sessionFromDB(sqlc.Session{
		DeviceType: "web",
		DeviceOS:   "browser",
		Token:      "token",
		ID:         pgtype.UUID{},
		UserId:     pgtype.UUID{},
		CreatedAt:  pgtype.Timestamptz{},
		UpdatedAt:  pgtype.Timestamptz{},
		ExpiresAt:  pgtype.Timestamptz{},
	})

	assert.Empty(t, got.ID)
	assert.Empty(t, got.UserID)
	assert.True(t, got.CreatedAt.IsZero())
	assert.True(t, got.UpdatedAt.IsZero())
	assert.True(t, got.ExpiresAt.IsZero())
}
