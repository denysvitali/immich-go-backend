package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsVersionNewer(t *testing.T) {
	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v1.2.3", "v1.2.2", true},
		{"v1.2.3", "v1.2.3", false},
		{"v1.2.3", "v1.2.4", false},
		{"v2.0.0", "v1.99.99", true},
		{"1.10.0", "1.9.9", true},
		{"v1.2.3-rc1", "v1.2.2", true},
		{"v1.2.3", "dev", false},
		{"not-a-version", "v1.2.3", false},
		{"v1.3", "v1.2.9", true},
	}
	for _, tt := range tests {
		if got := isVersionNewer(tt.latest, tt.current); got != tt.want {
			t.Errorf("isVersionNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}

func TestParseVersionParts(t *testing.T) {
	parts, ok := parseVersionParts("v1.2.3")
	if !ok || parts != [3]int{1, 2, 3} {
		t.Errorf("parseVersionParts(v1.2.3) = %v, %v", parts, ok)
	}
	if _, ok := parseVersionParts("dev"); ok {
		t.Error("parseVersionParts(dev) should not be ok")
	}
	parts, ok = parseVersionParts("1.2.3-beta+build")
	if !ok || parts != [3]int{1, 2, 3} {
		t.Errorf("parseVersionParts(1.2.3-beta+build) = %v, %v", parts, ok)
	}
}

func TestFetchLatestRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("unexpected Accept header: %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.99.0","html_url":"https://example.com/rel"}`))
	}))
	defer srv.Close()

	latest, err := fetchLatestRelease(context.Background(), srv.URL, "v1.0.0")
	if err != nil {
		t.Fatalf("fetchLatestRelease: %v", err)
	}
	if latest.TagName != "v1.99.0" || latest.HTMLURL != "https://example.com/rel" {
		t.Errorf("unexpected release: %+v", latest)
	}
}

func TestFetchLatestReleaseErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	if _, err := fetchLatestRelease(context.Background(), srv.URL, "v1.0.0"); err == nil {
		t.Error("expected error on 403 response")
	}

	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer empty.Close()

	if _, err := fetchLatestRelease(context.Background(), empty.URL, "v1.0.0"); err == nil {
		t.Error("expected error on missing tag_name")
	}
}
