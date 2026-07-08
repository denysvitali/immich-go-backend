//go:build bench

package perf

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	benchDBOnce sync.Once
	benchDB     *testdb.TestDB
	benchDBErr  error
)

// setupBenchDB returns a shared test DB (testcontainers) or skips when Docker
// is unavailable. Prefer SKIP_INTEGRATION_TESTS=1 to force-skip in CI.
func setupBenchDB(b *testing.B) *testdb.TestDB {
	b.Helper()

	if os.Getenv("SKIP_INTEGRATION_TESTS") != "" {
		b.Skip("SKIP_INTEGRATION_TESTS is set")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		b.Skip("docker not installed; skipping DB benchmarks")
	}

	benchDBOnce.Do(func() {
		benchDB, benchDBErr = testdb.SetupSharedTestDB()
	})
	if benchDBErr != nil {
		b.Skipf("postgres/testcontainers unavailable: %v", benchDBErr)
	}
	if benchDB == nil {
		b.Skip("postgres test DB not initialized")
	}
	return benchDB
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func nowTS() pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
}

func seedBenchUser(b *testing.B, q *sqlc.Queries, email string) pgtype.UUID {
	b.Helper()
	ctx := context.Background()
	id := uuid.New()
	_, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		ID:          pgUUID(id),
		Email:       email,
		Name:        "perf bench user",
		Password:    "hashed-password",
		IsAdmin:     false,
		IsOnboarded: true,
	})
	if err != nil {
		b.Fatalf("CreateUser: %v", err)
	}
	return pgUUID(id)
}

func seedBenchAsset(b *testing.B, q *sqlc.Queries, owner pgtype.UUID, idx int) sqlc.Asset {
	b.Helper()
	ctx := context.Background()
	id := uuid.New()
	sum := sha256.Sum256([]byte(fmt.Sprintf("bench-asset-%s-%d", id, idx)))
	ts := nowTS()
	asset, err := q.CreateAsset(ctx, sqlc.CreateAssetParams{
		DeviceAssetId:    fmt.Sprintf("bench-device-%d-%s", idx, id),
		OwnerId:          owner,
		DeviceId:         "perf-bench-device",
		Type:             "IMAGE",
		OriginalPath:     fmt.Sprintf("uploads/bench/%s/%d.jpg", id, idx),
		FileCreatedAt:    ts,
		FileModifiedAt:   ts,
		LocalDateTime:    ts,
		OriginalFileName: fmt.Sprintf("bench-%d.jpg", idx),
		Checksum:         sum[:],
		IsFavorite:       false,
		Visibility:       sqlc.AssetVisibilityEnumTimeline,
		Status:           sqlc.AssetsStatusEnumActive,
	})
	if err != nil {
		b.Fatalf("CreateAsset: %v", err)
	}
	return asset
}

func BenchmarkDB_GetUserByID(b *testing.B) {
	tdb := setupBenchDB(b)
	q := tdb.Queries
	owner := seedBenchUser(b, q, fmt.Sprintf("bench-getuser-%s@example.com", uuid.NewString()))

	b.ReportAllocs()
	b.ResetTimer()

	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if _, err := q.GetUserByID(ctx, owner); err != nil {
			b.Fatalf("GetUserByID: %v", err)
		}
	}
}

func BenchmarkDB_GetUserByEmail(b *testing.B) {
	tdb := setupBenchDB(b)
	q := tdb.Queries
	email := fmt.Sprintf("bench-email-%s@example.com", uuid.NewString())
	_ = seedBenchUser(b, q, email)

	b.ReportAllocs()
	b.ResetTimer()

	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if _, err := q.GetUserByEmail(ctx, email); err != nil {
			b.Fatalf("GetUserByEmail: %v", err)
		}
	}
}

func BenchmarkDB_GetAssetByID(b *testing.B) {
	tdb := setupBenchDB(b)
	q := tdb.Queries
	owner := seedBenchUser(b, q, fmt.Sprintf("bench-asset-%s@example.com", uuid.NewString()))
	asset := seedBenchAsset(b, q, owner, 0)

	b.ReportAllocs()
	b.ResetTimer()

	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if _, err := q.GetAssetByID(ctx, asset.ID); err != nil {
			b.Fatalf("GetAssetByID: %v", err)
		}
	}
}

func BenchmarkDB_CreateAsset(b *testing.B) {
	tdb := setupBenchDB(b)
	q := tdb.Queries
	owner := seedBenchUser(b, q, fmt.Sprintf("bench-create-%s@example.com", uuid.NewString()))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = seedBenchAsset(b, q, owner, i)
	}
}

func BenchmarkDB_CountAssets(b *testing.B) {
	tdb := setupBenchDB(b)
	q := tdb.Queries
	owner := seedBenchUser(b, q, fmt.Sprintf("bench-count-%s@example.com", uuid.NewString()))
	for i := 0; i < 50; i++ {
		_ = seedBenchAsset(b, q, owner, i)
	}

	params := sqlc.CountAssetsParams{
		OwnerId:    owner,
		Type:       pgtype.Text{},
		IsFavorite: pgtype.Bool{},
		IsArchived: pgtype.Bool{},
		IsTrashed:  pgtype.Bool{},
	}

	b.ReportAllocs()
	b.ResetTimer()

	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		n, err := q.CountAssets(ctx, params)
		if err != nil {
			b.Fatalf("CountAssets: %v", err)
		}
		if n < 50 {
			b.Fatalf("CountAssets: got %d, want >= 50", n)
		}
	}
}
