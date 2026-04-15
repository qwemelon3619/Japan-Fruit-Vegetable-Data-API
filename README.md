# Japan Fruit & Vegetable Data API

This project collects Japanese fruit/vegetable wholesale data, normalizes it, stores it in PostgreSQL, and serves it through a query API.

## Project Goal
- Collect public Japanese produce CSV data reliably
- Build a normalization + ingestion pipeline
- Provide REST APIs for analytics and data exploration
- Operate with lightweight monitoring and scheduled jobs

## Main Components
- **Downloader**: date/range CSV collection, Shift-JIS -> UTF-8 conversion
- **Ingestor**: PostgreSQL ingestion pipeline
- **API Server**: query endpoints for price trends, comparison, ranking, and coverage
- **Ops Pipeline**: scheduled automation with Docker Compose + cron
- **Monitoring**: `/metrics` + CSV snapshots + dashboard

## Data Source & Attribution
- Source website: MAFF wholesale market data portal (Japan)
  - https://www.seisen.maff.go.jp/seisen/bs04b040md001/BS04B040UC010SC999-Evt001.do
- This project fetches and transforms publicly accessible CSV data from that portal.
- Data rights and usage conditions belong to the original provider.
- This repository is an independent implementation and is not affiliated with MAFF.

## Tech Stack
- Go
- PostgreSQL
- Docker
- Nginx

## Architecture (High Level)
1. `downloader` fetches raw CSV files and converts them to UTF-8
2. `ingestor` loads normalized data into PostgreSQL (dimension/fact model)
3. `api` exposes query and monitoring endpoints
4. `pipeline-cron` runs scheduled ingestion and monitoring snapshots

## Quick Start
```bash
cp .env.example .env
```

Set required values in `.env`:
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `POSTGRES_DB`

Execution:
```bash
docker compose up -d --build
docker compose ps
```

Default local base URL:
```text
http://localhost:8080
```

Production URL:
```text
https://jp-vgfr-api.seungpyo.xyz
```

Interactive API documentation:
```text
https://jp-vgfr-api.seungpyo.xyz/doc
```

LLM-friendly API documentation:
```text
https://jp-vgfr-api.seungpyo.xyz/doc-llm
```

Database / API SQL reference:
```text
./DB_API_SQL_REFERENCE.md
```

Administrative access policy:
```text
/doc
/doc-llm
/ready
/metrics
/monitoring/dashboard
/monitoring/snapshots.csv
/ingestion/runs
/ingestion/files
These routes should be restricted to administrators or internal access only.
Production deployments for this project additionally enforce access control and rate limiting at Cloudflare and at the Nginx reverse proxy.
```

## API Highlights
- Health/Ready/Metrics
  - `GET /health`
  - `GET /ready`
  - `GET /metrics`
- API Documentation
  - `GET /doc`
  - `GET /doc-llm`
- Coverage / Latest Data
  - `GET /v1/coverage`
  - `GET /v1/prices/latest`
- Dimension Lookups
  - `GET /v1/markets`
  - `GET /v1/items`
  - `GET /v1/origins`
- Price Data
  - `GET /v1/prices/daily`
  - `GET /v1/prices/trend`
  - `GET /v1/prices/trend/1m`
  - `GET /v1/prices/trend/6m`
  - `GET /v1/prices/trend/1y`
  - `GET /v1/prices/summary`
- Compare/Ranking
  - `GET /v1/compare/markets`
  - `GET /v1/rankings/items`

## Endpoint Behavior Summary

### System and Monitoring
- `GET /health`: liveness check, returns `{ "data": { "status": "ok" } }`
- `GET /ready`: readiness check backed by a cached database ping result
- `GET /metrics`: Prometheus plaintext metrics
- `GET /monitoring/dashboard`: monitoring dashboard HTML
- `GET /monitoring/snapshots.csv`: monitoring CSV snapshot file from the configured snapshot path

### Dimension APIs
- `GET /v1/markets`, `GET /v1/items`, `GET /v1/origins`
  - Purpose:
    - `markets`: discover valid market codes before exact-code filters on fact/aggregation APIs
    - `items`: discover valid item codes before daily/latest/trend/summary/ranking queries
    - `origins`: discover valid origin codes before exact origin filters on price endpoints
  - Query: `limit` (default `50`, max `500`), `offset`, `q`, `sort`, `order`
  - `q` searches code/name with `LIKE`
  - Response `meta`: `limit`, `offset`, `total`

