package util

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// OptionalBool converts a *bool to a pgtype.Bool.
// Returns a valid pgtype.Bool when the pointer is non-nil.
func OptionalBool(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

// OptionalText converts a *string to a pgtype.Text.
// Returns a valid pgtype.Text when the pointer is non-nil.
func OptionalText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// OptionalInt8 converts a *int32 to a pgtype.Int8.
// Returns a valid pgtype.Int8 when the pointer is non-nil.
func OptionalInt8(i *int32) pgtype.Int8 {
	if i == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: int64(*i), Valid: true}
}

// OptionalUUID parses a *string into a pgtype.UUID.
// Returns a valid pgtype.UUID when the pointer is non-nil and the string is a valid UUID.
func OptionalUUID(s *string) (pgtype.UUID, error) {
	if s == nil {
		return pgtype.UUID{}, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}
