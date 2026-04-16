# 日本の青果データ API

[English README](./README.md)

このプロジェクトは、日本の青果卸売データを収集し、正規化し、PostgreSQL に保存し、クエリ API を通じて提供します。

## プロジェクトの目標
- 公開されている日本の青果 CSV データを確実に収集する
- 正規化 + 取り込みパイプラインを構築する
- 分析およびデータ探索のための REST API を提供する
- 軽量な監視とスケジュールジョブで運用する

## 主なコンポーネント
- **Downloader**: 日付/期間の CSV 収集、Shift-JIS -> UTF-8 変換
- **Ingestor**: PostgreSQL 取り込みパイプライン
- **API Server**: 価格推移、比較、ランキング、カバレッジのクエリエンドポイント
- **Ops Pipeline**: Docker Compose + cron によるスケジュール自動化
- **Monitoring**: `/metrics` + CSV スナップショット + ダッシュボード

## データソースと帰属
- ソースサイト: MAFF 卸売市場データポータル（日本）
  - https://www.seisen.maff.go.jp/seisen/bs04b040md001/BS04B040UC010SC999-Evt001.do
- このプロジェクトは、そのポータルから公開アクセス可能な CSV データを取得し、変換します。
- ソースドメイン `seisen.maff.go.jp` は、日本の青果・野菜市場の卸売価格データを対象とする MAFF の卸売市場 / 青果市場データソースです。
- このリポジトリは、日本の卸売青果データ、市場価格履歴、品目レベルの推移データ、および CSV ベースの市場データワークフローを探しているユーザーを想定しています。
- データの権利および利用条件は元の提供者に帰属します。
- このリポジトリは独立した実装であり、MAFF とは提携していません。

## 技術スタック
- Go
- PostgreSQL
- Docker
- Nginx

## アーキテクチャ（高レベル）
1. `downloader` が生の CSV ファイルを取得し、UTF-8 に変換します
2. `ingestor` が正規化されたデータを PostgreSQL に読み込みます（ディメンション / ファクトモデル）
3. `api` がクエリおよび監視エンドポイントを公開します
4. `pipeline-cron` がスケジュールされた取り込みと監視スナップショットを実行します

## クイックスタート
```bash
cp .env.example .env
```

`.env` に必須の値を設定してください:
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `POSTGRES_DB`

実行:
```bash
docker compose up -d --build
docker compose ps
```

デフォルトのローカルベース URL:
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

データベース / API SQL リファレンス:
```text
./plan/DB_API_SQL_REFERENCE.md
```

管理アクセス方針:
```text
/doc
/doc-llm
/ready
/metrics
/monitoring/dashboard
/monitoring/snapshots.csv
/ingestion/runs
/ingestion/files
これらのルートは管理者または内部アクセスのみに制限する必要があります。
このプロジェクトの本番デプロイでは、Cloudflare と Nginx リバースプロキシの両方で追加のアクセス制御とレート制限を適用しています。
```

## API のハイライト
- Health/Ready/Metrics
  - `GET /health`
  - `GET /ready`
  - `GET /metrics`
- API ドキュメント
  - `GET /doc`
  - `GET /doc-llm`
- カバレッジ / 最新データ
  - `GET /v1/coverage`
  - `GET /v1/prices/latest`
- ディメンション参照
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

## エンドポイント動作概要

### システムと監視
- `GET /health`: ライブネスチェックで、`{ "data": { "status": "ok" } }` を返します
- `GET /ready`: キャッシュされたデータベース ping 結果に基づくレディネスチェック
- `GET /metrics`: Prometheus のプレーンテキストメトリクス
- `GET /monitoring/dashboard`: 監視ダッシュボード HTML
- `GET /monitoring/snapshots.csv`: 設定されたスナップショットパスからの監視 CSV スナップショットファイル

### ディメンション API
- `GET /v1/markets`, `GET /v1/items`, `GET /v1/origins`
  - 目的:
    - `markets`: fact/aggregation API で厳密コードフィルタを使う前に有効な市場コードを見つける
    - `items`: daily/latest/trend/summary/ranking クエリの前に有効な品目コードを見つける
    - `origins`: price エンドポイントで厳密な産地フィルタを使う前に有効な産地コードを見つける
  - クエリ: `limit`（デフォルト `50`、最大 `500`）、`offset`, `q`, `sort`, `order`
  - `q` はコード / 名前を `LIKE` で検索します
  - レスポンス `meta`: `limit`, `offset`, `total`

