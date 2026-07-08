#!/usr/bin/env bash
# Lightweight smoke load against a running immich-go-backend instance.
# Read-only endpoints only (health + public server metadata).
#
# Usage:
#   IMMICH_URL=http://localhost:3001 ./scripts/perf/load-smoke.sh
#   make perf-load
#
# Env:
#   IMMICH_URL     Base URL (default: http://localhost:3001)
#   REQUESTS       Total requests across all endpoints (default: 100)
#   CONCURRENCY    Parallel workers (default: 10)
#   TIMEOUT_SEC    curl max-time per request (default: 5)
#   FAIL_ON_ERROR  If 1 (default), exit 1 when any request fails

set -euo pipefail

IMMICH_URL="${IMMICH_URL:-http://localhost:3001}"
IMMICH_URL="${IMMICH_URL%/}"
REQUESTS="${REQUESTS:-100}"
CONCURRENCY="${CONCURRENCY:-10}"
TIMEOUT_SEC="${TIMEOUT_SEC:-5}"
FAIL_ON_ERROR="${FAIL_ON_ERROR:-1}"

# Read-only endpoints (no auth required for these)
ENDPOINTS=(
  "/health"
  "/api/server/ping"
  "/api/server/version"
  "/api/server/features"
  "/api/server/config"
  "/api/server/media-types"
)

if ! command -v curl >/dev/null 2>&1; then
  echo "error: curl is required" >&2
  exit 1
fi

if ! [[ "$REQUESTS" =~ ^[1-9][0-9]*$ ]] || ! [[ "$CONCURRENCY" =~ ^[1-9][0-9]*$ ]]; then
  echo "error: REQUESTS and CONCURRENCY must be positive integers" >&2
  exit 1
fi

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/immich-perf-XXXXXX")"
trap 'rm -rf "$tmpdir"' EXIT

results_file="$tmpdir/results.tsv"
: >"$results_file"

echo "=== immich-go-backend load smoke ==="
echo "URL:         $IMMICH_URL"
echo "Requests:    $REQUESTS"
echo "Concurrency: $CONCURRENCY"
echo "Timeout:     ${TIMEOUT_SEC}s"
echo "Endpoints:   ${ENDPOINTS[*]}"
echo ""

# Quick reachability probe (fail fast if server down)
probe_code="000"
if probe_out="$(curl -sS -o /dev/null -w '%{http_code}' --max-time "$TIMEOUT_SEC" \
  "${IMMICH_URL}/api/server/ping" 2>/dev/null)"; then
  probe_code="$probe_out"
elif [[ -n "${probe_out:-}" ]]; then
  # curl may still write http_code "000" on connection failure
  probe_code="$probe_out"
fi
if [[ "$probe_code" != "200" ]]; then
  echo "error: server not reachable at ${IMMICH_URL}/api/server/ping (HTTP ${probe_code})" >&2
  echo "hint: start the server (e.g. go run ./cmd serve) or set IMMICH_URL" >&2
  exit 1
fi
echo "probe: /api/server/ping -> HTTP ${probe_code} OK"
echo ""

worker() {
  local id="$1"
  local n="$2"
  local out="$tmpdir/w-${id}.tsv"
  : >"$out"
  local i path url code ttotal raw
  for ((i = 0; i < n; i++)); do
    path="${ENDPOINTS[$(( (id + i) % ${#ENDPOINTS[@]} ))]}"
    url="${IMMICH_URL}${path}"
    raw="$(curl -sS -o /dev/null --max-time "$TIMEOUT_SEC" \
      -w '%{http_code} %{time_total}' "$url" 2>/dev/null || true)"
    if [[ -z "$raw" ]]; then
      raw="000 0"
    fi
    read -r code ttotal <<<"$raw"
    printf '%s\t%s\t%s\n' "$path" "$code" "$ttotal" >>"$out"
  done
}

# Distribute requests across workers
base=$((REQUESTS / CONCURRENCY))
rem=$((REQUESTS % CONCURRENCY))
pids=()
for ((w = 0; w < CONCURRENCY; w++)); do
  n="$base"
  if ((w < rem)); then
    n=$((n + 1))
  fi
  if ((n > 0)); then
    worker "$w" "$n" &
    pids+=("$!")
  fi
done

for pid in "${pids[@]:-}"; do
  wait "$pid" || true
done

# Merge worker results
for f in "$tmpdir"/w-*.tsv; do
  [[ -f "$f" ]] || continue
  cat "$f" >>"$results_file"
done

total="$(wc -l <"$results_file" | tr -d ' ')"
if [[ -z "$total" || "$total" -eq 0 ]]; then
  echo "error: no results recorded" >&2
  exit 1
fi

# Aggregate with awk
awk -F'\t' '
function pct(sorted, n, p,    idx) {
  if (n < 1) return 0
  idx = int((p/100.0) * (n - 1))
  if (idx < 0) idx = 0
  if (idx >= n) idx = n - 1
  return sorted[idx]
}
{
  path = $1; code = $2; t = $3 + 0
  n++
  times[n] = t
  sum_t += t
  if (t > max_t) max_t = t
  if (min_t == "" || t < min_t) min_t = t
  by_path[path]++
  by_code[code]++
  if (code ~ /^2/) ok++
  else fail++
  path_sum[path] += t
  path_n[path]++
  if (code !~ /^2/) path_fail[path]++
}
END {
  print "=== results ==="
  print "total:      " n
  print "ok (2xx):   " ok+0
  print "failed:     " fail+0
  if (n > 0) {
    # sort times for percentiles (simple insertion — n is small for smoke)
    for (i = 1; i <= n; i++) {
      for (j = i; j > 1 && times[j-1] > times[j]; j--) {
        tmp = times[j]; times[j] = times[j-1]; times[j-1] = tmp
      }
    }
    avg = sum_t / n
    p50 = pct(times, n, 50)
    p95 = pct(times, n, 95)
    p99 = pct(times, n, 99)
    printf "latency_ms: min=%.1f avg=%.1f p50=%.1f p95=%.1f p99=%.1f max=%.1f\n", \
      min_t*1000, avg*1000, p50*1000, p95*1000, p99*1000, max_t*1000
  }
  print ""
  print "by HTTP status:"
  for (c in by_code) printf "  %s: %d\n", c, by_code[c]
  print ""
  print "by endpoint:"
  for (p in by_path) {
    avg_p = (path_n[p] > 0) ? (path_sum[p] / path_n[p]) * 1000 : 0
    printf "  %s  n=%d fail=%d avg_ms=%.1f\n", p, path_n[p], path_fail[p]+0, avg_p
  }
  # exit code for FAIL_ON_ERROR handled outside
  if (fail > 0) exit 2
}
' "$results_file"
agg_rc=$?

echo ""
if [[ "$agg_rc" -eq 2 ]]; then
  echo "status: FAIL (some requests non-2xx)"
  if [[ "$FAIL_ON_ERROR" == "1" ]]; then
    exit 1
  fi
  exit 0
fi
echo "status: OK"
exit 0