### Coverage API
- `GET /v1/coverage`
  - Returns `earliest_trade_date`, `latest_trade_date`, `fact_rows_total`
  - Also includes latest ingestion metadata when available:
    - `last_ingestion_run_id`
    - `last_ingestion_status`
    - `last_ingestion_run_type`
    - `last_ingestion_finished_at`

### Price APIs
- `GET /v1/prices/daily`
  - Required: `item_code`
  - Optional: `date`, `from`, `to`, `market_code`, `origin_code`, `limit`, `offset`, `sort`, `order`
  - `date` is supported only on this endpoint among the main price list/aggregate APIs
  - If `date/from/to` are all omitted, a default recent filter is applied with `from = today - 31 days`
  - Response `meta`: `limit`, `offset`, `total`, and `default_from` / `default_window_days` when auto-applied

- `GET /v1/prices/latest`
  - Required: `item_code`
  - Optional: `market_code`, `origin_code`, `limit`, `offset`, `sort`, `order`
  - Not supported: `date`, `from`, `to`
  - Returns rows only for the latest available trade date matching the non-date filters
  - Response `meta`: `latest_trade_date`, `limit`, `offset`, `total`

- `GET /v1/prices/trend`
  - Required: `item_code`
  - Optional: `from`, `to`, `market_code`, `origin_code`
  - Aggregates by `trade_date`
  - Response fields include `rows_count`, `avg_price_mid_yen`, `max_price_mid_yen`, `min_price_mid_yen`, `arrival_ton_sum`
  - If `date/from/to` are all omitted, a default recent filter is applied with `from = today - 31 days`
  - Preset routes:
    - `GET /v1/prices/trend/1m`
    - `GET /v1/prices/trend/6m`
    - `GET /v1/prices/trend/1y`

- `GET /v1/prices/summary`
  - Required: `item_code`
  - Optional: `group_by=day|week|month`, `from`, `to`, `market_code`, `origin_code`
  - Aggregates by `period`
  - Response fields include `rows_count`, `avg_price_mid_yen`, `arrival_ton_sum`
  - If `date/from/to` are all omitted, a default recent filter is applied with `from = today - 31 days`

### Compare and Ranking APIs
- `GET /v1/compare/markets`
  - Required: `date`, `item_code`
  - Optional: `metric=price_mid|arrival`, `order`
  - Default metric: `price_mid`

- `GET /v1/rankings/items`
  - Required: `date`
  - Optional: `metric=arrival|price_mid`, `market_code`, `limit`, `order`
  - Default metric: `arrival`
  - `limit` default `50`, max `500`

### Ingestion Admin APIs
- `GET /ingestion/runs`
  - Optional: `limit` (default `50`, max `500`), `offset`
  - Returns ingestion runs ordered by `id DESC`

- `GET /ingestion/files`
  - Optional: `run_id`, `limit` (default `100`, max `1000`), `offset`
  - `run_id` must be a positive integer when provided
  - Returns ingestion files ordered by `id DESC`

## Deployment Notes
- `japan-data-api` must run with `command: "api"` so the API process stays up
- `japan-data-pipeline-cron` must run with `command: "start-cron.sh"` so cron stays up in foreground
- If `command` is omitted when using prebuilt images, the image default command runs a one-shot pipeline script and the container will exit normally
- Reverse proxy, IP restriction, or Cloudflare rules are recommended for `/monitoring`, `/metrics`, and `/doc` in public deployments
- Restrict `/doc`, `/doc-llm`, `/monitoring/*`, `/metrics`, `/ready`, and ingestion-related routes to administrators or internal networks
- Production environment uses Cloudflare access controls and burst protection in addition to Nginx access rules

## Data Workflow
1. `downloader` fetches source CSV files from the MAFF portal
2. Source data is converted from Shift-JIS to UTF-8
3. Files are merged into one daily file: `utf8/<YYYYMMDD>/all_<YYYYMMDD>.csv`
4. `ingestor` reads those UTF-8 files and loads them into PostgreSQL
5. Monitoring snapshots are written separately to `data/monitoring/csv/snapshots.csv`

## Data Coverage
- Current known earliest stored trade date: `2021-04-08`
- Coverage depends on source availability; non-business days and unpublished dates are skipped
- Downloader can attempt wider historical ranges, but only dates exposed by the source portal are actually stored

