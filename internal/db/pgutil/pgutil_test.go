package pgutil

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringToUUID(t *testing.T) {
	id := uuid.New()

	got, err := StringToUUID(id.String())
	require.NoError(t, err)

	assert.True(t, got.Valid)
	assert.Equal(t, id, uuid.UUID(got.Bytes))
}

func TestStringToUUIDRejectsInvalidValue(t *testing.T) {
	got, err := StringToUUID("not-a-uuid")

	assert.Error(t, err)
	assert.False(t, got.Valid)
}

func TestUUIDConversions(t *testing.T) {
	id := uuid.New()
	pgID := UUIDToPgtype(id)

	assert.True(t, pgID.Valid)
	assert.Equal(t, id.String(), UUIDToString(pgID))
	assert.Equal(t, id, PgtypeToUUID(pgID))
	assert.Empty(t, UUIDToString(pgtype.UUID{}))
	assert.Equal(t, uuid.Nil, PgtypeToUUID(pgtype.UUID{}))
}

func TestTimestamptzConversions(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	pgTime := TimeToTimestamptz(now)

	assert.True(t, pgTime.Valid)
	assert.Equal(t, now, pgTime.Time)
	assert.Equal(t, now, TimestamptzToTime(pgTime))
	assert.True(t, TimestamptzToTime(pgtype.Timestamptz{}).IsZero())
}

func TestText(t *testing.T) {
	got := Text("hello")

	assert.True(t, got.Valid)
	assert.Equal(t, "hello", got.String)
}