### カバレッジ API
- `GET /v1/coverage`
  - `earliest_trade_date`, `latest_trade_date`, `fact_rows_total` を返します
  - 利用可能な場合は最新の取り込みメタデータも含みます:
    - `last_ingestion_run_id`
    - `last_ingestion_status`
    - `last_ingestion_run_type`
    - `last_ingestion_finished_at`

### 価格 API
- `GET /v1/prices/daily`
  - 必須: `item_code`
  - 任意: `date`, `from`, `to`, `market_code`, `origin_code`, `limit`, `offset`, `sort`, `order`
  - 主要な price list/aggregate API の中で `date` をサポートするのはこのエンドポイントのみです
  - `date/from/to` をすべて省略した場合、`from = today - 31 days` のデフォルト最近期間フィルタが適用されます
  - レスポンス `meta`: 自動適用時の `limit`, `offset`, および `default_from` / `default_window_days`

- `GET /v1/prices/latest`
  - 必須: `item_code`
  - 任意: `market_code`, `origin_code`, `limit`, `offset`, `sort`, `order`
  - 非対応: `date`, `from`, `to`
  - 非日付フィルタに一致する最新の取引日の行のみを返します
  - レスポンス `meta`: `latest_trade_date`, `limit`, `offset`, `total`

- `GET /v1/prices/trend`
  - 必須: `item_code`
  - 任意: `from`, `to`, `market_code`, `origin_code`
  - `trade_date` ごとに集計します
  - レスポンス項目には `rows_count`, `avg_price_mid_yen`, `max_price_mid_yen`, `min_price_mid_yen`, `arrival_ton_sum` が含まれます
  - `date/from/to` をすべて省略した場合、`from = today - 31 days` のデフォルト最近期間フィルタが適用されます
  - プリセットルート:
    - `GET /v1/prices/trend/1m`
    - `GET /v1/prices/trend/6m`
    - `GET /v1/prices/trend/1y`

- `GET /v1/prices/summary`
  - 必須: `item_code`
  - 任意: `group_by=day|week|month`, `from`, `to`, `market_code`, `origin_code`
  - `period` ごとに集計します
  - レスポンス項目には `rows_count`, `avg_price_mid_yen`, `arrival_ton_sum` が含まれます
  - `date/from/to` をすべて省略した場合、`from = today - 31 days` のデフォルト最近期間フィルタが適用されます

### 比較とランキング API
- `GET /v1/compare/markets`
  - 必須: `date`, `item_code`
  - 任意: `metric=price_mid|arrival`, `order`
  - デフォルトの metric: `price_mid`

- `GET /v1/rankings/items`
  - 必須: `date`
  - 任意: `metric=arrival|price_mid`, `market_code`, `limit`, `order`
  - デフォルトの metric: `arrival`
  - `limit` のデフォルトは `50`、最大は `500`

### 取り込み管理 API
- `GET /ingestion/runs`
  - 任意: `limit`（デフォルト `50`、最大 `500`）、`offset`
  - `id DESC` 順の取り込み実行を返します

- `GET /ingestion/files`
  - 任意: `run_id`, `limit`（デフォルト `100`、最大 `1000`）、`offset`
  - `run_id` が指定された場合は正の整数である必要があります
  - `id DESC` 順の取り込みファイルを返します

## デプロイノート
- `japan-data-api` は API プロセスを継続稼働させるために `command: "api"` で実行する必要があります
- `japan-data-pipeline-cron` は cron をフォアグラウンドで維持するために `command: "start-cron.sh"` で実行する必要があります
- prebuilt イメージ使用時に `command` を省略すると、イメージのデフォルトコマンドが one-shot パイプラインスクリプトを実行し、コンテナは正常に終了します
- 公開デプロイでは `/monitoring`, `/metrics`, `/doc` に対してリバースプロキシ、IP 制限、または Cloudflare ルールを推奨します
- `/doc`, `/doc-llm`, `/monitoring/*`, `/metrics`, `/ready`, および取り込み関連ルートは管理者または内部ネットワークのみに制限してください
- 本番環境では、Nginx のアクセスルールに加えて Cloudflare のアクセス制御と burst 保護を使用しています

