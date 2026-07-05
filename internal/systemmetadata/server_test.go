package systemmetadata

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAdminOnlyEndpointsMapAuthErrors(t *testing.T) {
	endpoints := []struct {
		name string
		call func(context.Context) error
	}{
		{
			name: "reverse geocoding state",
			call: func(ctx context.Context) error {
				_, err := (&Server{}).GetReverseGeocodingState(ctx, &immichv1.GetReverseGeocodingStateRequest{})
				return err
			},
		},
		{
			name: "version check state",
			call: func(ctx context.Context) error {
				_, err := (&Server{}).GetVersionCheckState(ctx, &immichv1.GetVersionCheckStateRequest{})
				return err
			},
		},
	}

	contexts := []struct {
		name string
		ctx  context.Context
		code codes.Code
	}{
		{
			name: "unauthenticated",
			ctx:  context.Background(),
			code: codes.Unauthenticated,
		},
		{
			name: "non-admin",
			ctx: auth.WithUser(context.Background(), auth.UserInfo{
				ID:      "user-1",
				Email:   "user@example.com",
				IsAdmin: false,
			}),
			code: codes.PermissionDenied,
		},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			for _, tt := range contexts {
				t.Run(tt.name, func(t *testing.T) {
					err := endpoint.call(tt.ctx)

					require.Error(t, err)
					assert.Equal(t, tt.code, status.Code(err))
					assert.Equal(t, "admin privileges required", status.Convert(err).Message())
				})
			}
		})
	}
}
