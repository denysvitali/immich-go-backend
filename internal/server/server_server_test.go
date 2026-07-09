package server

import (
	"context"
	"testing"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/require"
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
