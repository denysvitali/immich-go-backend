package storage

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorageHealthCheckReflectsRuntimeAvailability(t *testing.T) {
	root := t.TempDir()
	backend, err := NewLocalBackend(LocalConfig{RootPath: root})
	require.NoError(t, err)
	service := &Service{backend: backend}

	require.NoError(t, service.HealthCheck(context.Background()))
	require.NoError(t, os.Remove(root))

	assert.ErrorContains(t, service.HealthCheck(context.Background()), "open storage root")
}

func TestLocalStorageHealthCheckHonorsCanceledContext(t *testing.T) {
	backend, err := NewLocalBackend(LocalConfig{RootPath: t.TempDir()})
	require.NoError(t, err)
	service := &Service{backend: backend}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.ErrorIs(t, service.HealthCheck(ctx), context.Canceled)
}
