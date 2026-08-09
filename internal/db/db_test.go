package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfiguredPoolAppliesOperationalSettings(t *testing.T) {
	options := Options{
		MaxOpenConns:    20,
		MinIdleConns:    4,
		MaxConnLifetime: 2 * time.Hour,
		ConnectTimeout:  17 * time.Second,
		QueryTimeout:    31 * time.Second,
		LogQueries:      true,
	}

	config, err := configuredPool("postgres://user:pass@localhost:5432/db", options)
	require.NoError(t, err)
	assert.Equal(t, int32(20), config.MaxConns)
	assert.Equal(t, int32(4), config.MinIdleConns)
	assert.Equal(t, 2*time.Hour, config.MaxConnLifetime)
	assert.Equal(t, 17*time.Second, config.ConnConfig.ConnectTimeout)
	assert.NotNil(t, config.AfterConnect)
	assert.IsType(t, dbQueryTracer{}, config.ConnConfig.Tracer)
}

func TestConfiguredPoolRejectsImpossibleLimits(t *testing.T) {
	_, err := configuredPool("postgres://localhost/db", Options{MaxOpenConns: 2, MinIdleConns: 3})
	require.ErrorContains(t, err, "exceeds max open")
}
