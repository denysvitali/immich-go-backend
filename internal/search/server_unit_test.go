package search

import (
	"testing"
	"time"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMetadataSearchRequestFromFilterDefaults(t *testing.T) {
	req := metadataSearchRequestFromFilter("", nil)

	if req.Query != "" {
		t.Fatalf("expected empty query, got %q", req.Query)
	}
	if req.Page != 0 {
		t.Fatalf("expected default page 0, got %d", req.Page)
	}
	if req.Size != 30 {
		t.Fatalf("expected default size 30, got %d", req.Size)
	}
}

func TestMetadataSearchRequestFromFilterQueryPrecedence(t *testing.T) {
	filter := &immichv1.SearchFilter{
		OriginalFileName: stringPtr("IMG_0001.JPG"),
		OriginalPath:     stringPtr("/photos/fallback.jpg"),
	}

	req := metadataSearchRequestFromFilter("beach", filter)
	if req.Query != "beach" {
		t.Fatalf("expected top-level query to win, got %q", req.Query)
	}

	req = metadataSearchRequestFromFilter("", filter)
	if req.Query != "IMG_0001.JPG" {
		t.Fatalf("expected original file name fallback, got %q", req.Query)
	}

	req = metadataSearchRequestFromFilter("", &immichv1.SearchFilter{
		OriginalPath: stringPtr("/photos/fallback.jpg"),
	})
	if req.Query != "/photos/fallback.jpg" {
		t.Fatalf("expected original path fallback, got %q", req.Query)
	}
}

func TestMetadataSearchRequestFromFilterMapsSupportedFields(t *testing.T) {
	takenAfter := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	takenBefore := time.Date(2024, time.February, 3, 4, 5, 6, 0, time.UTC)
	assetType := immichv1.AssetType_ASSET_TYPE_VIDEO
	filter := &immichv1.SearchFilter{
		City:        stringPtr("Zurich"),
		State:       stringPtr("Zurich"),
		Country:     stringPtr("Switzerland"),
		Make:        stringPtr("Nikon"),
		Model:       stringPtr("Zf"),
		LensModel:   stringPtr("Nikkor"),
		LibraryId:   stringPtr("11111111-1111-1111-1111-111111111111"),
		IsFavorite:  boolPtr(true),
		IsArchived:  boolPtr(false),
		IsEncoded:   boolPtr(true),
		IsMotion:    boolPtr(false),
		IsOffline:   boolPtr(true),
		IsExternal:  boolPtr(false),
		Type:        &assetType,
		Size:        int32Ptr(12),
		Page:        int32Ptr(3),
		TakenAfter:  timestamppb.New(takenAfter),
		TakenBefore: timestamppb.New(takenBefore),
	}

	req := metadataSearchRequestFromFilter("trip", filter)

	if req.Query != "trip" || req.Type != "VIDEO" {
		t.Fatalf("unexpected query/type: %q/%q", req.Query, req.Type)
	}
	if req.City != "Zurich" || req.State != "Zurich" || req.Country != "Switzerland" {
		t.Fatalf("unexpected place filters: %q/%q/%q", req.City, req.State, req.Country)
	}
	if req.Make != "Nikon" || req.Model != "Zf" || req.LensModel != "Nikkor" {
		t.Fatalf("unexpected camera filters: %q/%q/%q", req.Make, req.Model, req.LensModel)
	}
	if req.LibraryID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("unexpected library id: %q", req.LibraryID)
	}
	if req.Page != 3 || req.Size != 12 {
		t.Fatalf("unexpected pagination: page=%d size=%d", req.Page, req.Size)
	}
	if !req.TakenAfter.Equal(takenAfter) || !req.TakenBefore.Equal(takenBefore) {
		t.Fatalf("unexpected taken range: %s - %s", req.TakenAfter, req.TakenBefore)
	}
	assertBoolPtr(t, "isFavorite", req.IsFavorite, true)
	assertBoolPtr(t, "isArchived", req.IsArchived, false)
	assertBoolPtr(t, "isEncoded", req.IsEncoded, true)
	assertBoolPtr(t, "isMotion", req.IsMotion, false)
	assertBoolPtr(t, "isOffline", req.IsOffline, true)
	assertBoolPtr(t, "isExternal", req.IsExternal, false)
}

func assertBoolPtr(t *testing.T, name string, got *bool, want bool) {
	t.Helper()
	if got == nil {
		t.Fatalf("expected %s to be set", name)
	}
	if *got != want {
		t.Fatalf("expected %s=%v, got %v", name, want, *got)
	}
}

func stringPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func int32Ptr(value int32) *int32 {
	return &value
}
