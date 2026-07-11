package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHumanReadableBytes(t *testing.T) {
	assert.Equal(t, "512 B", humanReadableBytes(512))
	assert.Equal(t, "1.0 KiB", humanReadableBytes(1024))
	assert.Equal(t, "500.0 GiB", humanReadableBytes(500*1024*1024*1024))
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

func TestHandleFrontendShapeInterceptsVersionCheckStateRoutes(t *testing.T) {
	for _, path := range []string{
		"/system-metadata/version-check-state",
		"/api/system-metadata/version-check-state",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			handled := (&Server{}).handleFrontendShape(rec, req)

			assert.True(t, handled)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
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

func TestNormalizePartnerDirectionQuery(t *testing.T) {
	tests := []struct {
		name string
		input string
		want string
	}{
		{name: "shared by", input: "shared-by", want: "PARTNER_DIRECTION_SHARED_BY"},
		{name: "shared with", input: "shared-with", want: "PARTNER_DIRECTION_SHARED_WITH"},
		{name: "unchanged", input: "PARTNER_DIRECTION_SHARED_WITH", want: "PARTNER_DIRECTION_SHARED_WITH"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/partners?direction="+tt.input, nil)

			normalizePartnerDirectionQuery(req)

			assert.Equal(t, tt.want, req.URL.Query().Get("direction"))
		})
	}
}
