package admin

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // Immich uses SHA-1 asset checksums; not for crypto.
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/denysvitali/immich-go-backend/internal/storage"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	integrityTypeChecksumMismatch = "checksum_mismatch"
	integrityTypeMissingFile      = "missing_file"
	integrityTypeUntrackedFile    = "untracked_file"

	integrityDefaultLimit = int64(100)
	integrityMaxLimit     = int64(1000)
)

type integrityStorage interface {
	Exists(ctx context.Context, path string) (bool, error)
	Download(ctx context.Context, path string) (io.ReadCloser, error)
	List(ctx context.Context, prefix string, recursive bool) ([]storage.FileInfo, error)
}

type integrityOriginalAsset struct {
	ID       string
	Path     string
	Checksum []byte
}

type integrityReportItem struct {
	ID   string
	Type string
	Path string
}

type integrityReport struct {
	items []integrityReportItem
	byID  map[string]integrityReportItem
}

func (s *Service) buildIntegrityReport(ctx context.Context) (*integrityReport, error) {
	if s == nil || s.db == nil || s.storage == nil {
		return newIntegrityReport(nil), nil
	}

	ctx, span := tracer.Start(ctx, "admin.integrity_report",
		trace.WithAttributes(attribute.String("operation", "integrity_report")))
	defer span.End()

	start := time.Now()
	defer func() {
		attrs := metric.WithAttributes(attribute.String("operation", "integrity_report"))
		if s.operationDuration != nil {
			s.operationDuration.Record(ctx, time.Since(start).Seconds(), attrs)
		}
		if s.operationCounter != nil {
			s.operationCounter.Add(ctx, 1, attrs)
		}
	}()

	dbAssets, err := s.db.GetIntegrityOriginalAssets(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("list integrity original assets: %w", err)
	}

	trackedPaths, err := s.db.GetIntegrityTrackedPaths(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("list integrity tracked paths: %w", err)
	}

	assets := make([]integrityOriginalAsset, 0, len(dbAssets))
	for _, asset := range dbAssets {
		assets = append(assets, integrityOriginalAsset{
			ID:       asset.ID.String(),
			Path:     asset.OriginalPath,
			Checksum: asset.Checksum,
		})
	}

	report, err := buildIntegrityReport(ctx, assets, trackedPaths, s.storage)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	summary := report.summary()
	span.SetAttributes(
		attribute.Int64("integrity.checksum_mismatch", summary[integrityTypeChecksumMismatch]),
		attribute.Int64("integrity.missing_file", summary[integrityTypeMissingFile]),
		attribute.Int64("integrity.untracked_file", summary[integrityTypeUntrackedFile]),
	)

	return report, nil
}

func buildIntegrityReport(ctx context.Context, assets []integrityOriginalAsset, trackedPaths []string, storageSvc integrityStorage) (*integrityReport, error) {
	if storageSvc == nil {
		return newIntegrityReport(nil), nil
	}

	tracked := make(map[string]struct{}, len(trackedPaths)+len(assets))
	for _, path := range trackedPaths {
		addTrackedPath(tracked, path)
	}

	var items []integrityReportItem
	for _, asset := range assets {
		addTrackedPath(tracked, asset.Path)
		if asset.Path == "" {
			continue
		}

		exists, err := storageSvc.Exists(ctx, asset.Path)
		if err != nil {
			return nil, fmt.Errorf("check asset file %q: %w", asset.Path, err)
		}
		if !exists {
			items = append(items, integrityReportItem{
				ID:   asset.ID,
				Type: integrityTypeMissingFile,
				Path: asset.Path,
			})
			continue
		}

		matches, err := assetChecksumMatches(ctx, storageSvc, asset.Path, asset.Checksum)
		if err != nil {
			return nil, fmt.Errorf("checksum asset file %q: %w", asset.Path, err)
		}
		if !matches {
			items = append(items, integrityReportItem{
				ID:   asset.ID,
				Type: integrityTypeChecksumMismatch,
				Path: asset.Path,
			})
		}
	}

	storageFiles, err := storageSvc.List(ctx, "", true)
	if err != nil {
		return nil, fmt.Errorf("list storage files: %w", err)
	}
	for _, file := range storageFiles {
		if file.IsDir {
			continue
		}
		path := strings.TrimSpace(file.Path)
		if path == "" {
			continue
		}
		if isIgnoredIntegrityStoragePath(path) {
			continue
		}
		if _, ok := tracked[path]; ok {
			continue
		}
		items = append(items, integrityReportItem{
			ID:   untrackedIntegrityID(path),
			Type: integrityTypeUntrackedFile,
			Path: path,
		})
	}

	return newIntegrityReport(items), nil
}

