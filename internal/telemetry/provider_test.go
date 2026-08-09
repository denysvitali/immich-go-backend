package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderExposesPrometheusMetrics(t *testing.T) {
	provider, err := NewProvider(Config{
		ServiceName:    "observability-test",
		ServiceVersion: "test",
		Environment:    "test",
	}, WithPrometheusMetrics(true))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	counter, err := GetMeter().Int64Counter("observability_test_requests")
	require.NoError(t, err)
	counter.Add(context.Background(), 3)

	recorder := httptest.NewRecorder()
	provider.MetricsHandler().ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/metrics", nil),
	)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "observability_test_requests_total")
	assert.Contains(t, recorder.Body.String(), "go_goroutines")
	assert.Contains(t, recorder.Body.String(), "process_cpu_seconds_total")
}

func TestProviderDoesNotExposePrometheusWhenDisabled(t *testing.T) {
	provider, err := NewProvider(Config{
		ServiceName:    "observability-test",
		ServiceVersion: "test",
		Environment:    "test",
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	assert.Nil(t, provider.MetricsHandler())
}

func TestDefaultConfigDoesNotStartRemoteExporters(t *testing.T) {
	config := GetDefaultConfig()

	assert.False(t, config.TracingEnabled)
	assert.False(t, config.MetricsEnabled)
}
