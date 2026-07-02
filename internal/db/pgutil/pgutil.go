package pgutil

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// StringToUUID parses a string into a pgtype.UUID.
func StringToUUID(s string) (pgtype.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

// UUIDToString converts a pgtype.UUID to its string representation.
func UUIDToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

// TimestamptzToTime converts a pgtype.Timestamptz to time.Time.
func TimestamptzToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// TimeToTimestamptz converts a time.Time to pgtype.Timestamptz.
func TimeToTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  t,
		Valid: true,
	}
}

// UUIDToPgtype converts a uuid.UUID to pgtype.UUID.
func UUIDToPgtype(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: u,
		Valid: true,
	}
}

// PgtypeToUUID converts a pgtype.UUID to uuid.UUID.
func PgtypeToUUID(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return p.Bytes
}

// Text wraps a string in a pgtype.Text.
func Text(s string) pgtype.Text {
	return pgtype.Text{
		String: s,
		Valid:  true,
	}
}
