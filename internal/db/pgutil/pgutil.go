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

// ParseUserID parses a string into a pgtype.UUID.
// It is a convenience wrapper around StringToUUID.
func ParseUserID(s string) (pgtype.UUID, error) {
	return StringToUUID(s)
}

// Text wraps a string in a pgtype.Text.
func Text(s string) pgtype.Text {
	return pgtype.Text{
		String: s,
		Valid:  true,
	}
}

// OptionalBool returns a valid pgtype.Bool when b is non-nil, and the zero
// (NULL) value when it is nil. Use it to map an optional request field onto a
// nullable column without an if-statement at every call site.
func OptionalBool(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

// OptionalText returns a valid pgtype.Text when s is non-nil, and the zero
// (NULL) value when it is nil. Note that a non-nil pointer to "" produces a
// valid empty string, not NULL.
func OptionalText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
