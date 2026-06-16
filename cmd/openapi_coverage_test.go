package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/openapicoverage"
)

// TestNormalizeRoute verifies that NormalizePath produces the same
// canonical key for two paths whose parameter names differ ("{id}" vs
// "{assetId}"). This is the property that lets the upstream OpenAPI
// spec (which uses one naming convention) and the generated gateway
// code (which uses the proto field names) be joined in the diff.
func TestNormalizeRoute(t *testing.T) {
	const (
		upstream   = "/api/albums/{id}/assets"
		gateway    = "/api/albums/{assetId}/assets"
		wantNormal = "/albums/*/assets"
	)

	gotUp := openapicoverage.NormalizePath(upstream)
	gotGw := openapicoverage.NormalizePath(gateway)

	if gotUp != wantNormal {
		t.Errorf("NormalizePath(%q) = %q, want %q", upstream, gotUp, wantNormal)
	}
	if gotGw != wantNormal {
		t.Errorf("NormalizePath(%q) = %q, want %q", gateway, gotGw, wantNormal)
	}
	if gotUp != gotGw {
		t.Errorf(
			"normalized forms differ: upstream=%q, gateway=%q (must be equal for matching)",
			gotUp, gotGw,
		)
	}

	// And the route keys must match too - that's what the diff uses.
	r1 := openapicoverage.Route{Method: "GET", Path: upstream}
	r2 := openapicoverage.GatewayRoute{HTTPMethod: "GET", Path: gateway}
	if r1.Key() != r2.Key() {
		t.Errorf("Route.Key()=%q vs GatewayRoute.Key()=%q (must match)",
			r1.Key(), r2.Key())
	}
}