## Environment Variables
- `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`: database connection
- `POSTGRES_SSLMODE`, `POSTGRES_TIMEZONE`: PostgreSQL connection behavior
- `HTTP_PORT`: API listen port inside the container/process
- `DAILY_INGEST_SCHEDULE`: cron schedule for daily download + ingest
- `MONITOR_SNAPSHOT_SCHEDULE`: cron schedule for monitoring CSV snapshots
- `API_BASE_URL`: monitoring script target for `/ready`, `/metrics`, and ingestion status checks
- `MONITOR_OUT_DIR`, `SNAPSHOT_FILE`: monitoring CSV output location

## Operational Behavior
- `no market table rows` can be normal on holidays, weekends, or dates not published by the source portal
- If a target date has no published data, the downloader skips that date and ingest is skipped for that date as well
- Monitoring CSV snapshots are generated on a separate cron schedule and do not require daily ingest to succeed
- `/v1/coverage` returns `fact_rows_total` as a PostgreSQL statistics-based estimate (not exact full-table count) to avoid heavy `COUNT(*)` scans on large tables
- `/v1/prices/daily`, `/v1/prices/trend`, and `/v1/prices/summary` apply a default recent-date filter (`from = today - 31 days`) when `date/from/to` are all omitted
- `/v1/prices/latest` always returns rows from the latest available trade date for the requested non-date filters and does not support `date`, `from`, or `to`
- When the default recent-date filter is auto-applied, response `meta` includes `default_from` and `default_window_days`
- When source data has no origin name/code, the ingestor normalizes it to `origin_code='UNKNOWN'` and `origin_name='不明'`

## Example Queries
```bash
curl -s "https://jp-vgfr-api.seungpyo.xyz/v1/coverage"
curl -s "https://jp-vgfr-api.seungpyo.xyz/v1/prices/latest?item_code=30100&limit=10"
curl -s "https://jp-vgfr-api.seungpyo.xyz/v1/prices/daily?item_code=30100&from=2026-04-01&to=2026-04-10&limit=10"
curl -s "https://jp-vgfr-api.seungpyo.xyz/v1/prices/trend?item_code=30100&from=2026-04-01&to=2026-04-30"
curl -s "https://jp-vgfr-api.seungpyo.xyz/v1/rankings/items?date=2026-04-16&metric=arrival&limit=20"
```

## Testing
Run all tests:
```bash
go test ./...
```

Downloader example (merge into one UTF-8 CSV and delete source files):
```bash
go run ./cmd/downloader -date 20260412 -out ./data/data_downloads
```

Run API integration tests:
```bash
RUN_API_INTEGRATION_TESTS=1 API_BASE_URL=http://localhost:8080 go test ./tests/unit/api -count=1
```

Run pipeline integration tests:
```bash
RUN_PIPELINE_INTEGRATION_TESTS=1 go test ./tests/unit/pipeline -count=1
```

Run stress tests with `k6`:
```bash
k6 run tests/stress/baseline.js
k6 run tests/stress/step_read.js
k6 run tests/stress/mixed_read.js
```

## Project Structure
- `cmd/`: executable entrypoints
- `internal/`: domain, handler, and platform logic
- `scripts/`: operational scripts
- `docker/`: nginx and cron settings
- `tests/unit/`: unit and integration tests
- `tests/stress/`: `k6` stress test scripts and result artifacts

## Stress Testing

### Current setup
- Stress test scripts live under [tests/stress](./tests/stress)
- Result JSON files are stored under [tests/stress/result](./tests/stress/result)
- The current default target is the production API:
  - `https://jp-vgfr-api.seungpyo.xyz`

### Current production test target
- API server: `1 vCPU / 1GB RAM`
- PostgreSQL server: `1 vCPU / 1GB RAM`

This means tail latency under aggregation-heavy load is expected to rise earlier than on a larger deployment.

### Current `k6` profiles
- `baseline.js`
  - 5 VUs for 2 minutes
  - low-risk smoke/baseline check
- `step_read.js`
  - 5 -> 10 -> 20 -> 30 VUs
  - conservative production-safe staged read test
- `mixed_read.js`
  - 10 -> 15 -> 15 VUs
  - conservative read load while ingestion runs in parallel

