package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetUserIDFromContext(t *testing.T) {
	userID := uuid.New()

	got, err := GetUserIDFromContext(WithClaims(context.Background(), &Claims{UserID: userID.String()}))
	require.NoError(t, err)
	assert.Equal(t, userID, got)

	got, err = GetUserIDFromContext(SetUserIDInContext(context.Background(), userID))
	require.NoError(t, err)
	assert.Equal(t, userID, got)

	_, err = GetUserIDFromContext(context.Background())
	assert.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = GetUserIDFromContext(WithClaims(context.Background(), &Claims{UserID: "not-a-uuid"}))
	assert.Equal(t, codes.Internal, status.Code(err))
}