## パフォーマンス概要
- 計測した本番ターゲット:
  - `https://jp-vgfr-api.seungpyo.xyz`
- 計測時に使用した本番サイズ:
  - `1 CPU / 1 GB RAM`
- 現在の運用上の結論:
  - `p95 < 1s` 目標での最大スループットは約 `110 req/s`
  - 保守的な運用上限は `100 req/s`
- 計測方法:
  - `k6`
  - `tests/stress/p95_one_second_breakpoint.js`
  - キャッシュだけに偏らないように item/date rotation を有効化
- 補足:
  - `50 -> 60 -> 70 -> 80 -> 90 req/s` の段階実行では `p95 100ms` を大きく下回った
  - `50 -> 105 -> 110 -> 115 -> 120 req/s` の段階実行は `p95 978.79ms` で境界付近で終了した
  - `100 -> 120 -> 140 -> 160 -> 180 req/s` は `p95 1603.81ms` で目標を超過した

## データワークフロー
1. `downloader` が MAFF ポータルからソース CSV ファイルを取得します
2. ソースデータは Shift-JIS から UTF-8 に変換されます
3. ファイルは 1 つの日次ファイルにマージされます: `utf8/<YYYYMMDD>/all_<YYYYMMDD>.csv`
4. `ingestor` がそれらの UTF-8 ファイルを読み込み、PostgreSQL にロードします
5. 監視スナップショットは別途 `data/monitoring/csv/snapshots.csv` に書き込まれます

## データカバレッジ
- 現在確認されている最も古い保存済み取引日: `2021-04-08`
- カバレッジはソースの提供状況に依存し、非営業日と未公開日はスキップされます
- downloader はより広い履歴範囲を試行できますが、実際に保存されるのはソースポータルで公開されている日付のみです

## 環境変数
- `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`: データベース接続
- `POSTGRES_SSLMODE`, `POSTGRES_TIMEZONE`: PostgreSQL 接続の挙動
- `POSTGRES_MAX_OPEN_CONNS`, `POSTGRES_MAX_IDLE_CONNS`: DB プールサイズの制御
- `POSTGRES_CONN_MAX_LIFETIME`, `POSTGRES_CONN_MAX_IDLE_TIME`: DB 接続の再利用制御
- `HTTP_PORT`: コンテナ / プロセス内の API リッスンポート
- `DAILY_INGEST_SCHEDULE`: 日次ダウンロード + 取り込みの cron スケジュール
- `MONITOR_SNAPSHOT_SCHEDULE`: 監視 CSV スナップショットの cron スケジュール
- `API_BASE_URL`: `/ready`, `/metrics`, および取り込みステータスチェックのための監視スクリプト対象
- `MONITOR_OUT_DIR`, `SNAPSHOT_FILE`: 監視 CSV の出力先

## 運用上の挙動
- `no market table rows` は、祝日、週末、またはソースポータルで未公開の日付では正常である場合があります
- 対象日付に公開データがない場合、downloader はその日付をスキップし、その日付の取り込みもスキップされます
- 監視 CSV スナップショットは別の cron スケジュールで生成され、日次取り込みが成功する必要はありません
- `/v1/coverage` は、大きなテーブルに対する重い `COUNT(*)` スキャンを避けるため、`fact_rows_total` を PostgreSQL の統計ベース推定値として返します（正確な全件数ではありません）
- `/v1/prices/daily`, `/v1/prices/trend`, `/v1/prices/summary` は、`date/from/to` がすべて省略された場合、デフォルト最近期間フィルタ（`from = today - 31 days`）を適用します
- `/v1/prices/latest` は、要求された非日付フィルタに対する最新の利用可能な取引日の行を常に返し、`date`, `from`, `to` はサポートしません
- デフォルト最近期間フィルタが自動適用された場合、レスポンス `meta` には `default_from` と `default_window_days` が含まれます
- ソースデータに産地名 / 産地コードがない場合、ingestor はそれを `origin_code='UNKNOWN'` および `origin_name='不明'` に正規化します

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

downloader の例（1 つの UTF-8 CSV にマージしてソースファイルを削除）:
```bash
go run ./cmd/downloader -date 20260412 -out ./data/data_downloads
```

