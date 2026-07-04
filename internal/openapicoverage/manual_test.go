package openapicoverage

import "testing"

func TestParseManualRouteDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "manual.go", `package fixture

import "net/http"

func handleWs(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/api/oauth/mobile-redirect" {
		return
	}
	if r.URL.Path == "/api/socket.io/" {
		return
	}
}

func handleFrontendShape(w http.ResponseWriter, r *http.Request) bool {
	switch r.Method {
	case http.MethodGet:
		switch r.URL.Path {
		case "/api/system-config/storage-template-options":
			return true
		case "/":
			return true
		}
		if r.URL.Path == "/api/users/me" {
			return true
		}
	case http.MethodPost:
		switch r.URL.Path {
		case "/api/download/archive":
			return true
		}
	}
	return false
}
`)
	writeFile(t, dir, "manual_test.go", `package fixture

import "net/http"

func ignoredTestRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete && r.URL.Path == "/api/test-only" {
		return
	}
}
`)

	routes, err := ParseManualRouteDir(dir)
	if err != nil {
		t.Fatalf("ParseManualRouteDir: %v", err)
	}

	got := make(map[string]GatewayRoute, len(routes))
	for _, route := range routes {
		got[route.HTTPMethod+" "+route.Path] = route
	}

	want := []string{
		"GET /api/oauth/mobile-redirect",
		"GET /api/system-config/storage-template-options",
		"GET /api/users/me",
		"POST /api/download/archive",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d routes, want %d: %+v", len(got), len(want), routes)
	}
	for _, key := range want {
		route, ok := got[key]
		if !ok {
			t.Fatalf("missing route %s in %+v", key, routes)
		}
		if route.Service != "ManualHTTP" {
			t.Fatalf("%s service = %q, want ManualHTTP", key, route.Service)
		}
	}
}

func TestDiffCountsManualRoute(t *testing.T) {
	upstream := []Route{
		{
			Method:      "GET",
			Path:        "/oauth/mobile-redirect",
			OperationID: "redirectOAuthToMobile",
			Tags:        []string{"OAuth"},
		},
	}
	manual := []GatewayRoute{
		manualRoute("GET", "/api/oauth/mobile-redirect"),
	}

	report := Diff(upstream, manual, nil)
	if report.Implemented != 1 {
		t.Fatalf("Implemented = %d, want 1 (missing=%+v)", report.Implemented, report.MissingEndpoints)
	}
	if report.Missing != 0 {
		t.Fatalf("Missing = %d, want 0", report.Missing)
	}
}
