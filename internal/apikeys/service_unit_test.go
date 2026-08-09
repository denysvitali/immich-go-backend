package apikeys

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAPIKeyIsDeterministicAndVerifiable(t *testing.T) {
	service := NewService(nil)

	hash, err := service.HashAPIKey("test-api-key")
	require.NoError(t, err)
	assert.Equal(t, hashAPIKey("test-api-key"), hash)
	assert.True(t, service.VerifyAPIKey("test-api-key", hash))
	assert.False(t, service.VerifyAPIKey("different-api-key", hash))
}
