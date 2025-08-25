package libraries

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// UUIDToPgtype converts a uuid.UUID to pgtype.UUID
func UUIDToPgtype(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: u,
		Valid: true,
	}
}

// PgtypeToUUID converts a pgtype.UUID to uuid.UUID
func PgtypeToUUID(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return p.Bytes
}

// PgtypeToTime converts a pgtype.Timestamptz to time.Time
func PgtypeToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// TimeToPgtype converts a time.Time to pgtype.Timestamptz
func TimeToPgtype(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  t,
		Valid: !t.IsZero(),
	}
}