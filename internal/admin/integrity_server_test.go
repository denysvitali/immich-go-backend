package admin

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func adminContext() context.Context {
	return auth.WithUser(context.Background(), auth.UserInfo{IsAdmin: true})
}

func TestValidIntegrityReportType(t *testing.T) {
	assert.True(t, validIntegrityReportType("untracked_file"))
	assert.True(t, validIntegrityReportType("missing_file"))
	assert.True(t, validIntegrityReportType("checksum_mismatch"))
	assert.False(t, validIntegrityReportType(""))
	assert.False(t, validIntegrityReportType("unknown"))
}

func TestGetIntegrityReportReturnsEmptyReport(t *testing.T) {
	srv := &Server{}

	resp, err := srv.GetIntegrityReport(adminContext(), &immichv1.GetIntegrityReportRequest{
		Type: "missing_file",
	})
	require.NoError(t, err)
	assert.Empty(t, resp.GetItems())
	assert.Nil(t, resp.NextCursor)
}

func TestGetIntegrityReportRejectsInvalidType(t *testing.T) {
	srv := &Server{}

	_, err := srv.GetIntegrityReport(adminContext(), &immichv1.GetIntegrityReportRequest{
		Type: "unknown",
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestGetIntegrityReportSummaryReturnsZeroCounts(t *testing.T) {
	srv := &Server{}

	resp, err := srv.GetIntegrityReportSummary(adminContext(), &emptypb.Empty{})
	require.NoError(t, err)
	assert.EqualValues(t, 0, resp.GetChecksumMismatch())
	assert.EqualValues(t, 0, resp.GetMissingFile())
	assert.EqualValues(t, 0, resp.GetUntrackedFile())
}

func TestGetIntegrityReportSummaryRequiresAuthentication(t *testing.T) {
	srv := &Server{}

	_, err := srv.GetIntegrityReportSummary(context.Background(), &emptypb.Empty{})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGetIntegrityReportCsvReturnsHeader(t *testing.T) {
	srv := &Server{}

	resp, err := srv.GetIntegrityReportCsv(adminContext(), &immichv1.GetIntegrityReportCsvRequest{
		Type: "checksum_mismatch",
	})
	require.NoError(t, err)
	assert.Equal(t, "application/octet-stream", resp.GetContentType())
	assert.Equal(t, "checksum_mismatch.csv", resp.GetFilename())
	assert.Equal(t, integrityReportCSVHeader, string(resp.GetData()))
}

func TestIntegrityReportItemOperationsReturnNotFound(t *testing.T) {
	srv := &Server{}
	itemID := "00000000-0000-4000-8000-000000000000"

	_, err := srv.DeleteIntegrityReport(adminContext(), &immichv1.DeleteIntegrityReportRequest{Id: itemID})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))

	_, err = srv.GetIntegrityReportFile(adminContext(), &immichv1.GetIntegrityReportFileRequest{Id: itemID})
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestIntegrityReportItemOperationsRejectInvalidID(t *testing.T) {
	srv := &Server{}

	_, err := srv.DeleteIntegrityReport(adminContext(), &immichv1.DeleteIntegrityReportRequest{Id: "invalid"})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}
