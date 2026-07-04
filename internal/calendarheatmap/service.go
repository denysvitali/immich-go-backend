package calendarheatmap

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

const (
	dateLayout        = "2006-01-02"
	defaultWindowDays = 363
	typeUpload        = "Upload"
	typeTaken         = "Taken"
)

var ErrInvalidArgument = errors.New("invalid calendar heatmap argument")

type invalidArgumentError struct {
	message string
}

func (e invalidArgumentError) Error() string {
	return e.message
}

func (e invalidArgumentError) Unwrap() error {
	return ErrInvalidArgument
}

func IsInvalidArgument(err error) bool {
	return errors.Is(err, ErrInvalidArgument)
}

func Get(ctx context.Context, queries *sqlc.Queries, ownerID pgtype.UUID, fromValue, toValue, typeValue string) (*immichv1.CalendarHeatmapResponseDto, error) {
	toDate, err := parseOptionalDate(toValue, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	fromDefault := toDate.AddDate(0, 0, -defaultWindowDays)
	fromDate, err := parseOptionalDate(fromValue, fromDefault)
	if err != nil {
		return nil, err
	}

	if fromDate.After(toDate) {
		return nil, invalidArgumentError{message: "from must be before to"}
	}

	heatmapType, err := normalizeType(typeValue)
	if err != nil {
		return nil, err
	}

	rows, err := queries.GetCalendarHeatmap(ctx, sqlc.GetCalendarHeatmapParams{
		OwnerID:     ownerID,
		FromAt:      pgtype.Timestamptz{Time: fromDate, Valid: true},
		ToAt:        pgtype.Timestamptz{Time: toDate.AddDate(0, 0, 1), Valid: true},
		HeatmapType: heatmapType,
	})
	if err != nil {
		return nil, fmt.Errorf("get calendar heatmap: %w", err)
	}

	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		if !row.ActivityDate.Valid {
			continue
		}
		counts[row.ActivityDate.Time.Format(dateLayout)] = row.Count
	}

	series := make([]*immichv1.CalendarHeatmapSeriesItem, 0, int(toDate.Sub(fromDate).Hours()/24)+1)
	var total int64
	for date := fromDate; !date.After(toDate); date = date.AddDate(0, 0, 1) {
		key := date.Format(dateLayout)
		count := counts[key]
		total += count
		series = append(series, &immichv1.CalendarHeatmapSeriesItem{
			Date:  key,
			Count: count,
		})
	}

	return &immichv1.CalendarHeatmapResponseDto{
		From:       fromDate.Format(dateLayout),
		Series:     series,
		To:         toDate.Format(dateLayout),
		TotalCount: total,
	}, nil
}

func parseOptionalDate(value string, fallback time.Time) (time.Time, error) {
	if value == "" {
		return startOfUTCDay(fallback), nil
	}

	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, invalidArgumentError{message: fmt.Sprintf("invalid date %q, expected YYYY-MM-DD", value)}
	}
	return startOfUTCDay(parsed), nil
}

func startOfUTCDay(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func normalizeType(value string) (string, error) {
	if value == "" {
		return typeUpload, nil
	}
	switch value {
	case typeUpload, typeTaken:
		return value, nil
	default:
		return "", invalidArgumentError{message: `type must be "Upload" or "Taken"`}
	}
}