### Current observed results

Baseline result from `tests/stress/result/baseline.json`:
- total requests: `580`
- request rate: about `4.80 req/s`
- p95 latency: about `50.19ms`
- average latency: about `37.88ms`
- max latency: about `215.10ms`
- request checks: `580/580` passed

Step-read result from `tests/stress/result/step_read.json`:
- total requests: `9328`
- request rate: about `11.94 req/s`
- p95 latency: about `679.06ms`
- average latency: about `119.75ms`
- max latency: about `4525.22ms`
- failed request rate: about `0.01%`
- request checks: `9327/9328` passed

### Interpretation
- On this hardware profile, baseline performance is strong.
- Under staged read load, median latency remains acceptable, but tail latency rises significantly.
- The system still remains mostly available under the current conservative production-safe load.
- The likely source of tail latency is aggregation-heavy endpoints such as:
  - `/v1/prices/trend`
  - `/v1/prices/summary`
  - `/v1/rankings/items`

### What to look at when analyzing results
- `http_req_failed`
- `http_req_duration` p50 / p95 / p99
- max latency spikes
- `/metrics`
- `docker stats`
- `pg_stat_activity`
- `pg_stat_statements`

### Recommended optimization ideas
- Add short-TTL caching for:
  - `/v1/coverage`
  - `/v1/prices/latest`
  - `/v1/prices/trend`
  - `/v1/prices/summary`
  - `/v1/rankings/items`
- Consider summary tables or materialized views for aggregation-heavy endpoints.
- Keep the normalized source-of-truth fact table, but introduce pre-aggregated read models for trend/summary/ranking queries.
- If read/write contention becomes an issue during mixed load, consider a read replica before introducing more complex DB partitioning or sharding.
- Add new indexes only after confirming slow-query patterns with `pg_stat_statements` and `EXPLAIN ANALYZE`.

## Architecture Diagram
```text
                        +-----------------------------+
                        | MAFF wholesale data portal |
                        | seisen.maff.go.jp          |
                        +-------------+---------------+
                                      |
                                      | CSV / HTML fetch
                                      v
                    +---------------------------------------+
                    | downloader                            |
                    | - date/range fetch                    |
                    | - Shift-JIS -> UTF-8 conversion       |
                    | - daily merged CSV output             |
                    +-------------------+-------------------+
                                        |
                                        | utf8/<YYYYMMDD>/all_<YYYYMMDD>.csv
                                        v
                    +---------------------------------------+
                    | ingestor                              |
                    | - parse + validate                    |
                    | - dimension upsert                    |
                    | - fact stage copy                     |
                    | - fact merge on source_file,row_no    |
                    +-------------------+-------------------+
                                        |
                            +-----------+------------+
                            | PostgreSQL             |
                            | - dim_*                |
                            | - fact_prices_daily    |
                            | - ingestion_*          |
                            +-----------+------------+
                                        |
                     +------------------+------------------+
                     |                                     |
                     v                                     v
          +-------------------------+          +--------------------------+
          | API server              |          | monitoring snapshot job  |
          | - /v1/*                 |          | - /ready, /metrics check |
          | - /ready, /metrics      |          | - CSV snapshot output    |
          | - /doc, /doc-llm        |          +-------------+------------+
          +------------+------------+                        |
                       |                                     v
                       v                     data/monitoring/csv/snapshots.csv
          +-----------------------------+
          | Nginx / Cloudflare / users |
          +-----------------------------+
```

## Why This Design

### Why split downloader and ingestor
- Upstream CSV acquisition and DB ingestion fail for different reasons, so separating them makes retries and diagnosis simpler.
- Keeping UTF-8 normalized files on disk gives a reproducible intermediate artifact for debugging and re-ingestion.
- The downloader can skip unpublished dates without forcing DB write logic to run.

### Why use dimension + fact tables
- Query APIs repeatedly filter by market, item, origin, and date, so a dimension/fact model keeps repeated strings out of the largest table.
- Stable codes such as `market_code`, `item_code`, and `origin_code` map naturally to dimension lookups and exact filters.
- Aggregation endpoints become simpler because the fact table stores already-normalized numeric fields.

### Why keep ingestion metadata tables
- `ingestion_runs` and `ingestion_files` give operators a first-class audit trail instead of inferring status from logs only.
- `/v1/coverage` can expose recent ingestion status without reading log files or container state.

