package faces

import (
	"context"
	"testing"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFaceEndpointsRequireAuthentication(t *testing.T) {
	server := &Server{}
	tests := []struct {
		name string
		call func() error
	}{
		{name: "get", call: func() error {
			_, err := server.GetFaces(context.Background(), &immichv1.GetFacesRequest{})
			return err
		}},
		{name: "create", call: func() error {
			_, err := server.CreateFace(context.Background(), &immichv1.CreateFaceRequest{})
			return err
		}},
		{name: "delete", call: func() error {
			_, err := server.DeleteFace(context.Background(), &immichv1.DeleteFaceRequest{})
			return err
		}},
		{name: "reassign", call: func() error {
			_, err := server.ReassignFacesById(context.Background(), &immichv1.ReassignFacesByIdRequest{})
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, codes.Unauthenticated, status.Code(tt.call()))
		})
	}
}

func TestFaceServiceAccessDeniedMapsToPermissionDenied(t *testing.T) {
	assert.Equal(t, codes.PermissionDenied, status.Code(faceServiceError("test", ErrAccessDenied)))
}
