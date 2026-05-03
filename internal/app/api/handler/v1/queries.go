package v1

import (
	"context"
	"time"

	"japan_data_project/internal/domain/model"
)

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

// ---- Price query shared types ----

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

// TrendResult holds the result of a price trend query.
type TrendResult struct {
	Rows              []TrendRow
	DefaultFrom       *string
	DefaultWindowDays int
}

// SummaryResult holds the result of a price summary query.
type SummaryResult struct {
	Rows              []SummaryRow
	GroupBy           string
	DefaultFrom       *string
	DefaultWindowDays int
}

// buildPriceFilterClauses builds SQL WHERE clause and args from a PriceFilter.
// ids must already be resolved from ResolveCodeIDs.
// includeSingleDate controls whether the "date" field is used.
func buildPriceFilterClauses(filter PriceFilter, ids *latestFilterIDs, includeSingleDate bool) (string, []any, *string, error) {
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 8)
	var defaultFrom *string

	dr := filter.DateRange
	if includeSingleDate && dr.Date != "" {
		if _, err := mustParseDate(dr.Date, "date"); err != nil {
			return "", nil, nil, err
		}
		clauses = append(clauses, "f.trade_date = ?")
		args = append(args, dr.Date)
	}

	var fromDate, toDate *time.Time
	if dr.From != "" {
		d, err := mustParseDate(dr.From, "from")
		if err != nil {
			return "", nil, nil, err
		}
		fromDate = &d
		clauses = append(clauses, "f.trade_date >= ?")
		args = append(args, dr.From)
	}
	if dr.To != "" {
		d, err := mustParseDate(dr.To, "to")
		if err != nil {
			return "", nil, nil, err
		}
		toDate = &d
		clauses = append(clauses, "f.trade_date <= ?")
		args = append(args, dr.To)
	}
	if err := validateDateRange(fromDate, toDate); err != nil {
		return "", nil, nil, err
	}

	if dr.Date == "" && fromDate == nil && toDate == nil {
		df := time.Now().AddDate(0, 0, -defaultRecentDays).Format("2006-01-02")
		defaultFrom = &df
		clauses = append(clauses, "f.trade_date >= ?")
		args = append(args, df)
	}

	if ids != nil && ids.ItemID != nil {
		clauses = append(clauses, "f.item_id = ?")
		args = append(args, *ids.ItemID)
	}
	if ids != nil && ids.MarketID != nil {
		clauses = append(clauses, "f.market_id = ?")
		args = append(args, *ids.MarketID)
	}
	if ids != nil && ids.OriginID != nil {
		clauses = append(clauses, "f.origin_id = ?")
		args = append(args, *ids.OriginID)
	}

	if len(clauses) == 0 {
		return "", args, defaultFrom, nil
	}
	return "WHERE " + joinClauses(clauses, " AND "), args, defaultFrom, nil
}

func joinClauses(clauses []string, sep string) string {
	if len(clauses) == 0 {
		return ""
	}
	result := clauses[0]
	for _, c := range clauses[1:] {
		result += sep + c
	}
	return result
}

// ---- Price Query Methods ----

