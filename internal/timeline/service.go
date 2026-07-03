package timeline

import (
	"context"
	"fmt"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/pgutil"
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

// Bucket groups assets by a calendar period.
type Bucket struct {
	Date  string
	Count int64
}

// BucketAsset is the data required by the upstream web UI for a timeline
// bucket. It is a flattened/columnar view of the assets.
type BucketAsset struct {
	ID               uuid.UUID
	DeviceAssetId    string
	OwnerId          uuid.UUID
	DeviceId         string
	Type             string
	OriginalPath     string
	OriginalFileName string
	FileCreatedAt    time.Time
	FileModifiedAt   time.Time
	LocalDateTime    time.Time
	IsFavorite       bool
	Duration         *string
	EncodedVideoPath *string
	LivePhotoVideoId *uuid.UUID
	StackId          *uuid.UUID
	IsExternal       bool
	Visibility       string
	Status           string
	Width            *int32
	Height           *int32
	Latitude         *float64
	Longitude        *float64
	City             *string
	Country          *string
	ProjectionType   *string
	Thumbhash        *string
}

// ListOptions selects which assets are included in a timeline view.
type ListOptions struct {
	UserID     string
	Bucket     string // "day", "month", "year"
	Date       string // YYYY-MM-DD, YYYY-MM-01 or YYYY-01-01 depending on bucket
	IsFavorite bool
	IsTrashed  bool
	IsArchived bool
	Limit      int32
}

func (s *Service) GetTimeBuckets(ctx context.Context, opts ListOptions) ([]Bucket, error) {
	userUUID, err := pgutil.StringToUUID(opts.UserID)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.GetTimelineBuckets(ctx, sqlc.GetTimelineBucketsParams{
		OwnerId:   userUUID,
		DateTrunc: opts.Bucket,
		Column3:   opts.IsFavorite,
		Column4:   opts.IsTrashed,
	})
	if err != nil {
		return nil, err
	}

	buckets := make([]Bucket, len(rows))
	for i, row := range rows {
		buckets[i] = Bucket{
			Date:  row.TimeBucket.Time.Format("2006-01-02"),
			Count: row.Count,
		}
	}
	return buckets, nil
}

func (s *Service) GetBucketAssets(ctx context.Context, opts ListOptions) ([]BucketAsset, error) {
	userUUID, err := pgutil.StringToUUID(opts.UserID)
	if err != nil {
		return nil, err
	}

	parsedDate, err := time.Parse("2006-01-02", opts.Date)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: %w", opts.Date, err)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 500
	}

	rows, err := s.queries.GetTimelineBucketAssets(ctx, sqlc.GetTimelineBucketAssetsParams{
		OwnerId: userUUID,
		Column2: opts.Bucket,
		Column3: pgtype.Date{Time: parsedDate, Valid: true},
		Limit:   limit,
		Column5: opts.IsFavorite,
		Column6: opts.IsTrashed,
	})
	if err != nil {
		return nil, err
	}

	assets := make([]BucketAsset, len(rows))
	for i, row := range rows {
		assets[i] = BucketAsset{
			ID:               uuid.UUID(row.ID.Bytes),
			DeviceAssetId:    row.DeviceAssetId,
			OwnerId:          uuid.UUID(row.OwnerId.Bytes),
			DeviceId:         row.DeviceId,
			Type:             row.Type,
			OriginalPath:     row.OriginalPath,
			OriginalFileName: row.OriginalFileName,
			FileCreatedAt:    row.FileCreatedAt.Time,
			FileModifiedAt:   row.FileModifiedAt.Time,
			LocalDateTime:    row.LocalDateTime.Time,
			IsFavorite:       row.IsFavorite,
			IsExternal:       row.IsExternal,
			Visibility:       string(row.Visibility),
			Status:           string(row.Status),
		}
		if row.Duration.Valid {
			d := row.Duration.String
			assets[i].Duration = &d
		}
		if row.EncodedVideoPath.Valid {
			v := row.EncodedVideoPath.String
			assets[i].EncodedVideoPath = &v
		}
		if row.LivePhotoVideoId.Valid {
			id := uuid.UUID(row.LivePhotoVideoId.Bytes)
			assets[i].LivePhotoVideoId = &id
		}
		if row.StackId.Valid {
			id := uuid.UUID(row.StackId.Bytes)
			assets[i].StackId = &id
		}
		if row.ExifImageWidth.Valid {
			w := row.ExifImageWidth.Int32
			assets[i].Width = &w
		}
		if row.ExifImageHeight.Valid {
			h := row.ExifImageHeight.Int32
			assets[i].Height = &h
		}
		if row.Latitude.Valid {
			lat := row.Latitude.Float64
			assets[i].Latitude = &lat
		}
		if row.Longitude.Valid {
			lon := row.Longitude.Float64
			assets[i].Longitude = &lon
		}
		if row.City.Valid {
			c := row.City.String
			assets[i].City = &c
		}
		if row.Country.Valid {
			c := row.Country.String
			assets[i].Country = &c
		}
		if row.ProjectionType.Valid {
			p := row.ProjectionType.String
			assets[i].ProjectionType = &p
		}
		if row.Thumbhash != "" {
			th := row.Thumbhash
			assets[i].Thumbhash = &th
		}
	}

	return assets, nil
}

func (s *Service) GetTimelineStats(ctx context.Context, userID string) (map[string]interface{}, error) {
	userUUID, err := pgutil.ParseUserID(userID)
	if err != nil {
		return nil, err
	}

	statsRow, err := s.queries.GetAssetStatsByUser(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"images": statsRow.Images,
		"videos": statsRow.Videos,
		"total":  statsRow.Total,
	}, nil
}
