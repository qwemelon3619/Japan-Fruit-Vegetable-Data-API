#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="${PROJECT_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
DATA_ROOT="${DATA_ROOT:-$PROJECT_ROOT}"
cd "$PROJECT_ROOT"

mkdir -p "$DATA_ROOT/logs" "$DATA_ROOT/data_downloads"

TODAY="$(TZ=Asia/Tokyo date +%Y%m%d)"
START_AT="$(TZ=Asia/Tokyo date +%Y-%m-%dT%H:%M:%S%z)"

echo "[$START_AT] daily pipeline start (date=$TODAY)"

if command -v downloader >/dev/null 2>&1; then
  downloader -date "$TODAY" -out "$DATA_ROOT/data_downloads" -wait-per-date 0s
else
  go run ./cmd/downloader -date "$TODAY" -out "$DATA_ROOT/data_downloads" -wait-per-date 0s
fi

UTF8_DIR="$DATA_ROOT/data_downloads/utf8/$TODAY"
if [ ! -d "$UTF8_DIR" ]; then
  echo "[$(TZ=Asia/Tokyo date +%Y-%m-%dT%H:%M:%S%z)] no utf8 directory for $TODAY, skip ingest"
  exit 0
fi

CSV_COUNT="$(find "$UTF8_DIR" -maxdepth 1 -type f -name '*.csv' | wc -l | tr -d ' ')"
if [ "$CSV_COUNT" = "0" ]; then
  echo "[$(TZ=Asia/Tokyo date +%Y-%m-%dT%H:%M:%S%z)] no csv files for $TODAY, skip ingest"
  exit 0
fi

if command -v ingestor >/dev/null 2>&1; then
  ingestor -in "$UTF8_DIR" -run-type "cron_daily_$TODAY" -fail-on-file-error
else
  go run ./cmd/ingestor -in "$UTF8_DIR" -run-type "cron_daily_$TODAY" -fail-on-file-error
fi

echo "[$(TZ=Asia/Tokyo date +%Y-%m-%dT%H:%M:%S%z)] daily pipeline done (date=$TODAY, files=$CSV_COUNT)"
