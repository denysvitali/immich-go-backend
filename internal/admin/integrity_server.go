package admin

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/grpcutil"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const integrityReportCSVHeader = "id,type,path\n"

var integrityReportTypes = map[string]struct{}{
	integrityTypeChecksumMismatch: {},
	integrityTypeMissingFile:      {},
	integrityTypeUntrackedFile:    {},
}

func validIntegrityReportType(reportType string) bool {
	_, ok := integrityReportTypes[reportType]
	return ok
}

func requireIntegrityReportType(reportType string) error {
	if validIntegrityReportType(reportType) {
		return nil
	}
	return status.Error(codes.InvalidArgument, "invalid integrity report type")
}

func requireIntegrityReportItemID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return status.Error(codes.InvalidArgument, "invalid integrity report item ID")
	}
	return nil
}

func requireIntegrityAdmin(ctx context.Context) error {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return status.Error(auth.MapAuthErrorToGRPC(err), "admin privileges required")
	}
	return nil
}

// GetIntegrityReport returns integrity-report items for the requested report
// type.
func (s *Server) GetIntegrityReport(ctx context.Context, request *immichv1.GetIntegrityReportRequest) (*immichv1.IntegrityReportResponseDto, error) {
	if err := requireIntegrityAdmin(ctx); err != nil {
		return nil, err
	}
	if err := requireIntegrityReportType(request.GetType()); err != nil {
		return nil, err
	}

	report, err := s.integrityReport(ctx)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to build integrity report", err)
	}

	items, nextCursor, err := paginateIntegrityItems(report.itemsByType(request.GetType()), request.GetCursor(), request.GetLimit())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid integrity report cursor")
	}

	return &immichv1.IntegrityReportResponseDto{
		Items:      protoIntegrityItems(items),
		NextCursor: nextCursor,
	}, nil
}

// DeleteIntegrityReport deletes a flagged report item. Report items are derived
// from the current scan and are not persisted, so this remains non-destructive
// until explicit repair/delete semantics exist.
func (s *Server) DeleteIntegrityReport(ctx context.Context, request *immichv1.DeleteIntegrityReportRequest) (*emptypb.Empty, error) {
	if err := requireIntegrityAdmin(ctx); err != nil {
		return nil, err
	}
	if err := requireIntegrityReportItemID(request.GetId()); err != nil {
		return nil, err
	}

	return nil, status.Error(codes.NotFound, "integrity report item not found")
}

// GetIntegrityReportFile downloads the file for a flagged report item.
func (s *Server) GetIntegrityReportFile(ctx context.Context, request *immichv1.GetIntegrityReportFileRequest) (*immichv1.IntegrityReportFileResponse, error) {
	if err := requireIntegrityAdmin(ctx); err != nil {
		return nil, err
	}
	if err := requireIntegrityReportItemID(request.GetId()); err != nil {
		return nil, err
	}

	report, err := s.integrityReport(ctx)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to build integrity report", err)
	}

	item, ok := report.findItem(request.GetId())
	if !ok || item.Type == integrityTypeMissingFile || s.service == nil || s.service.storage == nil {
		return nil, status.Error(codes.NotFound, "integrity report item not found")
	}

	reader, err := s.service.storage.Download(ctx, item.Path)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to download integrity report file", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to read integrity report file", err)
	}

	return &immichv1.IntegrityReportFileResponse{
		Data:        data,
		ContentType: "application/octet-stream",
		Filename:    filepath.Base(item.Path),
	}, nil
}

// GetIntegrityReportCsv exports the requested report as CSV.
func (s *Server) GetIntegrityReportCsv(ctx context.Context, request *immichv1.GetIntegrityReportCsvRequest) (*immichv1.IntegrityReportFileResponse, error) {
	if err := requireIntegrityAdmin(ctx); err != nil {
		return nil, err
	}
	reportType := request.GetType()
	if err := requireIntegrityReportType(reportType); err != nil {
		return nil, err
	}

	report, err := s.integrityReport(ctx)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to build integrity report", err)
	}

	return &immichv1.IntegrityReportFileResponse{
		Data:        report.csvData(reportType),
		ContentType: "application/octet-stream",
		Filename:    fmt.Sprintf("%s.csv", reportType),
	}, nil
}

// GetIntegrityReportSummary returns counts for every integrity report type.
func (s *Server) GetIntegrityReportSummary(ctx context.Context, _ *emptypb.Empty) (*immichv1.IntegrityReportSummaryResponseDto, error) {
	if err := requireIntegrityAdmin(ctx); err != nil {
		return nil, err
	}

	report, err := s.integrityReport(ctx)
	if err != nil {
		return nil, grpcutil.SanitizedInternal(ctx, "failed to build integrity report", err)
	}

	summary := report.summary()
	return &immichv1.IntegrityReportSummaryResponseDto{
		ChecksumMismatch: summary[integrityTypeChecksumMismatch],
		MissingFile:      summary[integrityTypeMissingFile],
		UntrackedFile:    summary[integrityTypeUntrackedFile],
	}, nil
}

func (s *Server) integrityReport(ctx context.Context) (*integrityReport, error) {
	if s == nil || s.service == nil {
		return newIntegrityReport(nil), nil
	}
	return s.service.buildIntegrityReport(ctx)
}
