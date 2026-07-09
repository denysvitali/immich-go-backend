package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
)

func TestWriteUserAdminJSONEmitsNullableQuotaFields(t *testing.T) {
	w := httptest.NewRecorder()
	marshaler := &runtime.JSONPb{}
	writeUserAdminJSON(w, marshaler, &immichv1.UserAdminResponse{Id: "user-id"})

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Contains(t, body, "quotaSizeInBytes")
	require.Nil(t, body["quotaSizeInBytes"])
	require.Contains(t, body, "quotaUsageInBytes")
	require.Nil(t, body["quotaUsageInBytes"])
}
