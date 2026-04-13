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
- `SELECT COUNT(*)` on large fact tables can be slow because PostgreSQL may scan the full table for an exact count

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
