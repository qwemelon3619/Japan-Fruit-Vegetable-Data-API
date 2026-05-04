package query

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"japan_data_project/internal/domain/model"
)

// resolveCodeIDs resolves dimension codes to database IDs.
func (s *Service) resolveCodeIDs(ctx context.Context, itemCode, marketCode, originCode string) (*latestFilterIDs, error) {
	ids := &latestFilterIDs{}

	if itemCode != "" {
		var item model.DimItem
		if err := s.observeDB("prices_lookup_item", func() error {
			return s.db.WithContext(ctx).Select("id").Where("item_code = ?", itemCode).First(&item).Error
		}); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			return nil, err
		}
		ids.ItemID = &item.ID
	}

	if marketCode != "" {
		var market model.DimMarket
		if err := s.observeDB("prices_lookup_market", func() error {
			return s.db.WithContext(ctx).Select("id").Where("market_code = ?", marketCode).First(&market).Error
		}); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			return nil, err
		}
		ids.MarketID = &market.ID
	}

	if originCode != "" {
		var origin model.DimOrigin
		if err := s.observeDB("prices_lookup_origin", func() error {
			return s.db.WithContext(ctx).Select("id").Where("origin_code = ?", originCode).First(&origin).Error
		}); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			return nil, err
		}
		ids.OriginID = &origin.ID
	}

	return ids, nil
}

// buildPriceFilterClauses builds SQL WHERE clause and args from a PriceFilter.
func buildPriceFilterClauses(filter PriceFilter, ids *latestFilterIDs, includeSingleDate bool) (string, []any, *string, error) {
	clauses, args, defaultFrom, err := buildFactDateClauses(filter, includeSingleDate)
	if err != nil {
		return "", nil, nil, err
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

// buildFactLatestFilters builds filters for the latest trade date query.
func buildFactLatestFilters(ids *latestFilterIDs, latestTradeDate string) (string, []any) {
	clauses := []string{"f.trade_date = ?", "f.item_id = ?"}
	args := []any{latestTradeDate, *ids.ItemID}
	if ids.MarketID != nil {
		clauses = append(clauses, "f.market_id = ?")
		args = append(args, *ids.MarketID)
	}
	if ids.OriginID != nil {
		clauses = append(clauses, "f.origin_id = ?")
		args = append(args, *ids.OriginID)
	}
	return "WHERE " + joinClauses(clauses, " AND "), args
}

// GetDailyPrices returns paginated daily price rows.
func (s *Service) GetDailyPrices(ctx context.Context, filter PriceFilter) (*DailyPricesResult, error) {
	limit := ClampInt(filter.Limit, 1, 2000)
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
	limit := ClampInt(filter.Limit, 1, 2000)
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

	whereSQL, whereArgs := buildFactLatestFilters(ids, *latestTradeDate)
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
	// Validate groupBy
	if groupBy == "" {
		groupBy = "day"
	}
	if groupBy != "day" && groupBy != "week" && groupBy != "month" {
		groupBy = "day"
	}

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

// compile-time check that value type does not trigger allocation rules; kept as reference.
var _ = strings.TrimSpace
