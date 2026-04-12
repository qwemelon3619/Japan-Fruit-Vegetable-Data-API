#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${MONITOR_OUT_DIR:-./data/monitoring/csv}"
SNAPSHOT_FILE="${SNAPSHOT_FILE:-$OUT_DIR/snapshots.csv}"

MAX_5XX_RATE="${MAX_5XX_RATE:-0.02}"
MAX_P95_MS="${MAX_P95_MS:-800}"
READY_FAIL_CONSEC="${READY_FAIL_CONSEC:-3}"
WEBHOOK_URL="${WEBHOOK_URL:-}"

if [[ ! -f "$SNAPSHOT_FILE" ]]; then
  echo "snapshot file not found: $SNAPSHOT_FILE"
  exit 0
fi

line_count="$(wc -l < "$SNAPSHOT_FILE" | tr -d ' ')"
if [[ "$line_count" -lt 3 ]]; then
  echo "not enough samples for alert check"
  exit 0
fi

curr="$(tail -n1 "$SNAPSHOT_FILE")"
prev="$(tail -n2 "$SNAPSHOT_FILE" | head -n1)"

IFS=',' read -r ts0 ready0 total0 x50 db0 p950 run0 <<<"$prev"
IFS=',' read -r ts1 ready1 total1 x51 db1 p951 run1 <<<"$curr"

delta_total=$((total1-total0))
delta_5xx=$((x51-x50))
delta_db_err=$((db1-db0))

if [[ "$delta_total" -gt 0 ]]; then
  rate_5xx="$(awk -v a="$delta_5xx" -v b="$delta_total" 'BEGIN{printf "%.6f", a/b}')"
else
  rate_5xx="0"
fi

ready_consec_fail=0
while IFS=',' read -r _ts r _rest; do
  if [[ "$r" == "0" ]]; then
    ready_consec_fail=$((ready_consec_fail+1))
  else
    break
  fi
done < <(tail -n "$READY_FAIL_CONSEC" "$SNAPSHOT_FILE" | tac)

alerts=()
if awk -v v="$rate_5xx" -v th="$MAX_5XX_RATE" 'BEGIN{exit !(v>th)}'; then
  alerts+=("5xx_rate_high rate=$rate_5xx threshold=$MAX_5XX_RATE")
fi
if [[ -n "$p951" ]] && awk -v v="$p951" -v th="$MAX_P95_MS" 'BEGIN{exit !(v>th)}'; then
  alerts+=("http_p95_high p95_ms=$p951 threshold_ms=$MAX_P95_MS")
fi
if [[ "$ready_consec_fail" -ge "$READY_FAIL_CONSEC" ]]; then
  alerts+=("ready_failed_consecutive count=$ready_consec_fail threshold=$READY_FAIL_CONSEC")
fi
if [[ "$delta_db_err" -gt 0 ]]; then
  alerts+=("db_errors_increased delta=$delta_db_err")
fi
if [[ "$run1" == "failed" || "$run1" == "partial_failed" ]]; then
  alerts+=("ingestion_status_bad status=$run1")
fi

if [[ "${#alerts[@]}" -eq 0 ]]; then
  echo "alert check ok: $ts1"
  exit 0
fi

msg="[monitor-alert] $ts1 | $(IFS=' ; '; echo "${alerts[*]}")"
echo "$msg"

if [[ -n "$WEBHOOK_URL" ]]; then
  esc_msg="$(printf '%s' "$msg" | sed 's/"/\\"/g')"
  curl -sS -m 5 -X POST "$WEBHOOK_URL" \
    -H "Content-Type: application/json" \
    -d "{\"text\":\"$esc_msg\"}" >/dev/null || true
fi
