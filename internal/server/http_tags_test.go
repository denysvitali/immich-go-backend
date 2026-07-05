package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func TestHandleWsRoutesBulkTagAssetsBeforeGatewayTagIDMatch(t *testing.T) {
	tagsServer := &fakeTagsServiceServer{}
	handler := (&Server{tagsService: tagsServer}).handleWs(runtime.NewServeMux())
	req := httptest.NewRequest(http.MethodPut, "/api/tags/assets", strings.NewReader(`{"assetIds":["asset-1"],"tagIds":["tag-1"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, tagsServer.bulkReq)
	assert.Equal(t, []string{"asset-1"}, tagsServer.bulkReq.GetAssetIds())
	assert.Equal(t, []string{"tag-1"}, tagsServer.bulkReq.GetTagIds())
	assert.JSONEq(t, `{"count":1}`, rec.Body.String())
}

type fakeTagsServiceServer struct {
	immichv1.UnimplementedTagsServiceServer
	bulkReq *immichv1.BulkTagAssetsRequest
}

func (f *fakeTagsServiceServer) BulkTagAssets(ctx context.Context, req *immichv1.BulkTagAssetsRequest) (*immichv1.BulkTagAssetsResponse, error) {
	f.bulkReq = req
	return &immichv1.BulkTagAssetsResponse{Count: 1}, nil
}