// GetDailyPrices returns paginated daily price rows.
func (s *Service) GetDailyPrices(ctx context.Context, filter PriceFilter) (*DailyPricesResult, error) {
	limit := clampInt(filter.Limit, 1, 2000)
	offset := maxInt(filter.Offset, 0)

	ids, err := s.resolveCodeIDs(ctx, filter.ItemCode, filter.MarketCode, filter.OriginCode)
	if err != nil {
		return nil, err
	}
	if ids == nil || ids.ItemID == nil {
		return &DailyPricesResult{Rows: []DailyRow{}, Limit: limit, Offset: offset}, nil
	}

	whereSQL, whereArgs, defaultFrom, err := buildPriceFilterClauses(filter, ids, true)
	if err != nil {
		return nil, err
	}

	sort := parseDailySortColumn(parseDailySort(filter.Sort))
	order := parseOrder(filter.Order)
	query := `
WITH filtered_fact AS (
	SELECT f.id, f.trade_date, f.weekday_ja, f.market_id, f.item_id, f.origin_id,
		f.grade_id, f.item_total_ton, f.arrival_ton, f.price_high_yen,
		f.price_mid_yen, f.price_low_yen, f.trend_label, f.source_file, f.source_row_no
	FROM fact_prices_daily f
	` + whereSQL + `
)
SELECT
	to_char(ff.trade_date, 'YYYY-MM-DD') AS trade_date, ff.weekday_ja,
	m.market_code, m.market_name, i.item_code, i.item_name,
	o.origin_code, o.origin_name, g.grade, g.class, g.product_name, g.unit_weight,
	ff.item_total_ton, ff.arrival_ton, ff.price_high_yen, ff.price_mid_yen,
	ff.price_low_yen, ff.trend_label, ff.source_file, ff.source_row_no
FROM filtered_fact ff
JOIN dim_market m ON m.id = ff.market_id
JOIN dim_item i ON i.id = ff.item_id
JOIN dim_origin o ON o.id = ff.origin_id
JOIN dim_grade g ON g.id = ff.grade_id
ORDER BY ` + sort + ` ` + order + `
LIMIT ? OFFSET ?`

	pageRows := make([]DailyRowWithTotal, 0, limit)
	if err := s.observeDB("prices_daily_page", func() error {
		return s.db.WithContext(ctx).Raw(query, append(whereArgs, limit, offset)...).Scan(&pageRows).Error
	}); err != nil {
		return nil, err
	}

	rows := make([]DailyRow, 0, len(pageRows))
	for i := range pageRows {
		row := pageRows[i].DailyRow
		row.ItemTotal = roundFloatPtr2(row.ItemTotal)
		row.ArrivalTon = roundFloatPtr2(row.ArrivalTon)
		rows = append(rows, row)
	}

	wd := defaultRecentDays
	return &DailyPricesResult{
		Rows: rows, Limit: limit, Offset: offset,
		DefaultFrom: defaultFrom, DefaultWindowDays: wd,
	}, nil
}

