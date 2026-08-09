package server

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestGetServerVersionParsesBuildVersion(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })
	Version = "v2.7.5-rc.1"

	got, err := (&Server{}).GetServerVersion(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Equal(t, &immichv1.ServerVersionResponse{Major: 2, Minor: 7, Patch: 5}, got)
}

func TestGetServerVersionFallsBackToZeroForDevelopmentBuild(t *testing.T) {
	original := Version
	t.Cleanup(func() { Version = original })
	Version = "dev"

	got, err := (&Server{}).GetServerVersion(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Equal(t, &immichv1.ServerVersionResponse{}, got)
}

func TestGetServerStatisticsRequiresAuthentication(t *testing.T) {
	got, err := (&Server{}).GetServerStatistics(context.Background(), &emptypb.Empty{})

	assert.Nil(t, got)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGetServerStatisticsRequiresAdmin(t *testing.T) {
	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: uuid.NewString()})

	got, err := (&Server{}).GetServerStatistics(ctx, &emptypb.Empty{})

	assert.Nil(t, got)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}
