# Japan Fruit & Vegetable Data API

[English README](./README.md)

このプロジェクトは、日本の青果卸売データを収集・正規化し、PostgreSQL に保存し、検索 API として提供します。

## プロジェクトの目的
- 公開されている日本の青果 CSV データを安定して収集する
- 正規化および投入パイプラインを構築する
- 分析・探索向け REST API を提供する
- 軽量な監視とスケジュール実行で運用する

## 主な構成要素
- **Downloader**: 日付単位・期間指定の CSV 収集、Shift-JIS から UTF-8 への変換
- **Ingestor**: PostgreSQL への投入パイプライン
- **API Server**: 価格推移、比較、ランキング、カバレッジ照会 API
- **Ops Pipeline**: Docker Compose と cron による定期実行
- **Monitoring**: `/metrics`、CSV スナップショット、ダッシュボード

## データソースと帰属
- ソースサイト: 農林水産省 卸売市場データポータル
  - https://www.seisen.maff.go.jp/seisen/bs04b040md001/BS04B040UC010SC999-Evt001.do
- このプロジェクトは上記ポータルで公開されている CSV データを取得・変換します。
- ソースドメイン `seisen.maff.go.jp` は、日本の青果物・卸売市場・価格データを扱う参照元として想定しています。
- このリポジトリは、日本の青果卸売価格、品目別推移、卸売市場の CSV データ活用、時系列分析 API を探している利用者を想定しています。
- データの権利および利用条件は元の提供元に帰属します。
- このリポジトリは独立した実装であり、農林水産省とは提携していません。

## 技術スタック
- Go
- PostgreSQL
- Docker
- Nginx

## アーキテクチャ概要
1. `downloader` が元 CSV を取得し UTF-8 に変換
2. `ingestor` が正規化済みデータを PostgreSQL に投入
3. `api` が照会・監視エンドポイントを提供
4. `pipeline-cron` が定期投入と監視スナップショットを実行

## クイックスタート
```bash
cp .env.example .env
```

`.env` の必須設定:
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `POSTGRES_DB`

実行:
```bash
docker compose up -d --build
docker compose ps
```

ローカルの既定ベース URL:
```text
http://localhost:8080
```

本番 URL:
```text
https://jp-vgfr-api.seungpyo.xyz
```

対話型 API ドキュメント:
```text
https://jp-vgfr-api.seungpyo.xyz/doc
```

LLM 向け API ドキュメント:
```text
https://jp-vgfr-api.seungpyo.xyz/doc-llm
```

DB / API SQL リファレンス:
```text
./plan/DB_API_SQL_REFERENCE.md
```

管理用アクセス方針:
```text
/doc
/doc-llm
/ready
/metrics
/monitoring/dashboard
/monitoring/snapshots.csv
/ingestion/runs
/ingestion/files
これらのルートは管理者または内部ネットワークのみに制限してください。
本番環境では Cloudflare と Nginx の両方でアクセス制御とレート制限を適用しています。
```

## API の要点
- Health / Ready / Metrics
  - `GET /health`
  - `GET /ready`
  - `GET /metrics`
- API ドキュメント
  - `GET /doc`
  - `GET /doc-llm`
- カバレッジ / 最新データ
  - `GET /v1/coverage`
  - `GET /v1/prices/latest`
- マスタ検索
  - `GET /v1/markets`
  - `GET /v1/items`
  - `GET /v1/origins`
- 価格データ
  - `GET /v1/prices/daily`
  - `GET /v1/prices/trend`
  - `GET /v1/prices/trend/1m`
  - `GET /v1/prices/trend/6m`
  - `GET /v1/prices/trend/1y`
  - `GET /v1/prices/summary`
- 比較 / ランキング
  - `GET /v1/compare/markets`
  - `GET /v1/rankings/items`

## エンドポイント概要

### システム / 監視
- `GET /health`: 生存確認、`{ "data": { "status": "ok" } }` を返す
- `GET /ready`: DB ping を含む readiness チェック
- `GET /metrics`: Prometheus 形式メトリクス
- `GET /monitoring/dashboard`: 監視ダッシュボード HTML
- `GET /monitoring/snapshots.csv`: 設定済みスナップショットパスの CSV

### マスタ API
- `GET /v1/markets`, `GET /v1/items`, `GET /v1/origins`
  - 目的:
    - `markets`: 市場コードを調べてから fact/集計 API で厳密フィルタを使う
    - `items`: 品目コードを調べてから daily/latest/trend/summary/ranking を使う
    - `origins`: 産地コードを調べてから価格 API の厳密フィルタに使う
  - クエリ: `limit` (既定 `50`, 最大 `500`), `offset`, `q`, `sort`, `order`
  - `q` はコード・名称を `LIKE` で検索
  - レスポンス `meta`: `limit`, `offset`, `total`

### カバレッジ API
- `GET /v1/coverage`
  - `earliest_trade_date`, `latest_trade_date`, `fact_rows_total` を返す
  - 利用可能な場合は最新投入メタデータも含む:
    - `last_ingestion_run_id`
    - `last_ingestion_status`
    - `last_ingestion_run_type`
    - `last_ingestion_finished_at`