// GetLatestPrices returns prices for the latest available trade date.
func (s *Service) GetLatestPrices(ctx context.Context, filter PriceFilter) (*LatestPricesResult, error) {
	limit := clampInt(filter.Limit, 1, 2000)
	offset := maxInt(filter.Offset, 0)

	ids, err := s.resolveCodeIDs(ctx, filter.ItemCode, filter.MarketCode, filter.OriginCode)
	if err != nil {
		return nil, err
	}
	if ids == nil || ids.ItemID == nil {
		return &LatestPricesResult{Rows: []DailyRow{}, Limit: limit, Offset: offset}, nil
	}

	// Find latest trade date
	var latestTradeDate *string
	latestDateQuery := "SELECT to_char(MAX(trade_date), 'YYYY-MM-DD') AS latest_trade_date FROM fact_prices_daily WHERE item_id = ?"
	latestDateArgs := []any{*ids.ItemID}
	if ids.MarketID != nil {
		latestDateQuery += " AND market_id = ?"
		latestDateArgs = append(latestDateArgs, *ids.MarketID)
	}
	if ids.OriginID != nil {
		latestDateQuery += " AND origin_id = ?"
		latestDateArgs = append(latestDateArgs, *ids.OriginID)
	}
	if err := s.observeDB("prices_latest_trade_date", func() error {
		return s.db.WithContext(ctx).Raw(latestDateQuery, latestDateArgs...).Scan(&latestTradeDate).Error
	}); err != nil {
		return nil, err
	}
	if latestTradeDate == nil || *latestTradeDate == "" {
		return &LatestPricesResult{Rows: []DailyRow{}, Limit: limit, Offset: offset}, nil
	}

	// Build filter for latest date
	whereClauses := []string{"f.trade_date = ?", "f.item_id = ?"}
	whereArgs := []any{*latestTradeDate, *ids.ItemID}
	if ids.MarketID != nil {
		whereClauses = append(whereClauses, "f.market_id = ?")
		whereArgs = append(whereArgs, *ids.MarketID)
	}
	if ids.OriginID != nil {
		whereClauses = append(whereClauses, "f.origin_id = ?")
		whereArgs = append(whereArgs, *ids.OriginID)
	}
	whereSQL := "WHERE " + joinClauses(whereClauses, " AND ")

	sort := parseDailySortColumn(parseDailySort(filter.Sort))
	order := parseOrder(filter.Order)
	query := `
WITH filtered_fact AS (
	SELECT f.id, f.trade_date, f.weekday_ja, f.market_id, f.item_id, f.origin_id,
		f.grade_id, f.item_total_ton, f.arrival_ton, f.price_high_yen,
		f.price_mid_yen, f.price_low_yen, f.trend_label, f.source_file, f.source_row_no
	FROM fact_prices_daily f
	` + whereSQL + `
)
SELECT
	to_char(ff.trade_date, 'YYYY-MM-DD') AS trade_date, ff.weekday_ja,
	m.market_code, m.market_name, i.item_code, i.item_name,
	o.origin_code, o.origin_name, g.grade, g.class, g.product_name, g.unit_weight,
	ff.item_total_ton, ff.arrival_ton, ff.price_high_yen, ff.price_mid_yen,
	ff.price_low_yen, ff.trend_label, ff.source_file, ff.source_row_no,
	COUNT(*) OVER() AS total_count
FROM filtered_fact ff
JOIN dim_market m ON m.id = ff.market_id
JOIN dim_item i ON i.id = ff.item_id
JOIN dim_origin o ON o.id = ff.origin_id
JOIN dim_grade g ON g.id = ff.grade_id
ORDER BY ` + sort + ` ` + order + `
LIMIT ? OFFSET ?`

	pageRows := make([]DailyRowWithTotal, 0, limit)
	if err := s.observeDB("prices_latest_page", func() error {
		return s.db.WithContext(ctx).Raw(query, append(whereArgs, limit, offset)...).Scan(&pageRows).Error
	}); err != nil {
		return nil, err
	}

	rows := make([]DailyRow, 0, len(pageRows))
	var total int64
	if len(pageRows) > 0 {
		total = pageRows[0].TotalCount
		for i := range pageRows {
			row := pageRows[i].DailyRow
			row.ItemTotal = roundFloatPtr2(row.ItemTotal)
			row.ArrivalTon = roundFloatPtr2(row.ArrivalTon)
			rows = append(rows, row)
		}
	}

	return &LatestPricesResult{
		Rows: rows, LatestTradeDate: latestTradeDate,
		Limit: limit, Offset: offset, Total: total,
	}, nil
}

// GetPriceTrend returns daily aggregated trend data.
func (s *Service) GetPriceTrend(ctx context.Context, filter PriceFilter) (*TrendResult, error) {
	ids, err := s.resolveCodeIDs(ctx, filter.ItemCode, filter.MarketCode, filter.OriginCode)
	if err != nil {
		return nil, err
	}
	if ids == nil || ids.ItemID == nil {
		return &TrendResult{Rows: []TrendRow{}}, nil
	}

	whereSQL, whereArgs, defaultFrom, err := buildPriceFilterClauses(filter, ids, false)
	if err != nil {
		return nil, err
	}

	query := `
SELECT
	to_char(f.trade_date, 'YYYY-MM-DD') AS trade_date,
	COUNT(f.price_mid_yen) AS rows_count,
	AVG(f.price_mid_yen)::float8 AS avg_price_mid_yen,
	MAX(f.price_mid_yen) AS max_price_mid_yen,
	MIN(f.price_mid_yen) AS min_price_mid_yen,
	SUM(f.arrival_ton)::float8 AS arrival_ton_sum
FROM fact_prices_daily f
` + whereSQL + `
GROUP BY f.trade_date
HAVING COUNT(f.price_mid_yen) > 0
ORDER BY f.trade_date ASC`

	rows := make([]TrendRow, 0, 64)
	if err := s.observeDB("prices_trend", func() error {
		return s.db.WithContext(ctx).Raw(query, whereArgs...).Scan(&rows).Error
	}); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].AvgPriceMid = roundFloatPtr2(rows[i].AvgPriceMid)
		rows[i].ArrivalTon = roundFloatPtr2(rows[i].ArrivalTon)
	}

	wd := defaultRecentDays
	return &TrendResult{Rows: rows, DefaultFrom: defaultFrom, DefaultWindowDays: wd}, nil
}

