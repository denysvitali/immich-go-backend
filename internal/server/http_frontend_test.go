package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleFrontendShape_ServerVersionHistoryReturnsArray(t *testing.T) {
	srv := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/server/version-history", nil)
	rec := httptest.NewRecorder()

	handled := srv.handleFrontendShape(rec, req)

	require.True(t, handled)
	require.Equal(t, http.StatusOK, rec.Code)

	var body []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body, 2)
	assert.Equal(t, "foo-1", body[0]["id"])
	assert.Equal(t, "v1.0.0", body[0]["version"])
}

func TestResponseStatusRecorderCapturesImplicitStatusAndBytes(t *testing.T) {
	rec := httptest.NewRecorder()
	statusRec := &responseStatusRecorder{ResponseWriter: rec}

	n, err := statusRec.Write([]byte("ok"))

	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, http.StatusOK, statusRec.status)
	assert.Equal(t, int64(2), statusRec.bytes)
}

func TestResponseStatusRecorderCapturesExplicitStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	statusRec := &responseStatusRecorder{ResponseWriter: rec}

	statusRec.WriteHeader(http.StatusInternalServerError)

	assert.Equal(t, http.StatusInternalServerError, statusRec.status)
}

func TestPartnerIDFromPath(t *testing.T) {
	tests := []struct {
		path   string
		wantID string
		wantOK bool
	}{
		{path: "/api/partners/00000000-0000-0000-0000-000000000001", wantID: "00000000-0000-0000-0000-000000000001", wantOK: true},
		{path: "/api/partners", wantOK: false},
		{path: "/api/partners/", wantOK: false},
		{path: "/api/partners/one/two", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			gotID, gotOK := partnerIDFromPath(tt.path)
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotID != tt.wantID {
				t.Fatalf("id = %q, want %q", gotID, tt.wantID)
			}
		})
	}
}
