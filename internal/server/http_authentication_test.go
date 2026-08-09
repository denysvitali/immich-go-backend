package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestHTTPHandlerRejectsInvalidAPIKeyBeforeHandWrittenRoute(t *testing.T) {
	srv := &Server{config: &config.Config{}}
	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("x-api-key", "immich_invalid")
	rec := httptest.NewRecorder()

	mustHTTPHandler(t, srv).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHTTPHandlerRejectsMalformedBearerBeforeHandWrittenRoute(t *testing.T) {
	srv := &Server{config: &config.Config{}}
	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req.Header.Set("Authorization", "Basic not-supported")
	rec := httptest.NewRecorder()

	mustHTTPHandler(t, srv).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHTTPHandlerPublicRouteIgnoresStaleCredential(t *testing.T) {
	srv := &Server{config: &config.Config{}}
	req := httptest.NewRequest(http.MethodGet, "/api/server/version", nil)
	req.Header.Set("x-api-key", "immich_expired-or-revoked")
	rec := httptest.NewRecorder()

	mustHTTPHandler(t, srv).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHTTPHandlerUserReadsRequireAuthentication(t *testing.T) {
	srv := &Server{config: &config.Config{}}
	userID := uuid.NewString()

	for _, path := range []string{
		"/api/users/" + userID,
		"/api/users/" + userID + "/profile-image",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			mustHTTPHandler(t, srv).ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}
