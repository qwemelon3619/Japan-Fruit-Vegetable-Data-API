package query

import "time"

// ============================================================================
// Coverage
// ============================================================================

// CoverageResult holds the coverage query result.
type CoverageResult struct {
	EarliestTradeDate    *string
	LatestTradeDate      *string
	FactRowsTotal        int64
	LastIngestionRunID   *uint
	LastIngestionStatus  *string
	LastIngestionRunType *string
	LastIngestionAt      *time.Time
}

// ============================================================================
// Dimensions
// ============================================================================

// DimensionQuery holds common dimension list query parameters.
type DimensionQuery struct {
	Kind   string // "market", "item", or "origin"
	Limit  int
	Offset int
	Q      string
	Sort   string
	Order  string
}

// DimensionListResult holds a paginated dimension query result.
type DimensionListResult struct {
	Rows  []DimensionRow
	Total int64
	Limit int
	Off   int
}

// DimensionRow is a dimension lookup result row.
type DimensionRow struct {
	ID   uint   `json:"id" gorm:"column:id"`
	Code string `json:"code" gorm:"column:code"`
	Name string `json:"name" gorm:"column:name"`
}

// ============================================================================
// Prices — shared filter types
// ============================================================================

// DateRange holds optional date filter parameters.
type DateRange struct {
	Date string // exact date YYYY-MM-DD (daily only)
	From string // range start YYYY-MM-DD
	To   string // range end YYYY-MM-DD
}

// PriceFilter holds common price query filter parameters.
type PriceFilter struct {
	ItemCode   string
	MarketCode string
	OriginCode string
	DateRange  DateRange
	Limit      int
	Offset     int
	Sort       string
	Order      string
}

// ============================================================================
// Prices — daily rows
// ============================================================================

// DailyRow is a daily price fact row.
type DailyRow struct {
	TradeDate   string   `json:"trade_date" gorm:"column:trade_date"`
	WeekdayJA   string   `json:"weekday_ja" gorm:"column:weekday_ja"`
	MarketCode  string   `json:"market_code" gorm:"column:market_code"`
	MarketName  string   `json:"market_name" gorm:"column:market_name"`
	ItemCode    string   `json:"item_code" gorm:"column:item_code"`
	ItemName    string   `json:"item_name" gorm:"column:item_name"`
	OriginCode  string   `json:"origin_code" gorm:"column:origin_code"`
	OriginName  string   `json:"origin_name" gorm:"column:origin_name"`
	Grade       string   `json:"grade" gorm:"column:grade"`
	Class       string   `json:"class" gorm:"column:class"`
	ProductName string   `json:"product_name" gorm:"column:product_name"`
	UnitWeight  string   `json:"unit_weight" gorm:"column:unit_weight"`
	ItemTotal   *float64 `json:"item_total_ton" gorm:"column:item_total_ton"`
	ArrivalTon  *float64 `json:"arrival_ton" gorm:"column:arrival_ton"`
	PriceHigh   *int     `json:"price_high_yen" gorm:"column:price_high_yen"`
	PriceMid    *int     `json:"price_mid_yen" gorm:"column:price_mid_yen"`
	PriceLow    *int     `json:"price_low_yen" gorm:"column:price_low_yen"`
	TrendLabel  *string  `json:"trend_label" gorm:"column:trend_label"`
	SourceFile  string   `json:"source_file" gorm:"column:source_file"`
	SourceRowNo int      `json:"source_row_no" gorm:"column:source_row_no"`
}

// DailyRowWithTotal extends DailyRow with total count and latest date for pagination.
type DailyRowWithTotal struct {
	DailyRow        `gorm:"embedded"`
	TotalCount      int64    `gorm:"column:total_count"`
	LatestTradeDate *string  `gorm:"column:latest_trade_date"`
}

// DailyPricesResult holds the result of a daily prices query.
type DailyPricesResult struct {
	Rows              []DailyRow
	Total             int64
	Limit             int
	Offset            int
	DefaultFrom       *string
	DefaultWindowDays int
}

// LatestPricesResult holds the result of a latest prices query.
type LatestPricesResult struct {
	Rows            []DailyRow
	LatestTradeDate *string
	Limit           int
	Offset          int
	Total           int64
}

// ============================================================================
// Prices — trend rows
// ============================================================================

