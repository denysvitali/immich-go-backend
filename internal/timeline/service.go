package timeline

import (
	"context"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	queries *sqlc.Queries
}

func NewService(queries *sqlc.Queries) *Service {
	return &Service{
		queries: queries,
	}
}

type TimeBucket struct {
	Date      string    `json:"date"`     // YYYY-MM-DD format
	Count     int       `json:"count"`    // Number of assets
	AssetIDs  []string  `json:"assetIds"` // First few asset IDs for preview
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
}

type TimelineOptions struct {
	UserID     string
	PartnerIDs []string
	AlbumID    string
	IsArchived bool
	IsFavorite bool
	StartDate  *time.Time
	EndDate    *time.Time
	TimeBucket string // "day", "month", "year"
	Limit      int
	Offset     int
}

func (s *Service) GetTimeBuckets(ctx context.Context, opts TimelineOptions) ([]*TimeBucket, error) {
	// Parse user ID
	uuid, err := uuid.Parse(opts.UserID)
	if err != nil {
		return nil, err
	}

	userUUID := pgtype.UUID{Bytes: uuid, Valid: true}

	// Determine time bucket interval
	timeBucket := "day"
	if opts.TimeBucket != "" {
		timeBucket = opts.TimeBucket
	}

	// Get timeline buckets from database
	dbBuckets, err := s.queries.GetTimelineBuckets(ctx, sqlc.GetTimelineBucketsParams{
		OwnerId:   userUUID,
		DateTrunc: timeBucket,
	})
	if err != nil {
		return nil, err
	}

	// Convert database results to service layer format
	buckets := make([]*TimeBucket, len(dbBuckets))
	for i, row := range dbBuckets {
		// Parse interval to get actual date
		// For now, we'll create a basic time bucket
		// The interval would need proper parsing to get the actual date
		buckets[i] = &TimeBucket{
			Date:     "", // Would need to parse row.TimeBucket interval
			Count:    int(row.Count),
			AssetIDs: []string{}, // Would need separate query to get asset IDs
			// StartDate and EndDate would be calculated from the interval
		}
	}

	return buckets, nil
}

func (s *Service) GetTimelineAssets(ctx context.Context, opts TimelineOptions) ([]string, error) {
	// Parse user ID
	uid, err := uuid.Parse(opts.UserID)
	if err != nil {
		return nil, err
	}

	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Build query parameters
	params := sqlc.GetUserAssetsParams{
		OwnerId: userUUID,
		Status:  sqlc.NullAssetsStatusEnum{Valid: false}, // All statuses
		Limit:   pgtype.Int4{Int32: int32(opts.Limit), Valid: true},
		Offset:  pgtype.Int4{Int32: int32(opts.Offset), Valid: true},
	}

	// Get assets from database
	assets, err := s.queries.GetUserAssets(ctx, params)
	if err != nil {
		return nil, err
	}

	// Extract asset IDs
	assetIDs := make([]string, len(assets))
	for i, asset := range assets {
		assetIDs[i] = uuid.UUID(asset.ID.Bytes).String()
	}

	return assetIDs, nil
}

func (s *Service) GetMonthlyBuckets(ctx context.Context, userID string, year int) ([]*TimeBucket, error) {
	// Parse user ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}

	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Get monthly buckets from database
	dbBuckets, err := s.queries.GetTimelineBuckets(ctx, sqlc.GetTimelineBucketsParams{
		OwnerId:   userUUID,
		DateTrunc: "month",
	})
	if err != nil {
		return nil, err
	}

	// Convert to service layer format
	buckets := make([]*TimeBucket, len(dbBuckets))
	for i, row := range dbBuckets {
		buckets[i] = &TimeBucket{
			Date:     "", // Would need to parse row.TimeBucket interval
			Count:    int(row.Count),
			AssetIDs: []string{}, // Would need separate query
		}
	}

	return buckets, nil
}

func (s *Service) GetYearlyBuckets(ctx context.Context, userID string) ([]*TimeBucket, error) {
	// Parse user ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}

	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Get yearly buckets from database
	dbBuckets, err := s.queries.GetTimelineBuckets(ctx, sqlc.GetTimelineBucketsParams{
		OwnerId:   userUUID,
		DateTrunc: "year",
	})
	if err != nil {
		return nil, err
	}

	// Convert to service layer format
	buckets := make([]*TimeBucket, len(dbBuckets))
	for i, row := range dbBuckets {
		buckets[i] = &TimeBucket{
			Date:     "", // Would need to parse row.TimeBucket interval
			Count:    int(row.Count),
			AssetIDs: []string{}, // Would need separate query
		}
	}

	return buckets, nil
}

func (s *Service) GetDayDetail(ctx context.Context, userID string, date time.Time) (*TimeBucket, error) {
	// Parse user ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}

	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Get daily buckets to find this specific day
	dbBuckets, err := s.queries.GetTimelineBuckets(ctx, sqlc.GetTimelineBucketsParams{
		OwnerId:   userUUID,
		DateTrunc: "day",
	})
	if err != nil {
		return nil, err
	}

	// For now, create a basic bucket for the requested day
	// In a complete implementation, we'd filter for the specific date
	bucket := &TimeBucket{
		Date:      date.Format("2006-01-02"),
		Count:     0,
		AssetIDs:  []string{},
		StartDate: date.Truncate(24 * time.Hour),
		EndDate:   date.Truncate(24 * time.Hour).Add(24 * time.Hour),
	}

	// Check if we have data for this day
	for _, row := range dbBuckets {
		// Would need to parse row.TimeBucket to match against date
		bucket.Count = int(row.Count)
		break // For now, just use the first result
	}

	return bucket, nil
}

func (s *Service) GetTimelineStats(ctx context.Context, userID string) (map[string]interface{}, error) {
	// Parse user ID
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}

	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	// Get asset statistics from database
	statsRow, err := s.queries.GetAssetStatsByUser(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	// Convert to map format
	stats := map[string]interface{}{
		"images": statsRow.Images,
		"videos": statsRow.Videos,
		"total":  statsRow.Total,
	}

	return stats, nil
}
