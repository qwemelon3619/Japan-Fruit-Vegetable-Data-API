#!/usr/bin/env bash
set -euo pipefail

API_BASE_URL="${API_BASE_URL:-http://127.0.0.1:8080}"
OUT_DIR="${MONITOR_OUT_DIR:-./data/monitoring/csv}"
SNAPSHOT_FILE="${SNAPSHOT_FILE:-$OUT_DIR/snapshots.csv}"
TMP_METRICS="$(mktemp)"
TMP_RUNS="$(mktemp)"
trap 'rm -f "$TMP_METRICS" "$TMP_RUNS"' EXIT

mkdir -p "$OUT_DIR"

ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

ready_ok=0
if curl -fsS -m 5 "$API_BASE_URL/ready" >/dev/null 2>&1; then
  ready_ok=1
fi

http_total=0
http_5xx=0
db_errors_total=0
p95_le=""
p95_ms=""
ingestion_latest_status="unknown"

if curl -fsS -m 8 "$API_BASE_URL/metrics" >"$TMP_METRICS" 2>/dev/null; then
  http_total="$(awk '/^http_requests_total\{/{s+=$NF} END{printf "%.0f", s+0}' "$TMP_METRICS")"
  http_5xx="$(awk '
    /^http_requests_total\{/ {
      n=split($0, a, "status=\"")
      if (n > 1) {
        split(a[2], b, "\"")
        status=b[1]
        if (status ~ /^5/ || status == "5xx") s+=$NF
      }
    }
    END { printf "%.0f", s+0 }
  ' "$TMP_METRICS")"
  db_errors_total="$(awk '/^db_errors_total\{/{s+=$NF} END{printf "%.0f", s+0}' "$TMP_METRICS")"

  p95_le="$(awk '
    BEGIN {
      orderLen=split("0.005 0.01 0.025 0.05 0.1 0.25 0.5 1 2.5 5 10 +Inf", order, " ")
    }
    /^http_request_duration_seconds_bucket\{/ {
      n=split($0, a, "le=\"")
      if (n > 1) {
        split(a[2], b, "\"")
        le=b[1]
        c[le]+=$NF
      }
    }
    END {
      total=c["+Inf"]+0
      if (total <= 0) { print ""; exit }
      target=total*0.95
      for (i=1; i<=orderLen; i++) {
        le=order[i]
        if ((c[le]+0) >= target) { print le; exit }
      }
      print "+Inf"
    }
  ' "$TMP_METRICS")"

  if [[ -n "$p95_le" ]]; then
    if [[ "$p95_le" == "+Inf" ]]; then
      p95_ms="10000"
    else
      p95_ms="$(awk -v v="$p95_le" 'BEGIN{printf "%.0f", v*1000}')"
    fi
  fi
fi

if curl -fsS -m 8 "$API_BASE_URL/v1/ingestion/runs?limit=1" >"$TMP_RUNS" 2>/dev/null; then
  s="$(sed -n 's/.*"status":"\([^"]*\)".*/\1/p' "$TMP_RUNS" | head -n1)"
  if [[ -n "$s" ]]; then
    ingestion_latest_status="$s"
  fi
fi

if [[ ! -f "$SNAPSHOT_FILE" ]]; then
  echo "timestamp,ready_ok,http_total,http_5xx,db_errors_total,http_p95_ms,ingestion_latest_status" >"$SNAPSHOT_FILE"
fi

echo "$ts,$ready_ok,$http_total,$http_5xx,$db_errors_total,${p95_ms:-},$ingestion_latest_status" >>"$SNAPSHOT_FILE"
echo "snapshot written: $SNAPSHOT_FILE @ $ts"
