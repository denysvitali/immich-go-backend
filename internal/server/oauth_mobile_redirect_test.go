package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

func TestMobileOAuthRedirectURL(t *testing.T) {
	tests := []struct {
		name     string
		rawQuery string
		want     string
	}{
		{
			name:     "preserves callback query",
			rawQuery: "code=abc123&state=xyz",
			want:     "app.immich:///oauth-callback?code=abc123&state=xyz",
		},
		{
			name:     "matches upstream empty query behavior",
			rawQuery: "",
			want:     "app.immich:///oauth-callback?",
		},
		{
			name:     "preserves oauth error query",
			rawQuery: "error=access_denied&error_description=user%20cancelled",
			want:     "app.immich:///oauth-callback?error=access_denied&error_description=user%20cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mobileOAuthRedirectURL(tt.rawQuery); got != tt.want {
				t.Fatalf("mobileOAuthRedirectURL(%q) = %q, want %q", tt.rawQuery, got, tt.want)
			}
		})
	}
}

func TestHandleWsRedirectsMobileOAuth(t *testing.T) {
	handler := (&Server{}).handleWs(runtime.NewServeMux())
	req := httptest.NewRequest(http.MethodGet, "/api/oauth/mobile-redirect?code=abc123&state=xyz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}

	wantLocation := "app.immich:///oauth-callback?code=abc123&state=xyz"
	if got := rec.Header().Get("Location"); got != wantLocation {
		t.Fatalf("Location = %q, want %q", got, wantLocation)
	}
}