func addTrackedPath(paths map[string]struct{}, path string) {
	path = strings.TrimSpace(path)
	if path != "" {
		paths[path] = struct{}{}
	}
}

func isIgnoredIntegrityStoragePath(storagePath string) bool {
	switch path.Base(strings.TrimSpace(storagePath)) {
	case ".immich", ".immich-go-write-check":
		return true
	default:
		return false
	}
}

func newIntegrityReport(items []integrityReportItem) *integrityReport {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			return items[i].Type < items[j].Type
		}
		if items[i].Path != items[j].Path {
			return items[i].Path < items[j].Path
		}
		return items[i].ID < items[j].ID
	})

	byID := make(map[string]integrityReportItem, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}

	return &integrityReport{
		items: items,
		byID:  byID,
	}
}

func (r *integrityReport) summary() map[string]int64 {
	counts := map[string]int64{
		integrityTypeChecksumMismatch: 0,
		integrityTypeMissingFile:      0,
		integrityTypeUntrackedFile:    0,
	}
	if r == nil {
		return counts
	}
	for _, item := range r.items {
		counts[item.Type]++
	}
	return counts
}

func (r *integrityReport) itemsByType(reportType string) []integrityReportItem {
	if r == nil {
		return nil
	}
	var items []integrityReportItem
	for _, item := range r.items {
		if item.Type == reportType {
			items = append(items, item)
		}
	}
	return items
}

func (r *integrityReport) findItem(id string) (integrityReportItem, bool) {
	if r == nil {
		return integrityReportItem{}, false
	}
	item, ok := r.byID[id]
	return item, ok
}

func paginateIntegrityItems(items []integrityReportItem, cursor string, limit int64) ([]integrityReportItem, *string, error) {
	offset := int64(0)
	if cursor != "" {
		parsed, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil || parsed < 0 {
			return nil, nil, fmt.Errorf("invalid cursor")
		}
		offset = parsed
	}

	if limit <= 0 {
		limit = integrityDefaultLimit
	}
	if limit > integrityMaxLimit {
		limit = integrityMaxLimit
	}

	if offset >= int64(len(items)) {
		return nil, nil, nil
	}

	end := offset + limit
	if end > int64(len(items)) {
		end = int64(len(items))
	}

	var next *string
	if end < int64(len(items)) {
		nextCursor := strconv.FormatInt(end, 10)
		next = &nextCursor
	}

	return items[offset:end], next, nil
}

func (r *integrityReport) csvData(reportType string) []byte {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{"id", "type", "path"})
	for _, item := range r.itemsByType(reportType) {
		_ = writer.Write([]string{item.ID, item.Type, item.Path})
	}
	writer.Flush()
	return buf.Bytes()
}

func assetChecksumMatches(ctx context.Context, storageSvc integrityStorage, path string, storedChecksum []byte) (bool, error) {
	if len(storedChecksum) == 0 {
		return true, nil
	}

	reader, err := storageSvc.Download(ctx, path)
	if err != nil {
		return false, err
	}
	defer reader.Close()

	hasher := sha1.New() //nolint:gosec // Immich uses SHA-1 asset checksums; not for crypto.
	if _, err := io.Copy(hasher, reader); err != nil {
		return false, err
	}

	actualHex := hex.EncodeToString(hasher.Sum(nil))
	storedHexText := strings.ToLower(strings.TrimSpace(string(storedChecksum)))
	if storedHexText == actualHex {
		return true, nil
	}

	return hex.EncodeToString(storedChecksum) == actualHex, nil
}

func untrackedIntegrityID(path string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("immich-go-backend:integrity:untracked:"+path)).String()
}

func protoIntegrityItems(items []integrityReportItem) []*immichv1.IntegrityReportItemDto {
	out := make([]*immichv1.IntegrityReportItemDto, 0, len(items))
	for _, item := range items {
		out = append(out, &immichv1.IntegrityReportItemDto{
			Id:   item.ID,
			Type: item.Type,
			Path: item.Path,
		})
	}
	return out
}