// GetPriceSummary returns aggregated summary data grouped by period.
func (s *Service) GetPriceSummary(ctx context.Context, filter PriceFilter, groupBy string) (*SummaryResult, error) {
	ids, err := s.resolveCodeIDs(ctx, filter.ItemCode, filter.MarketCode, filter.OriginCode)
	if err != nil {
		return nil, err
	}
	if ids == nil || ids.ItemID == nil {
		return &SummaryResult{Rows: []SummaryRow{}, GroupBy: groupBy}, nil
	}

	periodExpr := "to_char(f.trade_date, 'YYYY-MM-DD')"
	switch groupBy {
	case "week":
		periodExpr = "to_char(date_trunc('week', f.trade_date), 'YYYY-MM-DD')"
	case "month":
		periodExpr = "to_char(date_trunc('month', f.trade_date), 'YYYY-MM')"
	}

	whereSQL, whereArgs, defaultFrom, err := buildPriceFilterClauses(filter, ids, false)
	if err != nil {
		return nil, err
	}

	query := `
SELECT
	` + periodExpr + ` AS period,
	COUNT(*) AS rows_count,
	AVG(f.price_mid_yen)::float8 AS avg_price_mid_yen,
	SUM(f.arrival_ton)::float8 AS arrival_ton_sum
FROM fact_prices_daily f
` + whereSQL + `
GROUP BY period
ORDER BY period ASC`

	rows := make([]SummaryRow, 0, 64)
	if err := s.observeDB("prices_summary", func() error {
		return s.db.WithContext(ctx).Raw(query, whereArgs...).Scan(&rows).Error
	}); err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].AvgPriceMid = roundFloatPtr2(rows[i].AvgPriceMid)
		rows[i].ArrivalTon = roundFloatPtr2(rows[i].ArrivalTon)
	}

	wd := defaultRecentDays
	return &SummaryResult{Rows: rows, GroupBy: groupBy, DefaultFrom: defaultFrom, DefaultWindowDays: wd}, nil
}

// GetCoverage returns aggregate coverage information from the database.
func (s *Service) GetCoverage(ctx context.Context) (*CoverageResult, error) {
	row := coverageRow{}
	if err := s.observeDB("coverage_fact_summary", func() error {
		return s.db.WithContext(ctx).Raw(`
SELECT
	to_char(MIN(trade_date), 'YYYY-MM-DD') AS earliest_trade_date,
	to_char(MAX(trade_date), 'YYYY-MM-DD') AS latest_trade_date
FROM fact_prices_daily
`).Scan(&row).Error
	}); err != nil {
		return nil, err
	}

	if err := s.observeDB("coverage_fact_count_estimate", func() error {
		return s.db.WithContext(ctx).Raw(`
SELECT COALESCE(
	(SELECT GREATEST(reltuples, 0)::bigint
	 FROM pg_class
	 WHERE oid = 'fact_prices_daily'::regclass),
	0
) AS fact_rows_total
`).Scan(&row.FactRowsTotal).Error
	}); err != nil {
		return nil, err
	}

	var latestRun model.IngestionRun
	if err := s.observeDB("coverage_latest_ingestion", func() error {
		return s.db.WithContext(ctx).Order("id DESC").Limit(1).Find(&latestRun).Error
	}); err != nil {
		return nil, err
	}

	result := &CoverageResult{
		EarliestTradeDate: row.EarliestTradeDate,
		LatestTradeDate:   row.LatestTradeDate,
		FactRowsTotal:     row.FactRowsTotal,
	}
	if latestRun.ID > 0 {
		id := latestRun.ID
		status := latestRun.Status
		runType := latestRun.RunType
		result.LastIngestionRunID = &id
		result.LastIngestionStatus = &status
		result.LastIngestionRunType = &runType
		result.LastIngestionAt = latestRun.FinishedAt
	}
	return result, nil
}

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

