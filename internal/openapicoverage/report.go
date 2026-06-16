package openapicoverage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// WriteJSON serializes the report as pretty-printed JSON to `w`.
func WriteJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteMarkdown writes a human-readable markdown report to `w`.
func WriteMarkdown(w io.Writer, r Report) error {
	var b strings.Builder

	fmt.Fprintf(&b, "# OpenAPI vs gRPC-implementation Coverage\n\n")
	fmt.Fprintf(&b, "**Coverage: %.2f%%**  (%d / %d upstream endpoints implemented)\n\n",
		r.CoveragePct, r.Implemented, r.UpstreamTotal)
	fmt.Fprintf(&b, "- Upstream total: %d\n", r.UpstreamTotal)
	fmt.Fprintf(&b, "- Implemented: %d\n", r.Implemented)
	fmt.Fprintf(&b, "- Missing: %d\n", r.Missing)
	fmt.Fprintf(&b, "- Extra (backend-only): %d\n\n", r.ExtraImplemented)

	if len(r.MissingEndpoints) > 0 {
		fmt.Fprintf(&b, "## Missing (upstream not implemented)\n\n")
		fmt.Fprintf(&b, "| Method | Path | Operation ID | Expected gRPC |\n")
		fmt.Fprintf(&b, "|---|---|---|---|\n")
		for _, m := range r.MissingEndpoints {
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n",
				m.Method, m.Path, m.OperationID, m.Expected)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(r.ExtraEndpoints) > 0 {
		fmt.Fprintf(&b, "## Extra (implemented but not in upstream)\n\n")
		fmt.Fprintf(&b, "| Method | Path | gRPC |\n")
		fmt.Fprintf(&b, "|---|---|---|\n")
		for _, e := range r.ExtraEndpoints {
			fmt.Fprintf(&b, "| %s | `%s` | `%s` |\n",
				e.Method, e.Path, e.RPC)
		}
		fmt.Fprintf(&b, "\n")
	}

	if len(r.ByTag) > 0 {
		fmt.Fprintf(&b, "## Coverage by tag\n\n")
		fmt.Fprintf(&b, "| Tag | Implemented / Total |\n")
		fmt.Fprintf(&b, "|---|---|\n")
		// Sort tags for stable output.
		tags := make([]string, 0, len(r.ByTag))
		for t := range r.ByTag {
			tags = append(tags, t)
		}
		sort.Strings(tags)
		for _, t := range tags {
			c := r.ByTag[t]
			fmt.Fprintf(&b, "| %s | %d / %d |\n", t, c.Implemented, c.Total)
		}
		fmt.Fprintf(&b, "\n")
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// WriteJSONFile is a convenience wrapper that opens `path` and writes
// the JSON report there. An empty `path` is a no-op.
func WriteJSONFile(path string, r Report) error {
	if path == "" {
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return WriteJSON(f, r)
}

// WriteMarkdownFile is a convenience wrapper that opens `path` and
// writes the markdown report there. An empty `path` is a no-op.
func WriteMarkdownFile(path string, r Report) error {
	if path == "" {
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return WriteMarkdown(f, r)
}
