// Package openapicoverage computes the coverage of upstream OpenAPI endpoints
// against the gRPC gateway routes implemented by the immich-go-backend.
//
// The package is intentionally dependency-free: it relies only on the Go
// standard library so it can be built without external network access.
package openapicoverage

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Route is a single HTTP endpoint described by the OpenAPI spec.
type Route struct {
	// Method is the upper-case HTTP verb (e.g. "GET", "POST").
	Method string
	// Path is the OpenAPI path template, e.g. "/activities/{id}".
	// The leading slash is preserved; a missing leading slash is added.
	Path string
	// OperationID is the OpenAPI operationId, which the upstream Immich
	// project sets to the matching gRPC method name.
	OperationID string
	// Tags is the list of OpenAPI tags declared on the operation. Used for
	// the by-tag breakdown in the report.
	Tags []string
}

// Key returns the canonical key used to join the upstream set with the
// implemented set. It is `METHOD + " " + path-template-normalized-path`.
func (r Route) Key() string {
	return r.Method + " " + NormalizePath(r.Path)
}

// ParseOpenAPI reads an OpenAPI 3.x document from disk and returns the set
// of routes declared under `paths`. Only the HTTP methods relevant to a
// REST API are considered: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS.
//
// The function performs no schema validation; it walks the `paths` object
// and emits a Route for every method key it recognises. Unknown method
// keys (custom verbs) are silently ignored to keep the tool future-proof
// against the upstream adding new entries.
func ParseOpenAPI(path string) ([]Route, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read openapi spec %q: %w", path, err)
	}

	var doc struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse openapi spec %q: %w", path, err)
	}

	allowed := map[string]bool{
		"get": true, "post": true, "put": true,
		"delete": true, "patch": true, "head": true, "options": true,
	}

	var routes []Route
	for path, raw := range doc.Paths {
		var methods map[string]json.RawMessage
		if err := json.Unmarshal(raw, &methods); err != nil {
			// Fall back to a generic object - this should not happen for a
			// well-formed spec.
			continue
		}
		for method, opRaw := range methods {
			if !allowed[method] {
				continue
			}
			var op struct {
				OperationID string   `json:"operationId"`
				Tags        []string `json:"tags"`
			}
			if err := json.Unmarshal(opRaw, &op); err != nil {
				continue
			}
			routes = append(routes, Route{
				Method:      strings.ToUpper(method),
				Path:        ensureLeadingSlash(path),
				OperationID: op.OperationID,
				Tags:        op.Tags,
			})
		}
	}

	return routes, nil
}

func ensureLeadingSlash(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}
