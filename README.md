# Japan Fruit & Vegetable Data API

This project collects Japanese fruit/vegetable wholesale data, normalizes it, stores it in PostgreSQL, and serves it through a query API.

## Project Goal
- Collect public Japanese produce CSV data reliably
- Build a normalization + ingestion pipeline
- Provide REST APIs for analytics and data exploration
- Operate with lightweight monitoring and scheduled jobs

## Main Components
- **Downloader**: date/range CSV collection, Shift-JIS -> UTF-8 conversion
- **Ingestor**: PostgreSQL ingestion with run/file-level logs
- **API Server**: query endpoints for price trends, comparison, ranking, and ingestion history
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
- Docker Compose
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

실행:
```bash
docker compose up -d --build
docker compose ps
```

Default local base URL:
```text
http://localhost:8080
```

## API Highlights
- Health/Ready/Metrics
  - `GET /health`
  - `GET /ready`
  - `GET /metrics`
- API Documentation
  - `GET /doc`
- Price Data
  - `GET /v1/prices/daily`
  - `GET /v1/prices/trend`
  - `GET /v1/prices/summary`
- Compare/Ranking
  - `GET /v1/compare/markets`
  - `GET /v1/rankings/items`
- Ingestion History
  - `GET /v1/ingestion/runs`
  - `GET /v1/ingestion/files`

## Testing
Run all tests:
```bash
go test ./...
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

## License
MIT License. See [LICENSE](./LICENSE).