// TrendRow is a price trend aggregation row.
type TrendRow struct {
	TradeDate   string   `json:"trade_date" gorm:"column:trade_date"`
	RowsCount   int64    `json:"rows_count" gorm:"column:rows_count"`
	AvgPriceMid *float64 `json:"avg_price_mid_yen" gorm:"column:avg_price_mid_yen"`
	MaxPriceMid *int     `json:"max_price_mid_yen" gorm:"column:max_price_mid_yen"`
	MinPriceMid *int     `json:"min_price_mid_yen" gorm:"column:min_price_mid_yen"`
	ArrivalTon  *float64 `json:"arrival_ton_sum" gorm:"column:arrival_ton_sum"`
}

// TrendResult holds the result of a price trend query.
type TrendResult struct {
	Rows              []TrendRow
	DefaultFrom       *string
	DefaultWindowDays int
}

// SummaryRow is a price summary aggregation row.
type SummaryRow struct {
	Period      string   `json:"period" gorm:"column:period"`
	RowsCount   int64    `json:"rows_count" gorm:"column:rows_count"`
	AvgPriceMid *float64 `json:"avg_price_mid_yen" gorm:"column:avg_price_mid_yen"`
	ArrivalTon  *float64 `json:"arrival_ton_sum" gorm:"column:arrival_ton_sum"`
}

// SummaryResult holds the result of a price summary query.
type SummaryResult struct {
	Rows              []SummaryRow
	GroupBy           string
	DefaultFrom       *string
	DefaultWindowDays int
}

// ============================================================================
// Analysis — compare markets
// ============================================================================

// CompareMarketRow holds a single market comparison result row.
type CompareMarketRow struct {
	MarketCode  string   `json:"market_code" gorm:"column:market_code"`
	MarketName  string   `json:"market_name" gorm:"column:market_name"`
	RowsCount   int64    `json:"rows_count" gorm:"column:rows_count"`
	MetricValue *float64 `json:"metric_value" gorm:"column:metric_value"`
}

// CompareMarketsResult holds the result of a compare markets query.
type CompareMarketsResult struct {
	Rows   []CompareMarketRow
	Metric string
	Total  int
}

// RankingItemRow holds a single ranking item result row.
type RankingItemRow struct {
	ItemCode    string   `json:"item_code" gorm:"column:item_code"`
	ItemName    string   `json:"item_name" gorm:"column:item_name"`
	RowsCount   int64    `json:"rows_count" gorm:"column:rows_count"`
	MetricValue *float64 `json:"metric_value" gorm:"column:metric_value"`
}

// RankItemsResult holds the result of a rank items query.
type RankItemsResult struct {
	Rows   []RankingItemRow
	Metric string
	Limit  int
	Total  int
}

// ============================================================================
// Ingestion
// ============================================================================

// IngestionRunRow holds a single ingestion run result row.
type IngestionRunRow struct {
	ID           uint       `json:"id" gorm:"column:id"`
	RunType      string     `json:"run_type" gorm:"column:run_type"`
	StartedAt    time.Time  `json:"started_at" gorm:"column:started_at"`
	FinishedAt   *time.Time `json:"finished_at" gorm:"column:finished_at"`
	Status       string     `json:"status" gorm:"column:status"`
	ErrorMessage *string    `json:"error_message" gorm:"column:error_message"`
	CreatedAt    time.Time  `json:"created_at" gorm:"column:created_at"`
	UpdatedAt    time.Time  `json:"updated_at" gorm:"column:updated_at"`
}

// IngestionRunsResult holds the paginated result of a list ingestion runs query.
type IngestionRunsResult struct {
	Rows   []IngestionRunRow
	Limit  int
	Offset int
	Total  int64
}

// IngestionFileRow holds a single ingestion file result row.
type IngestionFileRow struct {
	ID        uint      `json:"id" gorm:"column:id"`
	RunID     uint      `json:"run_id" gorm:"column:run_id"`
	FilePath  string    `json:"file_path" gorm:"column:file_path"`
	FileHash  string    `json:"file_hash" gorm:"column:file_hash"`
	RowsTotal int       `json:"rows_total" gorm:"column:rows_total"`
	RowsOK    int       `json:"rows_ok" gorm:"column:rows_ok"`
	RowsError int       `json:"rows_error" gorm:"column:rows_error"`
	Status    string    `json:"status" gorm:"column:status"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// IngestionFilesResult holds the paginated result of a list ingestion files query.
type IngestionFilesResult struct {
	Rows   []IngestionFileRow
	Limit  int
	Offset int
	Total  int64
}

// ============================================================================
// Internal helpers for DB scanning
// ============================================================================

// internalCoverageRow is used for scanning the coverage query result.
type internalCoverageRow struct {
	EarliestTradeDate *string `gorm:"column:earliest_trade_date"`
	LatestTradeDate   *string `gorm:"column:latest_trade_date"`
	FactRowsTotal     int64   `gorm:"column:fact_rows_total"`
}
