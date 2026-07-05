package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdminIntegrityCSVTypeFromPath(t *testing.T) {
	got, ok := adminIntegrityCSVTypeFromPath("/api/admin/integrity/report/missing_file/csv")
	assert.True(t, ok)
	assert.Equal(t, "missing_file", got)

	_, ok = adminIntegrityCSVTypeFromPath("/api/admin/integrity/report/missing_file")
	assert.False(t, ok)

	_, ok = adminIntegrityCSVTypeFromPath("/api/admin/integrity/report/missing_file/extra/csv")
	assert.False(t, ok)
}

func TestAdminIntegrityFileIDFromPath(t *testing.T) {
	itemID := "00000000-0000-4000-8000-000000000000"

	got, ok := adminIntegrityFileIDFromPath("/api/admin/integrity/report/" + itemID + "/file")
	assert.True(t, ok)
	assert.Equal(t, itemID, got)

	_, ok = adminIntegrityFileIDFromPath("/api/admin/integrity/report/" + itemID)
	assert.False(t, ok)

	_, ok = adminIntegrityFileIDFromPath("/api/admin/integrity/report/" + itemID + "/extra/file")
	assert.False(t, ok)
}

func TestWriteBinaryResponse(t *testing.T) {
	w := httptest.NewRecorder()

	writeBinaryResponse(w, "application/octet-stream", "missing_file.csv", []byte("id,type,path\n"))

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, `attachment; filename="missing_file.csv"`, resp.Header.Get("Content-Disposition"))
	assert.Equal(t, "id,type,path\n", w.Body.String())
}

func TestAdminIntegritySummaryResponseEncodesNumericCounts(t *testing.T) {
	w := httptest.NewRecorder()

	writeJSON(w, http.StatusOK, adminIntegritySummaryResponse{
		ChecksumMismatch: 1,
		MissingFile:      2,
		UntrackedFile:    3,
	})

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Equal(t, "{\"checksumMismatch\":1,\"missingFile\":2,\"untrackedFile\":3}\n", w.Body.String())
}