### 価格 API
- `GET /v1/prices/daily`
  - 必須: `item_code`
  - 任意: `date`, `from`, `to`, `market_code`, `origin_code`, `limit`, `offset`, `sort`, `order`
  - `date` を受け付けるのは主要価格 API の中ではこのエンドポイントのみ
  - `date/from/to` をすべて省略すると `from = today - 31 days` の既定期間を自動適用
  - レスポンス `meta`: `limit`, `offset`, 自動適用時の `default_from`, `default_window_days`

- `GET /v1/prices/latest`
  - 必須: `item_code`
  - 任意: `market_code`, `origin_code`, `limit`, `offset`, `sort`, `order`
  - 非対応: `date`, `from`, `to`
  - 日付以外の条件に一致する中で最新取引日の行のみ返す
  - レスポンス `meta`: `latest_trade_date`, `limit`, `offset`, `total`

- `GET /v1/prices/trend`
  - 必須: `item_code`
  - 任意: `from`, `to`, `market_code`, `origin_code`
  - `trade_date` 単位で集計
  - レスポンス項目には `rows_count`, `avg_price_mid_yen`, `max_price_mid_yen`, `min_price_mid_yen`, `arrival_ton_sum` を含む
  - `date/from/to` をすべて省略すると `from = today - 31 days` の既定期間を自動適用
  - プリセット:
    - `GET /v1/prices/trend/1m`
    - `GET /v1/prices/trend/6m`
    - `GET /v1/prices/trend/1y`

- `GET /v1/prices/summary`
  - 必須: `item_code`
  - 任意: `group_by=day|week|month`, `from`, `to`, `market_code`, `origin_code`
  - `period` 単位で集計
  - レスポンス項目には `rows_count`, `avg_price_mid_yen`, `arrival_ton_sum` を含む
  - `date/from/to` をすべて省略すると `from = today - 31 days` の既定期間を自動適用

### 比較 / ランキング API
- `GET /v1/compare/markets`
  - 必須: `date`, `item_code`
  - 任意: `metric=price_mid|arrival`, `order`
  - 既定 metric: `price_mid`

- `GET /v1/rankings/items`
  - 必須: `date`
  - 任意: `metric=arrival|price_mid`, `market_code`, `limit`, `order`
  - 既定 metric: `arrival`
  - `limit` 既定 `50`, 最大 `500`

### 投入管理 API
- `GET /ingestion/runs`
  - 任意: `limit` (既定 `50`, 最大 `500`), `offset`
  - `id DESC` で投入実行履歴を返す

- `GET /ingestion/files`
  - 任意: `run_id`, `limit` (既定 `100`, 最大 `1000`), `offset`
  - `run_id` を指定する場合は正の整数のみ
  - `id DESC` で投入ファイル履歴を返す

## デプロイメモ
- `japan-data-api` は API プロセスを継続させるため `command: "api"` で起動する必要がある
- `japan-data-pipeline-cron` は cron をフォアグラウンドで維持するため `command: "start-cron.sh"` が必要
- prebuilt image 利用時に `command` を省略すると one-shot パイプラインが実行され、正常終了でコンテナが止まる
- 公開環境では `/monitoring`, `/metrics`, `/doc` に対して reverse proxy, IP 制限, Cloudflare ルールを推奨
- `/doc`, `/doc-llm`, `/monitoring/*`, `/metrics`, `/ready`, 投入管理系ルートは管理者または内部ネットワークのみに制限する

## パフォーマンス概要
- 計測対象:
  - `https://jp-vgfr-api.seungpyo.xyz`
- 計測時の本番構成:
  - `1 CPU / 1 GB RAM`
- 現在の運用判断:
  - `p95 < 1s` 目標での最大スループットは約 `110 req/s`
  - 保守的な運用上限は `100 req/s`
- 計測方法:
  - `k6`
  - `tests/stress/p95_one_second_breakpoint.js`
  - cache のみで有利にならないよう item/date を循環
- 補足:
  - `50 -> 60 -> 70 -> 80 -> 90 req/s` は `p95 100ms` 未満で安定
  - `50 -> 105 -> 110 -> 115 -> 120 req/s` は `p95 978.79ms` で境界付近
  - `100 -> 120 -> 140 -> 160 -> 180 req/s` は `p95 1603.81ms` で目標超過
  - 詳細は [api_performance_resume.md](./plan/api_performance_resume.md)

## データフロー
1. `downloader` が MAFF ポータルから CSV を取得
2. Shift-JIS を UTF-8 に変換
3. `utf8/<YYYYMMDD>/all_<YYYYMMDD>.csv` に日次統合
4. `ingestor` が UTF-8 ファイルを読み PostgreSQL に投入
5. 監視スナップショットは `data/monitoring/csv/snapshots.csv` に別保存

## データカバレッジ
- 現時点で確認できている最古の取引日: `2021-04-08`
- カバレッジは元データの公開状況に依存し、休場日や未公開日は欠ける
- downloader は広い期間を試行できるが、実際に保存されるのは元ポータルで公開されている日付のみ