API 統合テストを実行:
```bash
RUN_API_INTEGRATION_TESTS=1 API_BASE_URL=http://localhost:8080 go test ./tests/unit/api -count=1
```

パイプライン統合テストを実行:
```bash
RUN_PIPELINE_INTEGRATION_TESTS=1 go test ./tests/unit/pipeline -count=1
```

`k6` でストレステストを実行:
```bash
k6 run tests/stress/baseline.js
k6 run tests/stress/step_read.js
k6 run tests/stress/mixed_read.js
k6 run tests/stress/p95_one_second_breakpoint.js
```

## プロジェクト構成
- `cmd/`: 実行可能エントリポイント
- `internal/`: ドメイン、ハンドラ、プラットフォームロジック
- `scripts/`: 運用スクリプト
- `docker/`: nginx と cron の設定
- `tests/unit/`: Go テストスイート
- `tests/stress/`: `k6` ストレステストスクリプトと結果アーティファクト

## ストレステスト
- ストレステストスクリプトは [tests/stress](./tests/stress) 配下にあります
- 結果 JSON ファイルは [tests/stress/result](./tests/stress/result) 配下に保存されます
- デフォルトの本番ターゲット:
  - `https://jp-vgfr-api.seungpyo.xyz`
- 最近のテストで使用した本番計測プロファイル:
  - API サーバー: `1 vCPU / 1GB RAM`
  - PostgreSQL サーバー: `1 vCPU / 1GB RAM`

### 利用可能な `k6` プロファイル
- `baseline.js`
  - 5 VUs を 2 分間
  - 低リスクのスモーク / ベースラインチェック
- `step_read.js`
  - 5 -> 10 -> 20 -> 30 VUs
  - 保守的で本番安全な段階的 read テスト
- `mixed_read.js`
  - 10 -> 15 -> 15 VUs
  - 取り込みを並行実行しながらの保守的な read 負荷
- `high_read_breakpoint.js`
  - 攻撃的な VU ベースのブレークポイント探索
- `p95_one_second_breakpoint.js`
  - `p95 < 1s` を維持できる最大スループットを探す req/s ベースの段階探索

### 結果分析時に確認するもの
- `http_req_failed`
- `http_req_duration` p50 / p95 / p99
- 最大レイテンシスパイク
- `/metrics`
- `docker stats`
- `pg_stat_activity`
- `pg_stat_statements`
- 詳細なコマンドと現在のスクリプト: [tests/stress/README.md](./tests/stress/README.md)

