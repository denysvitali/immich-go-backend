package openapicoverage

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// GatewayRoute is a single HTTP route declared by a generated
// `*.pb.gw.go` file produced by protoc-gen-grpc-gateway.
type GatewayRoute struct {
	// Service is the gRPC service name, e.g. "AssetService".
	Service string
	// Method is the gRPC method name, e.g. "GetAsset".
	Method string
	// HTTPMethod is the upper-case HTTP verb, e.g. "GET".
	HTTPMethod string
	// Path is the HTTP path template, e.g. "/api/assets/{asset_id}".
	Path string
}

// Key returns the canonical key used to join the implemented set with
// the upstream set. It is `HTTPMethod + " " + path-template-normalized-path`.
func (g GatewayRoute) Key() string {
	return g.HTTPMethod + " " + NormalizePath(g.Path)
}

// RPC returns the canonical "Service.Method" string.
func (g GatewayRoute) RPC() string {
	return g.Service + "." + g.Method
}

// Two regexes that together cover what we need from each generated
// gateway file:
//
//   - mux.Handle\(http\.Method(?P<m>Get|Post|Put|Delete|Patch|Head|Options),
//       pattern_(?P<svc>\w+)_(?P<mtd>\w+)_(?P<n>\d+)
//     captures the HTTP verb, the service+method the pattern was generated
//     for, and the per-method index.
//
//   - "/immich\.v1\.(?P<svc>\w+)/(?P<mtd>\w+)".*runtime\.WithHTTPPathPattern\("(?P<path>[^"]+)"\)
//     captures the authoritative (Service, Method, PathTemplate) triple as
//     baked in by protoc-gen-grpc-gateway from the proto's `google.api.http`
//     annotation. The pattern is `(?s)` (dotall) so the .* matches newlines.

var (
	muxHandleRe = regexp.MustCompile(
		`mux\.Handle\(http\.Method(Get|Post|Put|Delete|Patch|Head|Options),\s*pattern_(\w+)_(\w+)_(\d+)`,
	)
	annotateRe = regexp.MustCompile(
		`(?s)"/immich\.v1\.(\w+)/(\w+)"\s*,\s*runtime\.WithHTTPPathPattern\("([^"]+)"\)`,
	)
)

// ParseGatewayDir walks the directory `dir`, parses every `*.pb.gw.go`
// file it finds and returns the union of all gateway routes declared by
// those files.
//
// The `*.pb.gw.go` files are generated from the proto sources by
// `make proto-gen`; they embed each route as a pair of literals:
//
//	pattern_<Service>_<Method>_<n> = runtime.MustPattern(...)
//	mux.Handle(http.Method<Verb>, pattern_<Service>_<Method>_<n>, ...)
//	runtime.AnnotateContext(ctx, mux, req,
//	    "/immich.v1.<Service>/<Method>",
//	    runtime.WithHTTPPathPattern("/api/.../..."))
//
// We pair the `mux.Handle` call (HTTP verb) with the corresponding
// `AnnotateContext` call (path template) by matching on the
// `Service_Method` token. When the same logical route is registered both
// for direct-server and via-proxy (we get two mux.Handle calls per RPC
// in the generated code), the second occurrence is discarded.
func ParseGatewayDir(dir string) ([]GatewayRoute, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.pb.gw.go"))
	if err != nil {
		return nil, fmt.Errorf("glob %q: %w", dir, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no *.pb.gw.go files found in %q", dir)
	}

	// Build a global map of "Service_Method" -> GatewayRoute.
	// The handler registers each logical route twice in the same file
	// (once in RegisterXxxHandlerServer, once in RegisterXxxHandlerClient),
	// with the same HTTP verb and path template. We dedupe by RPC.
	type key = string
	seen := make(map[key]GatewayRoute)

	for _, file := range matches {
		f, err := os.Open(file)
		if err != nil {
			return nil, fmt.Errorf("open %q: %w", file, err)
		}
		routes, _, err := parseGatewayFile(f, file)
		_ = f.Close()
		if err != nil {
			return nil, err
		}
		for _, r := range routes {
			k := r.RPC() + "|" + r.HTTPMethod
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = r
		}
	}

	out := make([]GatewayRoute, 0, len(seen))
	for _, r := range seen {
		out = append(out, r)
	}
	return out, nil
}

// parseGatewayFile reads a single .pb.gw.go file and returns the routes
// it declares, plus any non-fatal warnings (a mux.Handle call whose
// matching AnnotateContext could not be located, for example).
func parseGatewayFile(f *os.File, name string) ([]GatewayRoute, []string, error) {
	scanner := bufio.NewScanner(f)
	// The generated files can have long lines; widen the buffer.
	scanner.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan %q: %w", name, err)
	}
	text := strings.Join(lines, "\n")

	// First, collect every (verb, Service, Method, idx) tuple from mux.Handle.
	type muxInfo struct {
		verb, svc, mtd, idx string
		pos                 int
	}
	var muxes []muxInfo
	for _, m := range muxHandleRe.FindAllStringSubmatchIndex(text, -1) {
		verb := strings.ToUpper(text[m[2]:m[3]])
		svc := text[m[4]:m[5]]
		mtd := text[m[6]:m[7]]
		idx := text[m[8]:m[9]]
		muxes = append(muxes, muxInfo{verb, svc, mtd, idx, m[0]})
	}

	// Now, collect every (Service, Method, path) tuple from AnnotateContext.
	type annotInfo struct {
		svc, mtd, path string
		pos            int
	}
	var annots []annotInfo
	for _, m := range annotateRe.FindAllStringSubmatchIndex(text, -1) {
		svc := text[m[2]:m[3]]
		mtd := text[m[4]:m[5]]
		path := text[m[6]:m[7]]
		annots = append(annots, annotInfo{svc, mtd, path, m[0]})
	}

	var routes []GatewayRoute
	var warnings []string

	// For each mux.Handle, find the next AnnotateContext for the same
	// (Service, Method) that occurs at or after its position. This
	// mirrors the generated source layout: the mux.Handle and the
	// AnnotateContext call sit inside the same closure, in that order.
	for _, mu := range muxes {
		matched := false
		for _, an := range annots {
			if an.svc != mu.svc || an.mtd != mu.mtd {
				continue
			}
			if an.pos < mu.pos {
				continue
			}
			routes = append(routes, GatewayRoute{
				Service:    mu.svc,
				Method:     mu.mtd,
				HTTPMethod: mu.verb,
				Path:       an.path,
			})
			matched = true
			break
		}
		if !matched {
			warnings = append(warnings, fmt.Sprintf(
				"no matching AnnotateContext for %s.%s (idx=%s)",
				mu.svc, mu.mtd, mu.idx,
			))
		}
	}

	return routes, warnings, nil
}

// MustParseIndex is a small helper that turns a numeric literal (as
// captured by parseGatewayFile) into an int. It is exported so that
// tests can assert the index values.
func MustParseIndex(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return n
}
