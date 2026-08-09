package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestCORSPreflightUsesConfiguredOrigin(t *testing.T) {
	srv := &Server{config: &config.Config{Server: config.ServerConfig{
		CORSAllowedOrigins: []string{"https://photos.example"},
	}}}
	called := false
	handler := srv.corsHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	req := httptest.NewRequest(http.MethodOptions, "/api/assets", nil)
	req.Header.Set("Origin", "https://photos.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "https://photos.example", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	assert.False(t, called)
}

func TestCORSPreflightRejectsUnknownOrigin(t *testing.T) {
	srv := &Server{config: &config.Config{Server: config.ServerConfig{
		CORSAllowedOrigins: []string{"https://photos.example"},
	}}}
	handler := srv.corsHandler(http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodOptions, "/api/assets", nil)
	req.Header.Set("Origin", "https://attacker.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSWildcardDoesNotEnableCredentialedRequests(t *testing.T) {
	srv := &Server{config: &config.Config{Server: config.ServerConfig{
		CORSAllowedOrigins: []string{"*"},
	}}}
	handler := srv.corsHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/server/ping", nil)
	req.Header.Set("Origin", "https://any.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
}