// TestParseProtoHTTPRoute writes a minimal generated *.pb.gw.go file
// to a temp directory and verifies that ParseGatewayDir extracts the
// (POST, /api/auth/login) route correctly. This pins the parser to
// the protoc-gen-grpc-gateway output format we depend on.
func TestParseProtoHTTPRoute(t *testing.T) {
	dir := t.TempDir()
	// Inline gateway fixture - one route, the bare minimum shape that
	// the regexes in gateway.go need to match. We deliberately avoid
	// pulling in any third-party packages so this stays self-contained.
	const fixture = `package fixture

import (
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
)

var pattern_AuthService_Login_0 = runtime.MustPattern(runtime.NewPattern(1, []int{2, 0, 2, 1}, []string{"api", "auth", "login"}, ""))

func RegisterAuthServiceHandlerServer(ctx context.Context, mux *runtime.ServeMux, server AuthServiceServer) error {
	mux.Handle(http.MethodPost, pattern_AuthService_Login_0, func(w http.ResponseWriter, req *http.Request, pathParams map[string]string) {
		annotatedContext, err := runtime.AnnotateIncomingContext(ctx, mux, req, "/immich.v1.AuthService/Login", runtime.WithHTTPPathPattern("/api/auth/login"))
		_ = annotatedContext
		_ = err
	})
	return nil
}
`
	if err := os.WriteFile(filepath.Join(dir, "auth.pb.gw.go"), []byte(fixture), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	routes, err := openapicoverage.ParseGatewayDir(dir)
	if err != nil {
		t.Fatalf("ParseGatewayDir: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1: %+v", len(routes), routes)
	}

	got := routes[0]
	if got.HTTPMethod != "POST" {
		t.Errorf("HTTPMethod = %q, want POST", got.HTTPMethod)
	}
	if got.Path != "/api/auth/login" {
		t.Errorf("Path = %q, want /api/auth/login", got.Path)
	}
	if got.Service != "AuthService" {
		t.Errorf("Service = %q, want AuthService", got.Service)
	}
	if got.Method != "Login" {
		t.Errorf("Method = %q, want Login", got.Method)
	}
	if got.RPC() != "AuthService.Login" {
		t.Errorf("RPC() = %q, want AuthService.Login", got.RPC())
	}
}

// TestMatchByRoute verifies the matcher pairs a known implemented
// endpoint with its upstream counterpart. We feed in a single upstream
// Route and a single GatewayRoute, both pointing at the same logical
// endpoint but using the two different naming conventions, and check
// that Diff() counts the endpoint as implemented.
func TestMatchByRoute(t *testing.T) {
	upstream := []openapicoverage.Route{
		{
			Method:      "GET",
			Path:        "/api/albums/{id}/assets",
			OperationID: "getAlbumAssets",
			Tags:        []string{"Albums"},
		},
	}
	gateway := []openapicoverage.GatewayRoute{
		{
			Service:    "AlbumService",
			Method:     "GetAlbumAssets",
			HTTPMethod: "GET",
			Path:       "/api/albums/{albumId}/assets",
		},
	}

	r := openapicoverage.Diff(upstream, gateway, nil)
	if r.UpstreamTotal != 1 {
		t.Errorf("UpstreamTotal = %d, want 1", r.UpstreamTotal)
	}
	if r.Implemented != 1 {
		t.Errorf("Implemented = %d, want 1", r.Implemented)
	}
	if r.Missing != 0 {
		t.Errorf("Missing = %d, want 0 (missing=%+v)", r.Missing, r.MissingEndpoints)
	}
	if r.ExtraImplemented != 0 {
		t.Errorf("ExtraImplemented = %d, want 0 (extra=%+v)",
			r.ExtraImplemented, r.ExtraEndpoints)
	}
	if r.CoveragePct != 100.0 {
		t.Errorf("CoveragePct = %v, want 100.0", r.CoveragePct)
	}
	if r.ByTag["Albums"].Implemented != 1 || r.ByTag["Albums"].Total != 1 {
		t.Errorf("ByTag[Albums] = %+v, want {Total:1, Implemented:1}", r.ByTag["Albums"])
	}
}

// TestReportShape feeds a small fake OpenAPI spec (written to a temp
// file) and a small proto-derived GatewayRoute map (constructed in
// memory) into the full pipeline and asserts that the emitted JSON has
// the expected top-level fields. This guards the public schema of the
// report so downstream consumers (CI bots, dashboards) keep working.
func TestReportShape(t *testing.T) {
	dir := t.TempDir()
	const spec = `{
  "openapi": "3.0.0",
  "info": { "title": "fixture", "version": "1.0.0" },
  "paths": {
    "/users/me": {
      "get": {
        "operationId": "getMyUser",
        "tags": ["Users"],
        "responses": { "200": { "description": "ok" } }
      }
    },
    "/users/{id}": {
      "get": {
        "operationId": "getUser",
        "tags": ["Users"],
        "responses": { "200": { "description": "ok" } }
      }
    }
  }
}
`
	specPath := filepath.Join(dir, "spec.json")
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	upstream, err := openapicoverage.ParseOpenAPI(specPath)
	if err != nil {
		t.Fatalf("ParseOpenAPI: %v", err)
	}

	// Only /users/me is "implemented" in the gateway; /users/{id} is
	// missing, and /users/search is a backend-only extra.
	gateway := []openapicoverage.GatewayRoute{
		{
			Service:    "UserService",
			Method:     "GetMyUser",
			HTTPMethod: "GET",
			Path:       "/api/users/me",
		},
		{
			Service:    "UserService",
			Method:     "SearchUsers",
			HTTPMethod: "GET",
			Path:       "/api/users/search",
		},
	}

	report := openapicoverage.Diff(upstream, gateway, nil)

	// Sanity-check the in-memory struct first, so a regression in
	// Diff() doesn't pass via the JSON marshaler.
	if report.UpstreamTotal != 2 {
		t.Errorf("UpstreamTotal = %d, want 2", report.UpstreamTotal)
	}
	if report.Implemented != 1 {
		t.Errorf("Implemented = %d, want 1", report.Implemented)
	}
	if report.Missing != 1 {
		t.Errorf("Missing = %d, want 1", report.Missing)
	}
	if report.ExtraImplemented != 1 {
		t.Errorf("ExtraImplemented = %d, want 1", report.ExtraImplemented)
	}
	if report.CoveragePct != 50.0 {
		t.Errorf("CoveragePct = %v, want 50.0", report.CoveragePct)
	}

	// Now serialize the report via the real writer and parse it back,
	// so we can assert on the JSON shape (this is what CI consumes).
	var buf bytes.Buffer
	if err := openapicoverage.WriteJSON(&buf, report); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("re-parse JSON: %v\n--- raw ---\n%s", err, buf.String())
	}

	// Top-level fields the public schema must expose.
	wantKeys := []string{
		"upstream_total",
		"implemented",
		"missing",
		"extra_implemented",
		"coverage_pct",
		"missing_endpoints",
		"extra_endpoints",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("JSON missing top-level key %q (got keys: %v)", k, keysOf(got))
		}
	}

	// Specific value checks against the in-memory report.
	if n, _ := got["upstream_total"].(float64); int(n) != report.UpstreamTotal {
		t.Errorf("upstream_total = %v, want %d", got["upstream_total"], report.UpstreamTotal)
	}
	if n, _ := got["implemented"].(float64); int(n) != report.Implemented {
		t.Errorf("implemented = %v, want %d", got["implemented"], report.Implemented)
	}
	if n, _ := got["missing"].(float64); int(n) != report.Missing {
		t.Errorf("missing = %v, want %d", got["missing"], report.Missing)
	}
	if n, _ := got["extra_implemented"].(float64); int(n) != report.ExtraImplemented {
		t.Errorf("extra_implemented = %v, want %d", got["extra_implemented"], report.ExtraImplemented)
	}
	if v, _ := got["coverage_pct"].(float64); v != report.CoveragePct {
		t.Errorf("coverage_pct = %v, want %v", got["coverage_pct"], report.CoveragePct)
	}

	// Shape checks on the array fields.
	missingArr, ok := got["missing_endpoints"].([]any)
	if !ok {
		t.Fatalf("missing_endpoints is not an array: %T", got["missing_endpoints"])
	}
	if len(missingArr) != 1 {
		t.Errorf("len(missing_endpoints) = %d, want 1", len(missingArr))
	} else {
		m := missingArr[0].(map[string]any)
		if m["path"] != "/users/{id}" {
			t.Errorf("missing_endpoints[0].path = %v, want /users/{id}", m["path"])
		}
		if m["method"] != "GET" {
			t.Errorf("missing_endpoints[0].method = %v, want GET", m["method"])
		}
		if m["operation_id"] != "getUser" {
			t.Errorf("missing_endpoints[0].operation_id = %v, want getUser", m["operation_id"])
		}
		if !strings.HasPrefix(m["expected"].(string), "UserService.") {
			t.Errorf("missing_endpoints[0].expected = %v, want prefix UserService.", m["expected"])
		}
	}

	extraArr, ok := got["extra_endpoints"].([]any)
	if !ok {
		t.Fatalf("extra_endpoints is not an array: %T", got["extra_endpoints"])
	}
	if len(extraArr) != 1 {
		t.Errorf("len(extra_endpoints) = %d, want 1", len(extraArr))
	} else {
		e := extraArr[0].(map[string]any)
		if e["rpc"] != "UserService.SearchUsers" {
			t.Errorf("extra_endpoints[0].rpc = %v, want UserService.SearchUsers", e["rpc"])
		}
		if e["path"] != "/api/users/search" {
			t.Errorf("extra_endpoints[0].path = %v, want /api/users/search", e["path"])
		}
		if e["service"] != "UserService" {
			t.Errorf("extra_endpoints[0].service = %v, want UserService", e["service"])
		}
		if e["grpc_method"] != "SearchUsers" {
			t.Errorf("extra_endpoints[0].grpc_method = %v, want SearchUsers", e["grpc_method"])
		}
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
