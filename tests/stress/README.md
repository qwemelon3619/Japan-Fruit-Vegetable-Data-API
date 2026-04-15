# Stress Tests

This directory contains `k6`-based stress test scripts for the API.

## Contents

- `baseline.js`
  - verifies basic latency and correctness under low load
- `step_read.js`
  - increases virtual users in conservative production-safe stages to find read-path limits
- `mixed_read.js`
  - applies conservative read load while ingestion is running in parallel

## Prerequisites

The API server must be running and reachable at the configured base URL.

Default base URL:

```text
https://jp-vgfr-api.seungpyo.xyz
```

## Example Commands

### Baseline

```bash
mkdir -p tests/stress/result
BASE_URL=https://jp-vgfr-api.seungpyo.xyz \
k6 run tests/stress/baseline.js \
  --summary-export tests/stress/result/baseline.json
```

### Step Read

```bash
mkdir -p tests/stress/result
BASE_URL=https://jp-vgfr-api.seungpyo.xyz \
k6 run tests/stress/step_read.js \
  --summary-export tests/stress/result/step_read.json
```

### Mixed Read

```bash
mkdir -p tests/stress/result
BASE_URL=https://jp-vgfr-api.seungpyo.xyz \
k6 run tests/stress/mixed_read.js \
  --summary-export tests/stress/result/mixed_read.json
```

Run ingestion in a separate terminal during the mixed-load scenario:

```bash
go run ./cmd/ingestor
```

## Environment Variables

- `BASE_URL`
  - default: `https://jp-vgfr-api.seungpyo.xyz`
- `ITEM_CODE`
  - default: `30100`
- `TEST_DATE`
  - default: `2026-04-10`
- `FROM_DATE`
  - default: `2026-03-01`
- `TO_DATE`
  - default: `2026-04-01`

## What To Observe

- `http_req_failed`
- `http_req_duration`
- per-endpoint p95 and p99 latency
- any 5xx responses
- `/metrics`
- `docker stats`
- `pg_stat_activity`
- `pg_stat_statements`

## Current Load Profile

- `baseline.js`
  - 5 VUs for 2 minutes
  - intended as a low-risk production smoke check
- `step_read.js`
  - 5 -> 10 -> 20 -> 30 VUs in stages
  - intended as a conservative production read-load test
- `mixed_read.js`
  - 10 -> 15 -> 15 VUs in stages
  - intended for read load during ingestion with lower operational risk
