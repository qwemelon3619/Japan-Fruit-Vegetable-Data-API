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
- Price Data
  - `GET /v1/prices/daily`
  - `GET /v1/prices/trend`
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
RUN_API_INTEGRATION_TESTS=1 API_BASE_URL=http://localhost:8080 go test ./tests/api -count=1
```

Run pipeline integration tests:
```bash
RUN_PIPELINE_INTEGRATION_TESTS=1 go test ./tests/pipeline -count=1
```

## Project Structure
- `cmd/`: executable entrypoints
- `internal/`: domain, handler, and platform logic
- `scripts/`: operational scripts
- `docker/`: nginx and cron settings
- `tests/`: unit/integration tests

## Limitations
- The downloader depends on the upstream MAFF HTML structure and CSV exposure flow; upstream changes may require parser updates
- Historical completeness is limited by source availability, not only by local storage
- Exact full-table analytics on large fact tables can become expensive without summary tables or filtered queries

## License
MIT License. See [LICENSE](./LICENSE).