### Why expose both raw and aggregated APIs
- `daily` supports debugging and exports.
- `latest` supports current-state dashboards.
- `trend` and `summary` support time-series and rollup analytics.
- `compare` and `rankings` support product-facing exploratory views without requiring clients to compose complex SQL.

## DB Schema and Index Design

### Schema overview
- `dim_market`: market code/name master
- `dim_item`: item code/name master
- `dim_origin`: origin code/name master
- `dim_grade`: grade/class/product/unit-weight composite dimension
- `fact_prices_daily`: normalized daily wholesale fact rows
- `ingestion_runs`, `ingestion_files`: ingestion audit metadata

Full SQL-oriented schema notes live in [DB_API_SQL_REFERENCE.md](./DB_API_SQL_REFERENCE.md).

### Why these keys
- `dim_market.market_code`, `dim_item.item_code`, `dim_origin.origin_code` are unique because codes are the canonical API filter keys.
- `dim_grade` uses a composite unique key because no single source code identifies grade/class/product/unit-weight combinations.
- `fact_prices_daily` uses `source_file + source_row_no` as a unique key because that is the closest stable row identity from the upstream source.

### Why these fact indexes
- `idx_fact_trade_market (trade_date, market_id)`
  - supports date-first scans and market comparisons on a single day.
- `idx_fact_trade_item_market (trade_date, item_id, market_id)`
  - supports date + item filters and compare/ranking patterns where market grouping matters.
- `idx_fact_item_trade (item_id, trade_date)`
  - supports `daily`, `trend`, and `summary` where `item_code` is required and date range is common.
- `idx_fact_origin_trade (origin_id, trade_date)`
  - supports origin-filtered queries without requiring a full scan by date.

### Why not store everything denormalized
- It would simplify some reads, but the fact table would be much wider and more repetitive.
- Repeated market/item/origin names would increase storage and update cost.
- Exact code lookup APIs would still need canonical code/name management somewhere.

### Why not use exact `COUNT(*)` for coverage
- `/v1/coverage` is intended as a lightweight operational endpoint.
- Full-table counts on a large fact table are expensive and unnecessary for a coarse coverage indicator.
- PostgreSQL statistics via `pg_class.reltuples` are a deliberate latency vs accuracy tradeoff.

## Failure, Exception, and Operations Strategy

### Upstream data exceptions
- The source portal may not publish data for weekends, holidays, or some dates.
- The downloader treats `no market table rows` as a normal skip condition for unpublished dates.
- Historical completeness is therefore limited by upstream exposure, not only by local storage.

### Ingestion exceptions
- CSV rows are parsed with validation for required columns, numeric fields, and non-negative values.
- Bad rows are counted into `rows_error` instead of crashing the entire system by default.
- File- or group-level failures are captured into ingestion status tables for later inspection.

### Idempotency and retry behavior
- Re-ingesting the same source file does not duplicate facts because merge uses `ON CONFLICT (source_file, source_row_no)`.
- Dimension tables are upserted before fact merge, so reference IDs stay consistent across reruns.
- Transaction-level retriable errors are explicitly recognized for retry handling.

### Operational protection
- Admin and monitoring routes are expected to be protected by Nginx, Cloudflare, or network policy.
- Monitoring snapshots are generated independently from daily ingest so observability can continue even when ingestion fails.
- `/ready` checks database availability; `/metrics` exposes request and DB error signals for alerting.

### Common operator actions
- If `/v1/coverage.fact_rows_total` looks stale after ingest, run `ANALYZE fact_prices_daily;`.
- If source origin is missing, the ingestor normalizes it to `origin_code='UNKNOWN'` and `origin_name='不明'`.
- Use `/ingestion/runs` and `/ingestion/files` to distinguish full-run failure from partial file failure.

## Testing Strategy

### Test layers
- Unit tests cover handler/helper behavior and validation logic.
- API integration tests validate live endpoint behavior against a running API.
- Pipeline integration tests validate downloader/ingestor behavior against the DB-backed pipeline.

### Why environment-gated integration tests
- API and pipeline integration tests need a running service or database-backed environment.
- Gating with `RUN_API_INTEGRATION_TESTS` and `RUN_PIPELINE_INTEGRATION_TESTS` keeps normal local `go test ./...` fast and predictable.

