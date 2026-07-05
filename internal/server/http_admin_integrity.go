package server

import (
	"fmt"
	"net/http"
	"strings"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func adminIntegrityCSVTypeFromPath(path string) (string, bool) {
	reportType, ok := strings.CutPrefix(path, "/api/admin/integrity/report/")
	if !ok {
		return "", false
	}
	reportType, ok = strings.CutSuffix(reportType, "/csv")
	if !ok || reportType == "" || strings.Contains(reportType, "/") {
		return "", false
	}
	return reportType, true
}

func adminIntegrityFileIDFromPath(path string) (string, bool) {
	itemID, ok := strings.CutPrefix(path, "/api/admin/integrity/report/")
	if !ok {
		return "", false
	}
	itemID, ok = strings.CutSuffix(itemID, "/file")
	if !ok || itemID == "" || strings.Contains(itemID, "/") {
		return "", false
	}
	return itemID, true
}

func (s *Server) handleAdminIntegrityCSV(w http.ResponseWriter, r *http.Request, reportType string) {
	ctx, ok := s.frontendGatewayContext(w, r)
	if !ok {
		return
	}

	resp, err := s.adminServer.GetIntegrityReportCsv(ctx, &immichv1.GetIntegrityReportCsvRequest{Type: reportType})
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}

	writeBinaryResponse(w, resp.GetContentType(), resp.GetFilename(), resp.GetData())
}

func (s *Server) handleAdminIntegrityFile(w http.ResponseWriter, r *http.Request, itemID string) {
	ctx, ok := s.frontendGatewayContext(w, r)
	if !ok {
		return
	}

	resp, err := s.adminServer.GetIntegrityReportFile(ctx, &immichv1.GetIntegrityReportFileRequest{Id: itemID})
	if err != nil {
		writeGRPCErrorJSON(w, r, err)
		return
	}

	writeBinaryResponse(w, resp.GetContentType(), resp.GetFilename(), resp.GetData())
}

func writeBinaryResponse(w http.ResponseWriter, contentType, filename string, data []byte) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	if filename != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
