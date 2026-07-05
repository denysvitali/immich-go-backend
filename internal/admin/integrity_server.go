package admin

import (
	"context"
	"fmt"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

const integrityReportCSVHeader = "id,type,path\n"

var integrityReportTypes = map[string]struct{}{
	"checksum_mismatch": {},
	"missing_file":      {},
	"untracked_file":    {},
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

// GetIntegrityReport returns integrity-report items for the requested report
// type. Integrity scanning is not implemented yet, so every report is empty.
func (s *Server) GetIntegrityReport(ctx context.Context, request *immichv1.GetIntegrityReportRequest) (*immichv1.IntegrityReportResponseDto, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}
	if err := requireIntegrityReportType(request.GetType()); err != nil {
		return nil, err
	}

	return &immichv1.IntegrityReportResponseDto{
		Items: []*immichv1.IntegrityReportItemDto{},
	}, nil
}

// DeleteIntegrityReport deletes a flagged report item. With no persisted
// integrity report items, every well-formed item ID is currently absent.
func (s *Server) DeleteIntegrityReport(ctx context.Context, request *immichv1.DeleteIntegrityReportRequest) (*emptypb.Empty, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}
	if err := requireIntegrityReportItemID(request.GetId()); err != nil {
		return nil, err
	}

	return nil, status.Error(codes.NotFound, "integrity report item not found")
}

// GetIntegrityReportFile downloads the file for a flagged report item. With no
// persisted integrity report items, every well-formed item ID is absent.
func (s *Server) GetIntegrityReportFile(ctx context.Context, request *immichv1.GetIntegrityReportFileRequest) (*immichv1.IntegrityReportFileResponse, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}
	if err := requireIntegrityReportItemID(request.GetId()); err != nil {
		return nil, err
	}

	return nil, status.Error(codes.NotFound, "integrity report item not found")
}

// GetIntegrityReportCsv exports the requested report as CSV. Empty reports
// still return the CSV header row so clients can save a valid file.
func (s *Server) GetIntegrityReportCsv(ctx context.Context, request *immichv1.GetIntegrityReportCsvRequest) (*immichv1.IntegrityReportFileResponse, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}
	reportType := request.GetType()
	if err := requireIntegrityReportType(reportType); err != nil {
		return nil, err
	}

	return &immichv1.IntegrityReportFileResponse{
		Data:        []byte(integrityReportCSVHeader),
		ContentType: "application/octet-stream",
		Filename:    fmt.Sprintf("%s.csv", reportType),
	}, nil
}

// GetIntegrityReportSummary returns counts for every integrity report type.
func (s *Server) GetIntegrityReportSummary(ctx context.Context, _ *emptypb.Empty) (*immichv1.IntegrityReportSummaryResponseDto, error) {
	if _, err := auth.RequireAdmin(ctx); err != nil {
		return nil, status.Error(codes.PermissionDenied, "admin privileges required")
	}

	return &immichv1.IntegrityReportSummaryResponseDto{}, nil
}