### What the tests are trying to prove
- The handler contract matches documented response shapes and status codes.
- Query parameter validation behaves correctly.
- Ingestion writes normalized data without breaking uniqueness or metadata logging assumptions.
- End-to-end flows remain stable when running against the real service boundary.

## Monitoring Metrics and Signals

### Primary observability surfaces
- `GET /health`: process liveness
- `GET /ready`: DB-backed readiness
- `GET /metrics`: Prometheus metrics
- `GET /monitoring/dashboard`: lightweight operator UI
- `GET /monitoring/snapshots.csv`: snapshot history for low-cost monitoring

### Useful metrics/signals
- HTTP request totals by path/method/status
- 5xx error count
- DB error totals
- request latency, including p95-style monitoring views
- latest ingestion status
- readiness success/failure

### Why snapshots exist in addition to Prometheus metrics
- CSV snapshots are simple to archive, inspect, and diff without a full metrics backend.
- They provide a lightweight operational history even in minimal deployments.
- They decouple coarse operational visibility from live scrape availability.

## Real API Examples

### Coverage and latest ingestion
```bash
curl -s "http://localhost:8080/v1/coverage"
```

```json
{
  "data": {
    "earliest_trade_date": "2021-04-08",
    "latest_trade_date": "2026-04-10",
    "fact_rows_total": 123456,
    "last_ingestion_run_id": 42,
    "last_ingestion_status": "success",
    "last_ingestion_run_type": "daily",
    "last_ingestion_finished_at": "2026-04-10T01:23:45Z"
  },
  "meta": {}
}
```

### Dimension lookup
```bash
curl -s "http://localhost:8080/v1/items?q=だいこん&limit=5&order=asc"
```

```json
{
  "data": [
    { "id": 1, "code": "30100", "name": "だいこん" }
  ],
  "meta": { "limit": 5, "offset": 0, "total": 1 }
}
```

### Raw fact rows
```bash
curl -s "http://localhost:8080/v1/prices/daily?item_code=30100&from=2026-04-01&to=2026-04-10&limit=3"
```

```json
{
  "data": [
    {
      "trade_date": "2026-04-10",
      "market_code": "13300",
      "item_code": "30100",
      "origin_code": "40100",
      "price_mid_yen": 2480
    }
  ],
  "meta": { "limit": 3, "offset": 0, "total": 1 }
}
```

### Trend aggregation
```bash
curl -s "http://localhost:8080/v1/prices/trend?item_code=30100&from=2026-04-01&to=2026-04-30"
```

```json
{
  "data": [
    {
      "trade_date": "2026-04-01",
      "rows_count": 12,
      "avg_price_mid_yen": 2510.5,
      "max_price_mid_yen": 3000,
      "min_price_mid_yen": 2000,
      "arrival_ton_sum": 14.2
    }
  ],
  "meta": { "total": 1 }
}
```

### Ranking view
```bash
curl -s "http://localhost:8080/v1/rankings/items?date=2026-04-16&metric=arrival&limit=20"
```

```json
{
  "data": [
    {
      "item_code": "30100",
      "item_name": "だいこん",
      "rows_count": 14,
      "metric_value": 42.7
    }
  ],
  "meta": { "metric": "arrival", "limit": 20, "total": 1 }
}
```

## Tradeoffs

### What this design optimizes for
- operational simplicity
- reproducible ingest artifacts
- low-friction analytics APIs
- straightforward SQL and debugging

### Tradeoffs accepted
- No auth middleware in the app itself.
  - Simpler app code, but security depends on reverse proxy and network controls.
- No heavy warehouse layer or summary tables.
  - Simpler writes and fewer moving parts, but some exact large-scale analytics are intentionally avoided.
- Coverage count is estimated, not exact.
  - Faster operational endpoint, but less precise.
- Recent-date defaults on some endpoints.
  - Safer and cheaper default queries, but users must know to pass explicit ranges for full-history analysis.
- Dimension/fact normalization instead of a wide denormalized fact table.
  - Better consistency and storage profile, but requires joins on read.

## Limitations
- The downloader depends on the upstream MAFF HTML structure and CSV exposure flow; upstream changes may require parser updates
- Historical completeness is limited by source availability, not only by local storage
- Exact full-table analytics on large fact tables can become expensive without summary tables or filtered queries

## License
MIT License. See [LICENSE](./LICENSE).
