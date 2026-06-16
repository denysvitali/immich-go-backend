package openapicoverage

import (
	"path"
	"sort"
	"strings"
)

// NormalizePath produces a canonical form of an HTTP path template that
// can be used to compare two paths which were authored under different
// conventions.
//
// The transformation does three things:
//
//  1. Collapse runs of "/" into a single "/".
//  2. Trim trailing "/" (except for the root "/").
//  3. Replace every "{name}" path-parameter with a single "*" placeholder.
//
// The result preserves segment order and count, so two templates that
// differ only in the *names* of their path parameters compare equal
// (`/assets/{assetId}` and `/assets/{asset_id}` both become
// `/assets/*`).
//
// The upstream OpenAPI spec uses paths without the `/api/` prefix
// (e.g. `/activities`), while the generated gRPC gateway uses the
// `/api/` prefix (e.g. `/api/activities`). To bridge that, we strip a
// leading `/api` segment when present, so both sides end up in the
// `/activities` form. This is documented as the project's convention
// (the gRPC `google.api.http` annotation always uses `/api/...` while
// the upstream spec documents the same path without the prefix).
func NormalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	// Collapse multiple slashes.
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	// Trim trailing slash (keep the root).
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimRight(p, "/")
	}
	// Strip a leading /api segment so the upstream spec and the gateway
	// route can be compared like-for-like. The project's google.api.http
	// annotations always use `/api/...` while the upstream spec omits
	// the prefix.
	if p == "/api" || strings.HasPrefix(p, "/api/") {
		p = strings.TrimPrefix(p, "/api")
		if p == "" {
			p = "/"
		}
	}
	// Replace {param} with *.
	// Done with strings.ReplaceAll to avoid pulling in regexp.
	for {
		start := strings.Index(p, "{")
		if start < 0 {
			break
		}
		end := strings.Index(p[start:], "}")
		if end < 0 {
			break
		}
		p = p[:start] + "*" + p[start+end+1:]
	}
	// Final collapse, just in case a * ended up adjacent to a /.
	p = path.Clean(p)
	if p == "." {
		p = "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// MissingEndpoint describes a route declared by the upstream OpenAPI
// spec that has no matching gateway route in the Go backend.
type MissingEndpoint struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	OperationID string   `json:"operation_id,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	// Expected is a best-effort guess at which gRPC Service.Method the
	// upstream endpoint should map to, derived from the path prefix.
	Expected string `json:"expected,omitempty"`
}

// ExtraEndpoint describes a route declared by the gRPC gateway that
// has no corresponding entry in the upstream OpenAPI spec.
type ExtraEndpoint struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	RPC        string `json:"rpc"`
	Service    string `json:"service"`
	GRPCMethod string `json:"grpc_method"`
}

// Report is the final, machine-readable output of the diff.
type Report struct {
	UpstreamTotal    int               `json:"upstream_total"`
	Implemented      int               `json:"implemented"`
	Missing          int               `json:"missing"`
	ExtraImplemented int               `json:"extra_implemented"`
	CoveragePct      float64           `json:"coverage_pct"`
	MissingEndpoints []MissingEndpoint `json:"missing_endpoints"`
	ExtraEndpoints   []ExtraEndpoint   `json:"extra_endpoints"`
	ByTag            map[string]TagCov `json:"by_tag,omitempty"`
}

// TagCov is the per-tag coverage breakdown.
type TagCov struct {
	Total       int `json:"total"`
	Implemented int `json:"implemented"`
}

// Diff computes the diff between an upstream OpenAPI set and an
// implemented gRPC-gateway set. The result is sorted (missing first by
// path, then by method; extra likewise) for stable output.
func Diff(upstream []Route, gateway []GatewayRoute, ignorePrefixes []string) Report {
	upKey := make(map[string]Route, len(upstream))
	gwKey := make(map[string]GatewayRoute, len(gateway))
	tagTotal := make(map[string]int)
	tagImpl := make(map[string]int)

	for _, r := range upstream {
		if shouldIgnore(r.Path, ignorePrefixes) {
			continue
		}
		upKey[r.Key()] = r
		for _, t := range r.Tags {
			tagTotal[t]++
		}
	}
	for _, g := range gateway {
		if shouldIgnore(g.Path, ignorePrefixes) {
			continue
		}
		gwKey[g.Key()] = g
	}

	missing := []MissingEndpoint{}
	extra := []ExtraEndpoint{}
	implemented := 0

	for k, r := range upKey {
		if _, ok := gwKey[k]; ok {
			implemented++
			for _, t := range r.Tags {
				tagImpl[t]++
			}
			continue
		}
		missing = append(missing, MissingEndpoint{
			Method:      r.Method,
			Path:        r.Path,
			OperationID: r.OperationID,
			Tags:        r.Tags,
			Expected:    guessRPC(r),
		})
	}
	for k, g := range gwKey {
		if _, ok := upKey[k]; ok {
			continue
		}
		extra = append(extra, ExtraEndpoint{
			Method:     g.HTTPMethod,
			Path:       g.Path,
			RPC:        g.RPC(),
			Service:    g.Service,
			GRPCMethod: g.Method,
		})
	}

	sort.Slice(missing, func(i, j int) bool {
		if missing[i].Path != missing[j].Path {
			return missing[i].Path < missing[j].Path
		}
		return missing[i].Method < missing[j].Method
	})
	sort.Slice(extra, func(i, j int) bool {
		if extra[i].Path != extra[j].Path {
			return extra[i].Path < extra[j].Path
		}
		return extra[i].Method < extra[j].Method
	})

	total := len(upKey)
	pct := 0.0
	if total > 0 {
		pct = float64(implemented) * 100.0 / float64(total)
	}
	// Round to 2 decimal places.
	pct = float64(int64(pct*100+0.5)) / 100.0

	return Report{
		UpstreamTotal:    total,
		Implemented:      implemented,
		Missing:          len(missing),
		ExtraImplemented: len(extra),
		CoveragePct:      pct,
		MissingEndpoints: missing,
		ExtraEndpoints:   extra,
		ByTag:            tagBreakdown(tagTotal, tagImpl),
	}
}

func shouldIgnore(p string, prefixes []string) bool {
	for _, pre := range prefixes {
		pre = strings.TrimSpace(pre)
		if pre == "" {
			continue
		}
		if strings.HasPrefix(p, pre) {
			return true
		}
	}
	return false
}

// serviceByPrefix maps the leading path segment of an upstream route to
// the gRPC service that almost always owns it. This is best-effort and
// only used for the "expected" hint in the report.
var serviceByPrefix = map[string]string{
	"activities":      "ActivityService",
	"admin":           "AdminService",
	"albums":          "AlbumService",
	"api-keys":        "ApiKeyService",
	"assets":          "AssetService",
	"auth":            "AuthService",
	"download":        "DownloadService",
	"duplicates":      "DuplicateService",
	"faces":           "FaceService",
	"jobs":            "JobService",
	"libraries":       "LibraryService",
	"map":             "MapService",
	"memories":        "MemoryService",
	"notifications":   "NotificationService",
	"oauth":           "OAuthService",
	"partners":        "PartnerService",
	"people":          "PersonService",
	"plugins":         "PluginService",
	"queues":          "QueueService",
	"search":          "SearchService",
	"server":          "ServerService",
	"sessions":        "SessionService",
	"shared-links":    "SharedLinkService",
	"stacks":          "StackService",
	"sync":            "SyncService",
	"system-config":   "SystemConfigService",
	"system-metadata": "SystemMetadataService",
	"tags":            "TagService",
	"timeline":        "TimelineService",
	"trash":           "TrashService",
	"users":           "UserService",
	"view":            "ViewService",
	"workflows":       "WorkflowService",
}

func guessRPC(r Route) string {
	// Use the raw path (not the normalized form) so the prefix detection
	// still works when /api/ has been stripped.
	seg := strings.SplitN(strings.TrimPrefix(r.Path, "/"), "/", 2)
	if len(seg) == 0 {
		return ""
	}
	svc, ok := serviceByPrefix[seg[0]]
	if !ok {
		return ""
	}
	if r.OperationID == "" {
		return svc
	}
	return svc + "." + r.OperationID
}

func tagBreakdown(total, impl map[string]int) map[string]TagCov {
	if len(total) == 0 {
		return nil
	}
	out := make(map[string]TagCov, len(total))
	for tag, n := range total {
		out[tag] = TagCov{Total: n, Implemented: impl[tag]}
	}
	return out
}
