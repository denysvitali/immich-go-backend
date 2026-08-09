package main

import (
	"net"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListenerBindFailuresAreSynchronous(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	address := listener.Addr().String()

	t.Run("grpc", func(t *testing.T) {
		_, err := startGRPC(nil, address)
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to listen")
	})

	t.Run("http", func(t *testing.T) {
		_, _, err := startHTTP(nil, config.ServerConfig{Address: address})
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to listen")
	})
}

func TestUnexpectedServerStopHandlesNilError(t *testing.T) {
	err := unexpectedServerStop("HTTP", nil)
	assert.EqualError(t, err, "HTTP server stopped unexpectedly")
}
