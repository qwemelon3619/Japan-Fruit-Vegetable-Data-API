package v1

import "time"

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiMeta map[string]any

type dimensionRow struct {
	ID   uint   `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type coverageRow struct {
	EarliestTradeDate    *string    `json:"earliest_trade_date"`
	LatestTradeDate      *string    `json:"latest_trade_date"`
	FactRowsTotal        int64      `json:"fact_rows_total"`
	LastIngestionRunID   *uint      `json:"last_ingestion_run_id"`
	LastIngestionStatus  *string    `json:"last_ingestion_status"`
	LastIngestionRunType *string    `json:"last_ingestion_run_type"`
	LastIngestionAt      *time.Time `json:"last_ingestion_finished_at"`
}

type dailyRow struct {
	TradeDate   string   `json:"trade_date"`
	WeekdayJA   string   `json:"weekday_ja"`
	MarketCode  string   `json:"market_code"`
	MarketName  string   `json:"market_name"`
	ItemCode    string   `json:"item_code"`
	ItemName    string   `json:"item_name"`
	OriginCode  string   `json:"origin_code"`
	OriginName  string   `json:"origin_name"`
	Grade       string   `json:"grade"`
	Class       string   `json:"class"`
	ProductName string   `json:"product_name"`
	UnitWeight  string   `json:"unit_weight"`
	ItemTotal   *float64 `json:"item_total_ton"`
	ArrivalTon  *float64 `json:"arrival_ton"`
	PriceHigh   *int     `json:"price_high_yen"`
	PriceMid    *int     `json:"price_mid_yen"`
	PriceLow    *int     `json:"price_low_yen"`
	TrendLabel  *string  `json:"trend_label"`
	SourceFile  string   `json:"source_file"`
	SourceRowNo int      `json:"source_row_no"`
}

type dailyRowWithTotal struct {
	DailyRow        dailyRow `gorm:"embedded"`
	TotalCount      int64    `gorm:"column:total_count"`
	LatestTradeDate *string  `gorm:"column:latest_trade_date"`
}

type trendRow struct {
	TradeDate   string   `json:"trade_date" gorm:"column:trade_date"`
	RowsCount   int64    `json:"rows_count" gorm:"column:rows_count"`
	AvgPriceMid *float64 `json:"avg_price_mid_yen" gorm:"column:avg_price_mid_yen"`
	MaxPriceMid *int     `json:"max_price_mid_yen" gorm:"column:max_price_mid_yen"`
	MinPriceMid *int     `json:"min_price_mid_yen" gorm:"column:min_price_mid_yen"`
	ArrivalTon  *float64 `json:"arrival_ton_sum" gorm:"column:arrival_ton_sum"`
}

type summaryRow struct {
	Period      string   `json:"period"`
	RowsCount   int64    `json:"rows_count"`
	AvgPriceMid *float64 `json:"avg_price_mid_yen"`
	ArrivalTon  *float64 `json:"arrival_ton_sum"`
}

type compareMarketRow struct {
	MarketCode string   `json:"market_code" gorm:"column:market_code"`
	MarketName string   `json:"market_name" gorm:"column:market_name"`
	RowsCount  int64    `json:"rows_count" gorm:"column:rows_count"`
	Metric     *float64 `json:"metric_value" gorm:"column:metric_value"`
}

type rankingItemRow struct {
	ItemCode  string   `json:"item_code" gorm:"column:item_code"`
	ItemName  string   `json:"item_name" gorm:"column:item_name"`
	RowsCount int64    `json:"rows_count" gorm:"column:rows_count"`
	Metric    *float64 `json:"metric_value" gorm:"column:metric_value"`
}

type ingestionRunRow struct {
	ID           uint       `json:"id"`
	RunType      string     `json:"run_type"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	Status       string     `json:"status"`
	ErrorMessage *string    `json:"error_message"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ingestionFileRow struct {
	ID        uint      `json:"id"`
	RunID     uint      `json:"run_id"`
	FilePath  string    `json:"file_path"`
	FileHash  string    `json:"file_hash"`
	RowsTotal int       `json:"rows_total"`
	RowsOK    int       `json:"rows_ok"`
	RowsError int       `json:"rows_error"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
