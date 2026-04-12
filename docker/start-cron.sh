#!/usr/bin/env bash
set -euo pipefail

mkdir -p /data/logs /data/data_downloads /etc/crontabs

SCHEDULE="${DAILY_INGEST_SCHEDULE:-${CRON_SCHEDULE:-0 10 * * *}}"
SNAPSHOT_SCHEDULE="${MONITOR_SNAPSHOT_SCHEDULE:-*/5 * * * *}"
CRON_FILE="/etc/crontabs/root"

cat > "$CRON_FILE" <<CRON
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
TZ=Asia/Tokyo
$SCHEDULE /bin/bash -lc '/usr/local/bin/daily_download_ingest.sh >> /data/logs/cron_daily.log 2>&1'
$SNAPSHOT_SCHEDULE /bin/bash -lc '/usr/local/bin/monitor_snapshot.sh >> /data/logs/monitor_snapshot.log 2>&1'
CRON

echo "[cron] daily_ingest_schedule=$SCHEDULE"
echo "[cron] monitor_snapshot_schedule=$SNAPSHOT_SCHEDULE"
echo "[cron] crontab:"
cat "$CRON_FILE"

exec crond -f -l 8
