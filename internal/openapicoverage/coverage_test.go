package openapicoverage

import (
	"os"
	"path/filepath"
	"testing"
)

// fixtureSpec is a minimal OpenAPI 3.0 document used to exercise the
// parser. It mirrors the real spec's shape: each path key is a map of
// method -> operation object.
const fixtureSpec = `{
  "openapi": "3.0.0",
  "info": { "title": "fixture", "version": "1.0.0" },
  "paths": {
    "/activities": {
      "get": {
        "operationId": "getActivities",
        "tags": ["Activities"],
        "responses": { "200": { "description": "ok" } }
      },
      "post": {
        "operationId": "createActivity",
        "tags": ["Activities"],
        "responses": { "201": { "description": "created" } }
      }
    },
    "/activities/{id}": {
      "delete": {
        "operationId": "deleteActivity",
        "tags": ["Activities"],
        "responses": { "204": { "description": "no content" } }
      }
    },
    "/albums/{albumId}/assets": {
      "put": {
        "operationId": "addAssetsToAlbum",
        "tags": ["Albums"],
        "responses": { "200": { "description": "ok" } }
      }
    },
    "/server/ping": {
      "get": {
        "operationId": "pingServer",
        "tags": ["Server"],
        "responses": { "200": { "description": "ok" } }
      }
    }
  }
}
`

// fixtureGateway is a synthetic *.pb.gw.go file that declares three
// routes. We strip the package declaration to keep the fixture inline
// in the test source.
const fixtureGateway = `package fixture

import (
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

var pattern_ActivityService_GetActivities_0 = runtime.MustPattern(runtime.NewPattern(1, []int{2, 0, 2, 1}, []string{"api", "activities"}, ""))

func request_ActivityService_GetActivities_0(ctx context.Context, marshaler runtime.Marshaler, client ActivityServiceClient, req *http.Request, pathParams map[string]string) (proto.Message, runtime.ServerMetadata, error) {
	return nil, runtime.ServerMetadata{}, nil
}

func RegisterActivityServiceHandlerServer(ctx context.Context, mux *runtime.ServeMux, server ActivityServiceServer) error {
	mux.Handle(http.MethodGet, pattern_ActivityService_GetActivities_0, func(w http.ResponseWriter, req *http.Request, pathParams map[string]string) {
		annotatedContext, err := runtime.AnnotateIncomingContext(ctx, mux, req, "/immich.v1.ActivityService/GetActivities", runtime.WithHTTPPathPattern("/api/activities"))
		_ = annotatedContext
		_ = err
	})
	mux.Handle(http.MethodPost, pattern_ActivityService_CreateActivity_0, func(w http.ResponseWriter, req *http.Request, pathParams map[string]string) {
		annotatedContext, err := runtime.AnnotateIncomingContext(ctx, mux, req, "/immich.v1.ActivityService/CreateActivity", runtime.WithHTTPPathPattern("/api/activities"))
		_ = annotatedContext
		_ = err
	})
	return nil
}

var pattern_AssetService_GetRandom_0 = runtime.MustPattern(runtime.NewPattern(1, []int{2, 0, 2, 1, 2, 2}, []string{"api", "assets", "random"}, ""))

func RegisterAssetServiceHandlerServer(ctx context.Context, mux *runtime.ServeMux, server AssetServiceServer) error {
	mux.Handle(http.MethodGet, pattern_AssetService_GetRandom_0, func(w http.ResponseWriter, req *http.Request, pathParams map[string]string) {
		annotatedContext, err := runtime.AnnotateIncomingContext(ctx, mux, req, "/immich.v1.AssetService/GetRandom", runtime.WithHTTPPathPattern("/api/assets/random"))
		_ = annotatedContext
		_ = err
	})
	return nil
}
`

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
	return p
}

func TestParseOpenAPI(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "spec.json", fixtureSpec)

	routes, err := ParseOpenAPI(p)
	if err != nil {
		t.Fatalf("ParseOpenAPI: %v", err)
	}
	if got, want := len(routes), 5; got != want {
		t.Fatalf("got %d routes, want %d (routes=%+v)", got, want, routes)
	}

	// Spot-check a few specific entries.
	idx := map[string]Route{}
	for _, r := range routes {
		idx[r.Key()] = r
	}
	if r, ok := idx["GET /activities"]; !ok {
		t.Errorf("missing GET /activities")
	} else if r.OperationID != "getActivities" {
		t.Errorf("GET /activities opId = %q, want getActivities", r.OperationID)
	}
	if r, ok := idx["DELETE /activities/*"]; !ok {
		t.Errorf("missing DELETE /activities/* (normalized)")
	} else if r.OperationID != "deleteActivity" {
		t.Errorf("opId = %q", r.OperationID)
	}
}

func TestParseGatewayDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "activity.pb.gw.go", fixtureGateway)

	// A second, simpler gateway file. This one is its own fully-formed
	// declaration so the parser doesn't have to deal with overlapping
	// identifiers.
	second := `package fixture

import (
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

var pattern_AssetService_GetRandom_0 = runtime.MustPattern(runtime.NewPattern(1, []int{2, 0, 2, 1, 2, 2}, []string{"api", "assets", "random"}, ""))

func RegisterAssetServiceHandlerServer(ctx context.Context, mux *runtime.ServeMux, server AssetServiceServer) error {
	mux.Handle(http.MethodGet, pattern_AssetService_GetRandom_0, func(w http.ResponseWriter, req *http.Request, pathParams map[string]string) {
		annotatedContext, err := runtime.AnnotateIncomingContext(ctx, mux, req, "/immich.v1.AssetService/GetRandom", runtime.WithHTTPPathPattern("/api/assets/random"))
		_ = annotatedContext
		_ = err
	})
	return nil
}
`
	writeFile(t, dir, "asset.pb.gw.go", second)

	routes, err := ParseGatewayDir(dir)
	if err != nil {
		t.Fatalf("ParseGatewayDir: %v", err)
	}
	if got, want := len(routes), 3; got != want {
		t.Fatalf("got %d routes, want %d (routes=%+v)", got, want, routes)
	}

	wantRPCs := map[string]bool{
		"ActivityService.GetActivities":  true,
		"ActivityService.CreateActivity": true,
		"AssetService.GetRandom":         true,
	}
	for _, r := range routes {
		if !wantRPCs[r.RPC()] {
			t.Errorf("unexpected route %+v", r)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/activities", "/activities"},
		{"/api/activities", "/activities"},
		{"/api/", "/"},
		{"/", "/"},
		{"//foo//bar//", "/foo/bar"},
		{"/activities/{id}", "/activities/*"},
		{"/activities/{asset_id}/thumbnail", "/activities/*/thumbnail"},
		{"/api/assets/{assetId}/video/playback", "/assets/*/video/playback"},
	}
	for _, c := range cases {
		got := NormalizePath(c.in)
		if got != c.want {
			t.Errorf("NormalizePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDiff(t *testing.T) {
	upstream := []Route{
		{Method: "GET", Path: "/activities", OperationID: "getActivities", Tags: []string{"Activities"}},
		{Method: "POST", Path: "/activities", OperationID: "createActivity", Tags: []string{"Activities"}},
		{Method: "DELETE", Path: "/activities/{id}", OperationID: "deleteActivity", Tags: []string{"Activities"}},
		{Method: "PUT", Path: "/albums/{albumId}/assets", OperationID: "addAssetsToAlbum", Tags: []string{"Albums"}},
		{Method: "GET", Path: "/server/ping", OperationID: "pingServer", Tags: []string{"Server"}},
	}
	gateway := []GatewayRoute{
		{Service: "ActivityService", Method: "GetActivities", HTTPMethod: "GET", Path: "/api/activities"},
		{Service: "ActivityService", Method: "CreateActivity", HTTPMethod: "POST", Path: "/api/activities"},
		// No DeleteActivity, no AddAssetsToAlbum, no PingServer in the gateway.
		// Extra: a backend-only route.
		{Service: "AssetService", Method: "GetRandom", HTTPMethod: "GET", Path: "/api/assets/random"},
	}

	r := Diff(upstream, gateway, nil)
	if r.UpstreamTotal != 5 {
		t.Errorf("UpstreamTotal = %d, want 5", r.UpstreamTotal)
	}
	if r.Implemented != 2 {
		t.Errorf("Implemented = %d, want 2", r.Implemented)
	}
	if r.Missing != 3 {
		t.Errorf("Missing = %d, want 3", r.Missing)
	}
	if r.ExtraImplemented != 1 {
		t.Errorf("ExtraImplemented = %d, want 1", r.ExtraImplemented)
	}
	if r.CoveragePct != 40.0 {
		t.Errorf("CoveragePct = %v, want 40.0", r.CoveragePct)
	}
	// Missing endpoints should be sorted by path.
	wantMissingPaths := []string{
		"/activities/{id}",
		"/albums/{albumId}/assets",
		"/server/ping",
	}
	for i, w := range wantMissingPaths {
		if r.MissingEndpoints[i].Path != w {
			t.Errorf("MissingEndpoints[%d].Path = %q, want %q",
				i, r.MissingEndpoints[i].Path, w)
		}
	}
	if got := r.MissingEndpoints[2].Expected; got != "ServerService.pingServer" {
		t.Errorf("Expected = %q, want ServerService.pingServer", got)
	}
	if len(r.ExtraEndpoints) != 1 || r.ExtraEndpoints[0].RPC != "AssetService.GetRandom" {
		t.Errorf("unexpected extras: %+v", r.ExtraEndpoints)
	}
}

func TestDiffIgnorePrefix(t *testing.T) {
	upstream := []Route{
		{Method: "GET", Path: "/server/ping", OperationID: "pingServer", Tags: []string{"Server"}},
		{Method: "GET", Path: "/activities", OperationID: "getActivities"},
	}
	gateway := []GatewayRoute{
		{Service: "ActivityService", Method: "GetActivities", HTTPMethod: "GET", Path: "/api/activities"},
	}

	r := Diff(upstream, gateway, []string{"/server"})
	if r.UpstreamTotal != 1 {
		t.Errorf("after ignore /server: UpstreamTotal = %d, want 1", r.UpstreamTotal)
	}
	if r.Implemented != 1 {
		t.Errorf("after ignore /server: Implemented = %d, want 1", r.Implemented)
	}
}
