package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/webui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandlerReportsCriticalAndDegradedDependencies(t *testing.T) {
	healthy := func(context.Context) error { return nil }
	unhealthy := func(context.Context) error { return errors.New("unavailable") }

	tests := []struct {
		name         string
		dependencies healthDependencies
		wantCode     int
		want         healthResponse
	}{
		{
			name: "healthy with jobs disabled",
			dependencies: healthDependencies{
				database: healthy,
				storage:  healthy,
			},
			wantCode: http.StatusOK,
			want: healthResponse{Status: "healthy", Checks: healthCheckResults{
				Database: "healthy", Storage: "healthy", Jobs: "disabled",
			}},
		},
		{
			name: "configured jobs unavailable",
			dependencies: healthDependencies{
				database:       healthy,
				storage:        healthy,
				jobsConfigured: true,
			},
			wantCode: http.StatusOK,
			want: healthResponse{Status: "degraded", Checks: healthCheckResults{
				Database: "healthy", Storage: "healthy", Jobs: "degraded",
			}},
		},
		{
			name: "storage unavailable",
			dependencies: healthDependencies{
				database:       healthy,
				storage:        unhealthy,
				jobs:           healthy,
				jobsConfigured: true,
			},
			wantCode: http.StatusServiceUnavailable,
			want: healthResponse{Status: "unhealthy", Checks: healthCheckResults{
				Database: "healthy", Storage: "unhealthy", Jobs: "healthy",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			newHealthHandler(tt.dependencies).ServeHTTP(
				recorder,
				httptest.NewRequest(http.MethodGet, "/ready", nil),
			)

			assert.Equal(t, tt.wantCode, recorder.Code)
			assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			var got healthResponse
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHealthChecksRunIndependently(t *testing.T) {
	storageChecked := make(chan struct{})
	slowDatabase := func(ctx context.Context) error {
		<-storageChecked
		<-ctx.Done()
		return ctx.Err()
	}
	healthyStorage := func(context.Context) error {
		close(storageChecked)
		return nil
	}
	requestCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/health", nil).WithContext(requestCtx)
	recorder := httptest.NewRecorder()

	newHealthHandler(healthDependencies{
		database: slowDatabase,
		storage:  healthyStorage,
	}).ServeHTTP(recorder, request)

	var response healthResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Equal(t, "unhealthy", response.Checks.Database)
	assert.Equal(t, "healthy", response.Checks.Storage)
}

func TestOperationalEndpointsArePublicAndConfigurable(t *testing.T) {
	serverConfig := config.ServerConfig{
		MetricsEnabled:     true,
		MetricsPath:        "/internal/metrics",
		HealthCheckEnabled: true,
		HealthCheckPath:    "/internal/ready",
	}
	metrics := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("metric 1\n"))
	})
	health := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "authentication required", http.StatusUnauthorized)
	})
	handler := operationalEndpoints(serverConfig, metrics, health, next)

	for path, body := range map[string]string{
		"/internal/metrics": "metric 1\n",
		"/internal/ready":   `{"status":"healthy"}`,
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, body, recorder.Body.String())
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/assets", nil))
	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestOperationalEndpointsRespectDisabledAndMissingHandlers(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	t.Run("disabled metrics falls through", func(t *testing.T) {
		handler := operationalEndpoints(config.ServerConfig{MetricsPath: "/metrics"}, nil, nil, next)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		assert.Equal(t, http.StatusTeapot, recorder.Code)
	})

	t.Run("enabled metrics without provider is unavailable", func(t *testing.T) {
		handler := operationalEndpoints(config.ServerConfig{MetricsEnabled: true, MetricsPath: "/metrics"}, nil, nil, next)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	})

	t.Run("operational paths reject writes", func(t *testing.T) {
		handler := operationalEndpoints(
			config.ServerConfig{MetricsEnabled: true, MetricsPath: "/metrics"},
			http.NotFoundHandler(), nil, next,
		)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/metrics", nil))
		assert.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
		assert.Equal(t, http.MethodGet, recorder.Header().Get("Allow"))
	})
}

func TestOperationalEndpointsTakePrecedenceOverSPAFallback(t *testing.T) {
	webRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(webRoot, "index.html"), []byte("spa"), 0o600))
	metrics := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("metrics"))
	})
	handler := operationalEndpoints(
		config.ServerConfig{MetricsEnabled: true, MetricsPath: "/metrics"},
		metrics,
		nil,
		webui.Handler(webRoot, http.NotFoundHandler()),
	)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "metrics", recorder.Body.String())
}