// ListDimensions returns a paginated list of dimension entries.
func (s *Service) ListDimensions(ctx context.Context, q DimensionQuery) (*DimensionListResult, error) {
	limit := clampInt(q.Limit, 1, 500)
	offset := maxInt(q.Offset, 0)
	sort := parseDimensionSort(q.Kind, q.Sort)
	order := parseOrder(q.Order)

	query := s.db.WithContext(ctx)

	switch q.Kind {
	case "market":
		if q.Q != "" {
			query = query.Where("market_code LIKE ? OR market_name LIKE ?", "%"+q.Q+"%", "%"+q.Q+"%")
		}
		var total int64
		if err := s.observeDB("markets_count", func() error {
			return query.Model(&model.DimMarket{}).Count(&total).Error
		}); err != nil {
			return nil, err
		}
		var data []model.DimMarket
		if err := s.observeDB("markets_list", func() error {
			return query.Model(&model.DimMarket{}).Order(sort + " " + order).Limit(limit).Offset(offset).Find(&data).Error
		}); err != nil {
			return nil, err
		}
		rows := make([]DimensionRow, 0, len(data))
		for _, d := range data {
			rows = append(rows, DimensionRow{ID: d.ID, Code: d.MarketCode, Name: d.MarketName})
		}
		return &DimensionListResult{Rows: rows, Total: total, Limit: limit, Off: offset}, nil

	case "item":
		if q.Q != "" {
			query = query.Where("item_code LIKE ? OR item_name LIKE ?", "%"+q.Q+"%", "%"+q.Q+"%")
		}
		var total int64
		if err := s.observeDB("items_count", func() error {
			return query.Model(&model.DimItem{}).Count(&total).Error
		}); err != nil {
			return nil, err
		}
		var data []model.DimItem
		if err := s.observeDB("items_list", func() error {
			return query.Model(&model.DimItem{}).Order(sort + " " + order).Limit(limit).Offset(offset).Find(&data).Error
		}); err != nil {
			return nil, err
		}
		rows := make([]DimensionRow, 0, len(data))
		for _, d := range data {
			rows = append(rows, DimensionRow{ID: d.ID, Code: d.ItemCode, Name: d.ItemName})
		}
		return &DimensionListResult{Rows: rows, Total: total, Limit: limit, Off: offset}, nil

	default: // "origin"
		if q.Q != "" {
			query = query.Where("origin_code LIKE ? OR origin_name LIKE ?", "%"+q.Q+"%", "%"+q.Q+"%")
		}
		var total int64
		if err := s.observeDB("origins_count", func() error {
			return query.Model(&model.DimOrigin{}).Count(&total).Error
		}); err != nil {
			return nil, err
		}
		var data []model.DimOrigin
		if err := s.observeDB("origins_list", func() error {
			return query.Model(&model.DimOrigin{}).Order(sort + " " + order).Limit(limit).Offset(offset).Find(&data).Error
		}); err != nil {
			return nil, err
		}
		rows := make([]DimensionRow, 0, len(data))
		for _, d := range data {
			rows = append(rows, DimensionRow{ID: d.ID, Code: d.OriginCode, Name: d.OriginName})
		}
		return &DimensionListResult{Rows: rows, Total: total, Limit: limit, Off: offset}, nil
	}
}


