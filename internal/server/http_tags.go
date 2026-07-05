package server

import (
	"errors"
	"io"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
)

func (s *Server) handleBulkTagAssets(w http.ResponseWriter, r *http.Request, mux *runtime.ServeMux, marshaler runtime.Marshaler) {
	ctx := gatewayIncomingContext(r)
	var req immichv1.BulkTagAssetsRequest
	if err := marshaler.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		runtime.HTTPError(ctx, mux, marshaler, w, r, status.Errorf(codes.InvalidArgument, "%v", err))
		return
	}

	resp, err := s.tagsService.BulkTagAssets(ctx, &req)
	if err != nil {
		runtime.HTTPError(ctx, mux, marshaler, w, r, err)
		return
	}

	writeProtoJSON(w, marshaler, resp)
}