## アーキテクチャ図
```text
                        +-----------------------------+
                        | MAFF 卸売データポータル          |
                        | seisen.maff.go.jp           |
                        +-------------+---------------+
                                      |
                                      | CSV / HTML fetch
                                      v
                    +---------------------------------------+
                    | downloader                            |
                    | - 日付 / 期間取得                         |
                    | - Shift-JIS -> UTF-8 変換              |
                    | - 日次マージ済み CSV 出力                  |
                    +-------------------+-------------------+
                                        |
                                        | utf8/<YYYYMMDD>/all_<YYYYMMDD>.csv
                                        v
                    +---------------------------------------+
                    | ingestor                              |
                    | - 解析 + 検証                           |
                    | - ディメンション upsert                   |
                    | - fact stage copy                     |
                    | - source_file,row_no で fact merge     |
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

## この設計の理由

### downloader と ingestor を分ける理由
- 上流の CSV 取得と DB 取り込みは失敗理由が異なるため、分離することでリトライと診断が簡単になります。
- UTF-8 に正規化されたファイルをディスクに保持することで、デバッグや再取り込みのための再現可能な中間成果物が得られます。
- downloader は未公開日をスキップでき、DB 書き込みロジックを強制的に実行させません。

### ディメンション + ファクトテーブルを使う理由
- クエリ API は市場、品目、産地、日付で繰り返しフィルタするため、ディメンション / ファクトモデルは最大のテーブルから繰り返し文字列を排除できます。
- `market_code`, `item_code`, `origin_code` のような安定したコードは、ディメンション参照と厳密フィルタに自然に対応します。
- ファクトテーブルがすでに正規化済みの数値フィールドを保持するため、集計エンドポイントが単純になります。

### 取り込みメタデータテーブルを保持する理由
- `ingestion_runs` と `ingestion_files` は、ステータスをログだけから推測する代わりに、運用者に第一級の監査証跡を提供します。
- `/v1/coverage` は、ログファイルやコンテナ状態を読まずに最近の取り込みステータスを公開できます。

### 生データ API と集計 API の両方を公開する理由
- `daily` はデバッグとエクスポートをサポートします。
- `latest` は現在状態のダッシュボードをサポートします。
- `trend` と `summary` は時系列分析とロールアップ分析をサポートします。
- `compare` と `rankings` は、クライアントに複雑な SQL を組み立てさせることなく、プロダクト向けの探索ビューをサポートします。

## DB スキーマとインデックス設計

### スキーマ概要
- `dim_market`: 市場コード / 名前マスタ
- `dim_item`: 品目コード / 名前マスタ
- `dim_origin`: 産地コード / 名前マスタ
- `dim_grade`: 等級 / 階級 / 品種 / 単位重量の複合ディメンション
- `fact_prices_daily`: 正規化された日次卸売ファクト行
- `ingestion_runs`, `ingestion_files`: 取り込み監査メタデータ

### これらのキーを使う理由
- `dim_market.market_code`, `dim_item.item_code`, `dim_origin.origin_code` は、コードが API の正規フィルタキーであるため一意です。
- `dim_grade` は、単一のソースコードで等級 / 階級 / 品種 / 単位重量の組み合わせを識別できないため、複合一意キーを使います。
- `fact_prices_daily` は、上流ソースで最も安定した行識別子に近いため、`source_file + source_row_no` を一意キーとして使います。

### これらの fact インデックスを使う理由
- `idx_fact_trade_market (trade_date, market_id)`
  - 日付先頭のスキャンと、単一日の市場比較をサポートします。
- `idx_fact_trade_item_market (trade_date, item_id, market_id)`
  - 市場グルーピングが重要な日付 + 品目フィルタ、および compare/ranking パターンをサポートします。
- `idx_fact_item_trade (item_id, trade_date)`
  - `item_code` が必須で日付範囲が一般的な `daily`, `trend`, `summary` をサポートします。
- `idx_fact_origin_trade (origin_id, trade_date)`
  - 日付によるフルスキャンを要求せずに産地フィルタ付きクエリをサポートします。

### すべてを非正規化して保存しない理由
- 一部の読み取りは単純になりますが、ファクトテーブルははるかに幅広くなり、重複も増えます。
- 市場 / 品目 / 産地名の繰り返しは、保存容量と更新コストを増加させます。
- 厳密なコード参照 API は、それでもどこかで正規のコード / 名前管理を必要とします。

### カバレッジに正確な `COUNT(*)` を使わない理由
- `/v1/coverage` は軽量な運用エンドポイントを意図しています。
- 大きなファクトテーブルに対する全件カウントは高コストであり、粗いカバレッジ指標には不要です。
- `pg_class.reltuples` を通じた PostgreSQL 統計は、意図的なレイテンシ対精度のトレードオフです。

## 障害、例外、および運用戦略

### 上流データの例外
- ソースポータルは、週末、祝日、または一部の日付についてデータを公開しないことがあります。
- downloader は `no market table rows` を未公開日の正常なスキップ条件として扱います。
- したがって履歴の完全性は、ローカル保存だけでなく上流の公開状況によって制限されます。

### 取り込み時の例外
- CSV 行は、必須カラム、数値フィールド、非負値に対する検証付きで解析されます。
- 不正な行は、デフォルトではシステム全体を停止させる代わりに `rows_error` に計上されます。
- ファイル単位またはグループ単位の失敗は、後で確認できるよう取り込みステータステーブルに記録されます。

### 冪等性とリトライ挙動
- マージでは `ON CONFLICT (source_file, source_row_no)` を使用するため、同じソースファイルを再取り込みしても fact は重複しません。
- fact マージの前にディメンションテーブルが upsert されるため、再実行間でも参照 ID は一貫します。
- トランザクションレベルの再試行可能なエラーは、リトライ処理のために明示的に認識されます。

### 運用上の保護
- 管理ルートと監視ルートは、Nginx、Cloudflare、またはネットワークポリシーで保護される想定です。
- 監視スナップショットは日次取り込みとは独立して生成されるため、取り込みが失敗しても可観測性を継続できます。
- `/ready` はデータベース可用性を確認し、`/metrics` はアラートのためのリクエストおよび DB エラーシグナルを公開します。

### よくある運用者アクション
- 取り込み後に `/v1/coverage.fact_rows_total` が古く見える場合は、`ANALYZE fact_prices_daily;` を実行してください。
- ソースの産地が欠落している場合、ingestor はそれを `origin_code='UNKNOWN'` および `origin_name='不明'` に正規化します。
- `/ingestion/runs` と `/ingestion/files` を使って、全体実行の失敗と部分的なファイル失敗を区別してください。

## テスト戦略

### テストレイヤー
- 単体テストは、ハンドラ / ヘルパーの挙動とバリデーションロジックを対象とします。
- API 統合テストは、実行中の API に対するライブエンドポイントの挙動を検証します。
- パイプライン統合テストは、DB 連携パイプラインに対する downloader/ingestor の挙動を検証します。

### 環境変数で統合テストをゲートする理由
- API およびパイプライン統合テストには、実行中のサービスまたは DB 連携環境が必要です。
- `RUN_API_INTEGRATION_TESTS` と `RUN_PIPELINE_INTEGRATION_TESTS` でゲートすることで、通常のローカル `go test ./...` を高速かつ予測可能に保てます。

### テストが証明しようとしていること
- ハンドラ契約が文書化されたレスポンス形状とステータスコードに一致すること。
- クエリパラメータのバリデーションが正しく動作すること。
- 取り込みが、一意性やメタデータ記録の前提を壊さずに正規化データを書き込むこと。
- 実際のサービス境界に対して実行したときに、エンドツーエンドのフローが安定していること。

## 監視メトリクスとシグナル

### 主要な可観測性インターフェース
- `GET /health`: プロセスのライブネス
- `GET /ready`: DB 連携レディネス
- `GET /metrics`: Prometheus メトリクス
- `GET /monitoring/dashboard`: 軽量な運用者 UI
- `GET /monitoring/snapshots.csv`: 低コスト監視のためのスナップショット履歴

### 有用なメトリクス / シグナル
- パス / メソッド / ステータス別 HTTP リクエスト総数
- 5xx エラー数
- DB エラー総数
- p95 スタイルの監視ビューを含むリクエストレイテンシ
- 最新の取り込みステータス
- レディネスの成功 / 失敗

### Prometheus メトリクスに加えてスナップショットが存在する理由
- CSV スナップショットは、完全なメトリクスバックエンドがなくてもアーカイブ、確認、差分比較が簡単です。
- 最小構成のデプロイでも軽量な運用履歴を提供します。
- 粗い運用可視性をライブスクレイプ可用性から切り離します。

## 実際の API 例

### カバレッジと最新の取り込み
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

### ディメンション参照
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

### 生の fact 行
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

### 推移集計
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

### ランキングビュー
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

## トレードオフ

### この設計が最適化していること
- 運用の単純さ
- 再現可能な取り込み成果物
- 摩擦の少ない分析 API
- わかりやすい SQL とデバッグ

### 受け入れているトレードオフ
- アプリ自体には認証ミドルウェアがありません。
  - アプリコードは単純になりますが、セキュリティはリバースプロキシとネットワーク制御に依存します。
- 重いウェアハウス層やサマリーテーブルはありません。
  - 書き込みは単純で可動部も少ない一方、一部の正確な大規模分析は意図的に避けています。
- カバレッジ件数は推定値であり、正確ではありません。
  - 運用エンドポイントは高速になりますが、精度は下がります。
- 一部のエンドポイントには最近期間のデフォルトがあります。
  - より安全で低コストなデフォルトクエリになりますが、完全な履歴分析には明示的な範囲指定が必要だとユーザーが理解している必要があります。
- 幅広い非正規化 fact テーブルではなく、ディメンション / fact 正規化を採用しています。
  - 一貫性とストレージ効率は向上しますが、読み取り時に join が必要です。

## 制限事項
- downloader は上流の MAFF HTML 構造と CSV 公開フローに依存しており、上流の変更によってパーサ更新が必要になる場合があります
- 履歴の完全性は、ローカル保存だけでなくソースの公開状況にも制限されます
- 大きな fact テーブルに対する正確な全件分析は、サマリーテーブルやフィルタ付きクエリなしでは高コストになる場合があります

## ライセンス
MIT License。詳細は [LICENSE](./LICENSE) を参照してください。