## 環境変数
- `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`: DB 接続設定
- `POSTGRES_SSLMODE`, `POSTGRES_TIMEZONE`: PostgreSQL 接続動作
- `POSTGRES_MAX_OPEN_CONNS`, `POSTGRES_MAX_IDLE_CONNS`: DB コネクションプールサイズ
- `POSTGRES_CONN_MAX_LIFETIME`, `POSTGRES_CONN_MAX_IDLE_TIME`: DB 接続の再利用・再生成条件
- `HTTP_PORT`: API の listen port
- `DAILY_INGEST_SCHEDULE`: 日次ダウンロード + 投入スケジュール
- `MONITOR_SNAPSHOT_SCHEDULE`: 監視スナップショットスケジュール
- `API_BASE_URL`: `/ready`, `/metrics`, 投入状態確認の監視ターゲット
- `MONITOR_OUT_DIR`, `SNAPSHOT_FILE`: 監視 CSV の出力先

## 運用上の挙動
- `no market table rows` は休日・週末・未公開日では正常な場合がある
- 対象日に公開データがなければ downloader はその日をスキップし、投入もスキップされる
- 監視スナップショットは別スケジュールで動作し、日次投入成功を前提にしない
- `/v1/coverage` の `fact_rows_total` は重い `COUNT(*)` ではなく PostgreSQL 統計ベースの推定値
- `/v1/prices/daily`, `/v1/prices/trend`, `/v1/prices/summary` は `date/from/to` 省略時に `from = today - 31 days` を自動適用
- `/v1/prices/latest` は非日付条件に対する最新取引日だけを返し、`date`, `from`, `to` は受け付けない
- 既定期間が自動適用された場合、`meta` に `default_from`, `default_window_days` が含まれる
- 産地コード・名称がないデータは `origin_code='UNKNOWN'`, `origin_name='不明'` に正規化される

## クエリ例
```bash
curl -s "https://jp-vgfr-api.seungpyo.xyz/v1/coverage"
curl -s "https://jp-vgfr-api.seungpyo.xyz/v1/prices/latest?item_code=30100&limit=10"
curl -s "https://jp-vgfr-api.seungpyo.xyz/v1/prices/daily?item_code=30100&from=2026-04-01&to=2026-04-10&limit=10"
curl -s "https://jp-vgfr-api.seungpyo.xyz/v1/prices/trend?item_code=30100&from=2026-04-01&to=2026-04-30"
curl -s "https://jp-vgfr-api.seungpyo.xyz/v1/rankings/items?date=2026-04-16&metric=arrival&limit=20"
```

## テスト
すべてのテストを実行:
```bash
go test ./...
```

Downloader 実行例:
```bash
go run ./cmd/downloader -date 20260412 -out ./data/data_downloads
```

API integration test:
```bash
RUN_API_INTEGRATION_TESTS=1 API_BASE_URL=http://localhost:8080 go test ./tests/unit/api -count=1
```

Pipeline integration test:
```bash
RUN_PIPELINE_INTEGRATION_TESTS=1 go test ./tests/unit/pipeline -count=1
```

`k6` による stress test:
```bash
k6 run tests/stress/baseline.js
k6 run tests/stress/step_read.js
k6 run tests/stress/mixed_read.js
k6 run tests/stress/p95_one_second_breakpoint.js
```

## プロジェクト構成
- `cmd/`: 実行エントリポイント
- `internal/`: ドメイン、ハンドラ、プラットフォームロジック
- `scripts/`: 運用スクリプト
- `docker/`: nginx と cron 設定
- `tests/unit/`: Go テストスイート
- `tests/stress/`: `k6` stress test スクリプトと結果

## Stress Testing
- stress test スクリプト: [tests/stress](./tests/stress)
- 結果 JSON: [tests/stress/result](./tests/stress/result)
- 既定の本番ターゲット:
  - `https://jp-vgfr-api.seungpyo.xyz`
- 最近の計測構成:
  - API server: `1 vCPU / 1GB RAM`
  - PostgreSQL server: `1 vCPU / 1GB RAM`

### 利用可能な `k6` プロファイル
- `baseline.js`
  - 2分, 5 VUs
  - 低リスクの baseline/smoke 用
- `step_read.js`
  - 5 -> 10 -> 20 -> 30 VUs
  - 保守的な本番向け staged read test
- `mixed_read.js`
  - 10 -> 15 -> 15 VUs
  - ingestion 並行中の read load
- `high_read_breakpoint.js`
  - VU ベースの積極的な breakpoint 探索
- `p95_one_second_breakpoint.js`
  - `p95 < 1s` を維持できる最大 req/s を探す staged test

### 結果を見るときの主な観点
- `http_req_failed`
- `http_req_duration` の p50 / p95 / p99
- max latency spike
- `/metrics`
- `docker stats`
- `pg_stat_activity`
- `pg_stat_statements`
- 詳細な実行コマンドと最新スクリプト: [tests/stress/README.md](./tests/stress/README.md)
