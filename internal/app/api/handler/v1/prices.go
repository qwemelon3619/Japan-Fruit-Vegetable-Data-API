package v1

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
	"japan_data_project/internal/domain/model"
)

func (h *Service) handleCoverage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	row := coverageRow{}
	if err := h.observeDB("coverage_fact_summary", func() error {
		return h.db.WithContext(ctx).Raw(`
SELECT
	to_char(MIN(trade_date), 'YYYY-MM-DD') AS earliest_trade_date,
	to_char(MAX(trade_date), 'YYYY-MM-DD') AS latest_trade_date
FROM fact_prices_daily
`).Scan(&row).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	if err := h.observeDB("coverage_fact_count_estimate", func() error {
		return h.db.WithContext(ctx).Raw(`
SELECT COALESCE(
	(SELECT GREATEST(reltuples, 0)::bigint
	 FROM pg_class
	 WHERE oid = 'fact_prices_daily'::regclass),
	0
) AS fact_rows_total
`).Scan(&row.FactRowsTotal).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}

	var latestRun model.IngestionRun
	if err := h.observeDB("coverage_latest_ingestion", func() error {
		return h.db.WithContext(ctx).Order("id DESC").Limit(1).Find(&latestRun).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	if latestRun.ID > 0 {
		id := latestRun.ID
		status := latestRun.Status
		runType := latestRun.RunType
		row.LastIngestionRunID = &id
		row.LastIngestionStatus = &status
		row.LastIngestionRunType = &runType
		row.LastIngestionAt = latestRun.FinishedAt
	}

	writeOK(w, row, apiMeta{})
}

func (h *Service) handlePricesDaily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if strings.TrimSpace(r.URL.Query().Get("item_code")) == "" {
		writeMissingRequiredParams(w, "item_code")
		return
	}

	limit := clampInt(parseIntOrDefault(r.URL.Query().Get("limit"), 100), 1, 2000)
	offset := maxInt(parseIntOrDefault(r.URL.Query().Get("offset"), 0), 0)
	sort := parseDailySort(r.URL.Query().Get("sort"))
	order := parseOrder(r.URL.Query().Get("order"))

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	ids, err := h.resolveCodeIDs(ctx,
		strings.TrimSpace(r.URL.Query().Get("item_code")),
		strings.TrimSpace(r.URL.Query().Get("market_code")),
		strings.TrimSpace(r.URL.Query().Get("origin_code")),
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	if ids == nil || ids.ItemID == nil {
		writeOK(w, []dailyRow{}, apiMeta{"limit": limit, "offset": offset, "total": 0})
		return
	}

	whereSQL, whereArgs, defaultFrom, err := buildFactPriceFilters(r.URL.Query(), ids, true)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}

	orderBy := parseDailySortColumn(sort)
	query := `
WITH filtered_fact AS (
	SELECT
		f.id,
		f.trade_date,
		f.weekday_ja,
		f.market_id,
		f.item_id,
		f.origin_id,
		f.grade_id,
		f.item_total_ton,
		f.arrival_ton,
		f.price_high_yen,
		f.price_mid_yen,
		f.price_low_yen,
		f.trend_label,
		f.source_file,
		f.source_row_no
	FROM fact_prices_daily f
	` + whereSQL + `
)
SELECT
	to_char(ff.trade_date, 'YYYY-MM-DD') AS trade_date,
	ff.weekday_ja,
	market_code,
	market_name,
	item_code,
	item_name,
	origin_code,
	origin_name,
	grade,
	class,
	product_name,
	unit_weight,
	ff.item_total_ton,
	ff.arrival_ton,
	ff.price_high_yen,
	ff.price_mid_yen,
	ff.price_low_yen,
	ff.trend_label,
	ff.source_file,
	ff.source_row_no,
	COUNT(*) OVER() AS total_count
FROM filtered_fact ff
JOIN dim_market m ON m.id = ff.market_id
JOIN dim_item i ON i.id = ff.item_id
JOIN dim_origin o ON o.id = ff.origin_id
JOIN dim_grade g ON g.id = ff.grade_id
ORDER BY ` + orderBy + ` ` + order + `
LIMIT ? OFFSET ?`

	countQuery := `
SELECT COUNT(1)
FROM fact_prices_daily f
` + whereSQL

	pageRows := make([]dailyRowWithTotal, 0, limit)
	if err := h.observeDB("prices_daily_page", func() error {
		return h.db.WithContext(ctx).Raw(query, append(whereArgs, limit, offset)...).Scan(&pageRows).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	rows := make([]dailyRow, 0, len(pageRows))
	var total int64
	if len(pageRows) > 0 {
		total = pageRows[0].TotalCount
		for i := range pageRows {
			row := pageRows[i].DailyRow
			row.ItemTotal = roundFloatPtr2(row.ItemTotal)
			row.ArrivalTon = roundFloatPtr2(row.ArrivalTon)
			rows = append(rows, row)
		}
	} else if err := h.observeDB("prices_daily_count_fallback", func() error {
		return h.db.WithContext(ctx).Raw(countQuery, whereArgs...).Scan(&total).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "count failed")
		return
	}

	meta := apiMeta{"limit": limit, "offset": offset, "total": total}
	if defaultFrom != nil {
		meta["default_from"] = *defaultFrom
		meta["default_window_days"] = defaultRecentDays
	}
	writeOK(w, rows, meta)
}

func (h *Service) handlePricesLatest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if strings.TrimSpace(r.URL.Query().Get("item_code")) == "" {
		writeMissingRequiredParams(w, "item_code")
		return
	}

	limit := clampInt(parseIntOrDefault(r.URL.Query().Get("limit"), 100), 1, 2000)
	offset := maxInt(parseIntOrDefault(r.URL.Query().Get("offset"), 0), 0)
	sort := parseDailySort(r.URL.Query().Get("sort"))
	order := parseOrder(r.URL.Query().Get("order"))

	if _, _, err := buildLatestFilters(r); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	ids, err := h.resolveCodeIDs(ctx,
		strings.TrimSpace(r.URL.Query().Get("item_code")),
		strings.TrimSpace(r.URL.Query().Get("market_code")),
		strings.TrimSpace(r.URL.Query().Get("origin_code")),
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	if ids == nil || ids.ItemID == nil {
		meta := apiMeta{"latest_trade_date": nil, "limit": limit, "offset": offset, "total": 0}
		writeOK(w, []dailyRow{}, meta)
		return
	}

	var latestTradeDate *string
	latestDateQuery := `
SELECT to_char(MAX(trade_date), 'YYYY-MM-DD') AS latest_trade_date
FROM fact_prices_daily
WHERE item_id = ?`
	latestDateArgs := []any{*ids.ItemID}
	if ids.MarketID != nil {
		latestDateQuery += " AND market_id = ?"
		latestDateArgs = append(latestDateArgs, *ids.MarketID)
	}
	if ids.OriginID != nil {
		latestDateQuery += " AND origin_id = ?"
		latestDateArgs = append(latestDateArgs, *ids.OriginID)
	}
	if err := h.observeDB("prices_latest_trade_date", func() error {
		return h.db.WithContext(ctx).Raw(latestDateQuery, latestDateArgs...).Scan(&latestTradeDate).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	if latestTradeDate == nil || strings.TrimSpace(*latestTradeDate) == "" {
		meta := apiMeta{"latest_trade_date": nil, "limit": limit, "offset": offset, "total": 0}
		writeOK(w, []dailyRow{}, meta)
		return
	}

	latestWhereSQL, latestWhereArgs := buildFactLatestFilters(ids, *latestTradeDate)

	orderBy := parseDailySortColumn(sort)
	query := `
WITH filtered_fact AS (
	SELECT
		f.id,
		f.trade_date,
		f.weekday_ja,
		f.market_id,
		f.item_id,
		f.origin_id,
		f.grade_id,
		f.item_total_ton,
		f.arrival_ton,
		f.price_high_yen,
		f.price_mid_yen,
		f.price_low_yen,
		f.trend_label,
		f.source_file,
		f.source_row_no
	FROM fact_prices_daily f
	` + latestWhereSQL + `
)
SELECT
	to_char(ff.trade_date, 'YYYY-MM-DD') AS trade_date,
	ff.weekday_ja,
	market_code,
	market_name,
	item_code,
	item_name,
	origin_code,
	origin_name,
	grade,
	class,
	product_name,
	unit_weight,
	ff.item_total_ton,
	ff.arrival_ton,
	ff.price_high_yen,
	ff.price_mid_yen,
	ff.price_low_yen,
	ff.trend_label,
	ff.source_file,
	ff.source_row_no,
	COUNT(*) OVER() AS total_count
FROM filtered_fact ff
JOIN dim_market m ON m.id = ff.market_id
JOIN dim_item i ON i.id = ff.item_id
JOIN dim_origin o ON o.id = ff.origin_id
JOIN dim_grade g ON g.id = ff.grade_id
ORDER BY ` + orderBy + ` ` + order + `
LIMIT ? OFFSET ?`

	pageRows := make([]dailyRowWithTotal, 0, limit)
	if err := h.observeDB("prices_latest_page", func() error {
		return h.db.WithContext(ctx).Raw(query, append(latestWhereArgs, limit, offset)...).Scan(&pageRows).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}

	rows := make([]dailyRow, 0, len(pageRows))
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

	meta := apiMeta{"latest_trade_date": latestTradeDate, "limit": limit, "offset": offset, "total": total}
	writeOK(w, rows, meta)
}

func (h *Service) handlePricesTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if strings.TrimSpace(r.URL.Query().Get("item_code")) == "" {
		writeMissingRequiredParams(w, "item_code")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	ids, err := h.resolveCodeIDs(ctx,
		strings.TrimSpace(r.URL.Query().Get("item_code")),
		strings.TrimSpace(r.URL.Query().Get("market_code")),
		strings.TrimSpace(r.URL.Query().Get("origin_code")),
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	if ids == nil || ids.ItemID == nil {
		writeOK(w, []trendRow{}, apiMeta{"total": 0})
		return
	}

	whereSQL, whereArgs, defaultFrom, err := buildFactPriceFilters(r.URL.Query(), ids, false)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
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

	rows := make([]trendRow, 0, 64)
	if err := h.observeDB("prices_trend", func() error {
		return h.db.WithContext(ctx).Raw(query, whereArgs...).Scan(&rows).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	for i := range rows {
		rows[i].AvgPriceMid = roundFloatPtr2(rows[i].AvgPriceMid)
		rows[i].ArrivalTon = roundFloatPtr2(rows[i].ArrivalTon)
	}
	meta := apiMeta{"total": len(rows)}
	if defaultFrom != nil {
		meta["default_from"] = *defaultFrom
		meta["default_window_days"] = defaultRecentDays
	}
	writeOK(w, rows, meta)
}

func (h *Service) handlePricesTrend1Month(w http.ResponseWriter, r *http.Request) {
	h.handlePricesTrendPreset(w, r, 0, -1, 0)
}

func (h *Service) handlePricesTrend6Months(w http.ResponseWriter, r *http.Request) {
	h.handlePricesTrendPreset(w, r, 0, -6, 0)
}

func (h *Service) handlePricesTrend1Year(w http.ResponseWriter, r *http.Request) {
	h.handlePricesTrendPreset(w, r, -1, 0, 0)
}

func (h *Service) handlePricesTrendPreset(w http.ResponseWriter, r *http.Request, years, months, days int) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	now := time.Now()
	cloned := r.Clone(r.Context())
	cloned.URL = cloneURLWithTrendRange(r.URL, now.AddDate(years, months, days).Format("2006-01-02"), now.Format("2006-01-02"))
	h.handlePricesTrend(w, cloned)
}

func cloneURLWithTrendRange(src *url.URL, from, to string) *url.URL {
	cloned := *src
	query := cloned.Query()
	query.Del("date")
	query.Set("from", from)
	query.Set("to", to)
	cloned.RawQuery = query.Encode()
	return &cloned
}

func (h *Service) handlePricesSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if strings.TrimSpace(r.URL.Query().Get("item_code")) == "" {
		writeMissingRequiredParams(w, "item_code")
		return
	}
	groupBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("group_by")))
	if groupBy == "" {
		groupBy = "day"
	}

	periodExpr := "to_char(f.trade_date, 'YYYY-MM-DD')"
	switch groupBy {
	case "day":
		periodExpr = "to_char(f.trade_date, 'YYYY-MM-DD')"
	case "week":
		periodExpr = "to_char(date_trunc('week', f.trade_date), 'YYYY-MM-DD')"
	case "month":
		periodExpr = "to_char(date_trunc('month', f.trade_date), 'YYYY-MM')"
	default:
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "group_by must be day|week|month")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	ids, err := h.resolveCodeIDs(ctx,
		strings.TrimSpace(r.URL.Query().Get("item_code")),
		strings.TrimSpace(r.URL.Query().Get("market_code")),
		strings.TrimSpace(r.URL.Query().Get("origin_code")),
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	if ids == nil || ids.ItemID == nil {
		writeOK(w, []summaryRow{}, apiMeta{"group_by": groupBy, "total": 0})
		return
	}

	whereSQL, whereArgs, defaultFrom, err := buildFactPriceFilters(r.URL.Query(), ids, false)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
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

	rows := make([]summaryRow, 0, 64)
	if err := h.observeDB("prices_summary", func() error {
		return h.db.WithContext(ctx).Raw(query, whereArgs...).Scan(&rows).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	for i := range rows {
		rows[i].AvgPriceMid = roundFloatPtr2(rows[i].AvgPriceMid)
		rows[i].ArrivalTon = roundFloatPtr2(rows[i].ArrivalTon)
	}

	meta := apiMeta{"group_by": groupBy, "total": len(rows)}
	if defaultFrom != nil {
		meta["default_from"] = *defaultFrom
		meta["default_window_days"] = defaultRecentDays
	}
	writeOK(w, rows, meta)
}

func (h *Service) handleCompareMarkets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	itemCode := strings.TrimSpace(r.URL.Query().Get("item_code"))
	if date == "" || itemCode == "" {
		writeMissingRequiredParams(w, "date", "item_code")
		return
	}
	if _, err := mustParseDate(date, "date"); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}
	if err := validateCode("item_code", itemCode); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}

	metric := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("metric")))
	if metric == "" {
		metric = "price_mid"
	}
	metricExpr := "AVG(f.price_mid_yen)::float8"
	switch metric {
	case "price_mid":
		metricExpr = "AVG(f.price_mid_yen)::float8"
	case "arrival":
		metricExpr = "SUM(f.arrival_ton)::float8"
	default:
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "metric must be price_mid|arrival")
		return
	}
	order := parseOrder(r.URL.Query().Get("order"))

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ids, err := h.resolveCodeIDs(ctx, itemCode, "", "")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	if ids == nil || ids.ItemID == nil {
		writeOK(w, []compareMarketRow{}, apiMeta{"metric": metric, "total": 0})
		return
	}

	query := `
		SELECT
			m.market_code,
			m.market_name,
			COUNT(*) AS rows_count,
			` + metricExpr + ` AS metric_value
		FROM fact_prices_daily f
		JOIN dim_market m ON m.id = f.market_id
		WHERE f.trade_date = ? AND f.item_id = ?
		GROUP BY m.market_code, m.market_name
		ORDER BY metric_value ` + order + `, m.market_code ASC`
	rows := make([]compareMarketRow, 0, 64)
	if err := h.observeDB("compare_markets", func() error {
		return h.db.WithContext(ctx).Raw(query, date, *ids.ItemID).Scan(&rows).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	for i := range rows {
		rows[i].Metric = roundFloatPtr2(rows[i].Metric)
	}
	writeOK(w, rows, apiMeta{"metric": metric, "total": len(rows)})
}

func (h *Service) handleRankingsItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	if date == "" {
		writeMissingRequiredParams(w, "date")
		return
	}
	if _, err := mustParseDate(date, "date"); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}

	metric := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("metric")))
	if metric == "" {
		metric = "arrival"
	}
	metricExpr := "SUM(f.arrival_ton)::float8"
	metricCountExpr := "COUNT(f.arrival_ton)"
	switch metric {
	case "arrival":
		metricExpr = "SUM(f.arrival_ton)::float8"
		metricCountExpr = "COUNT(f.arrival_ton)"
	case "price_mid":
		metricExpr = "AVG(f.price_mid_yen)::float8"
		metricCountExpr = "COUNT(f.price_mid_yen)"
	default:
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "metric must be arrival|price_mid")
		return
	}
	order := parseOrder(r.URL.Query().Get("order"))
	limit := clampInt(parseIntOrDefault(r.URL.Query().Get("limit"), 50), 1, 500)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var marketID *uint
	where := "WHERE f.trade_date = ?"
	args := []any{date}
	marketCode := strings.TrimSpace(r.URL.Query().Get("market_code"))
	if marketCode != "" {
		if err := validateCode("market_code", marketCode); err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
			return
		}
		ids, err := h.resolveCodeIDs(ctx, "", marketCode, "")
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
			return
		}
		if ids == nil || ids.MarketID == nil {
			writeOK(w, []rankingItemRow{}, apiMeta{"metric": metric, "limit": limit, "total": 0})
			return
		}
		marketID = ids.MarketID
		where += " AND f.market_id = ?"
		args = append(args, *marketID)
	}

	query := `
SELECT
	i.item_code,
	i.item_name,
	` + metricCountExpr + ` AS rows_count,
	` + metricExpr + ` AS metric_value
FROM fact_prices_daily f
JOIN dim_item i ON i.id = f.item_id
` + where + `
GROUP BY i.item_code, i.item_name
HAVING ` + metricCountExpr + ` > 0
ORDER BY metric_value ` + order + ` NULLS LAST, i.item_code ASC
LIMIT ?`
	args = append(args, limit)

	rows := make([]rankingItemRow, 0, limit)
	if err := h.observeDB("rankings_items", func() error {
		return h.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	for i := range rows {
		rows[i].Metric = roundFloatPtr2(rows[i].Metric)
	}
	writeOK(w, rows, apiMeta{"metric": metric, "limit": limit, "total": len(rows)})
}

func roundFloatPtr2(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := math.Round((*v)*100) / 100
	return &r
}

func parseDailySortColumn(sort string) string {
	switch sort {
	case "f.trade_date":
		return "trade_date"
	case "m.market_code":
		return "market_code"
	case "i.item_code":
		return "item_code"
	case "o.origin_code":
		return "origin_code"
	case "f.price_mid_yen":
		return "price_mid_yen"
	case "f.arrival_ton":
		return "arrival_ton"
	default:
		panic(fmt.Sprintf("unsupported daily sort column: %s", sort))
	}
}

type latestFilterIDs struct {
	ItemID   *uint
	MarketID *uint
	OriginID *uint
}

func (h *Service) resolveCodeIDs(ctx context.Context, itemCode, marketCode, originCode string) (*latestFilterIDs, error) {
	ids := &latestFilterIDs{}

	if itemCode != "" {
		var item model.DimItem
		if err := h.observeDB("prices_lookup_item", func() error {
			return h.db.WithContext(ctx).Select("id").Where("item_code = ?", itemCode).First(&item).Error
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
		if err := h.observeDB("prices_lookup_market", func() error {
			return h.db.WithContext(ctx).Select("id").Where("market_code = ?", marketCode).First(&market).Error
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
		if err := h.observeDB("prices_lookup_origin", func() error {
			return h.db.WithContext(ctx).Select("id").Where("origin_code = ?", originCode).First(&origin).Error
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
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func buildFactPriceFilters(q url.Values, ids *latestFilterIDs, includeSingleDate bool) (string, []any, *string, error) {
	clauses, args, defaultFrom, err := buildFactDateClauses(q, includeSingleDate)
	if err != nil {
		return "", nil, nil, err
	}
	if ids == nil || ids.ItemID == nil {
		return "", nil, defaultFrom, nil
	}
	clauses = append(clauses, "f.item_id = ?")
	args = append(args, *ids.ItemID)
	if ids.MarketID != nil {
		clauses = append(clauses, "f.market_id = ?")
		args = append(args, *ids.MarketID)
	}
	if ids.OriginID != nil {
		clauses = append(clauses, "f.origin_id = ?")
		args = append(args, *ids.OriginID)
	}
	return "WHERE " + strings.Join(clauses, " AND "), args, defaultFrom, nil
}

func buildFactDateClauses(q url.Values, includeSingleDate bool) ([]string, []any, *string, error) {
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 8)
	var defaultFrom *string
	date := strings.TrimSpace(q.Get("date"))
	from := strings.TrimSpace(q.Get("from"))
	to := strings.TrimSpace(q.Get("to"))

	if includeSingleDate && date != "" {
		if _, err := mustParseDate(date, "date"); err != nil {
			return nil, nil, nil, err
		}
		clauses = append(clauses, "f.trade_date = ?")
		args = append(args, date)
	}

	var fromDate, toDate *time.Time
	if from != "" {
		d, err := mustParseDate(from, "from")
		if err != nil {
			return nil, nil, nil, err
		}
		fromDate = &d
		clauses = append(clauses, "f.trade_date >= ?")
		args = append(args, from)
	}
	if to != "" {
		d, err := mustParseDate(to, "to")
		if err != nil {
			return nil, nil, nil, err
		}
		toDate = &d
		clauses = append(clauses, "f.trade_date <= ?")
		args = append(args, to)
	}
	if err := validateDateRange(fromDate, toDate); err != nil {
		return nil, nil, nil, err
	}

	if date == "" && fromDate == nil && toDate == nil {
		df := time.Now().AddDate(0, 0, -defaultRecentDays).Format("2006-01-02")
		defaultFrom = &df
		clauses = append(clauses, "f.trade_date >= ?")
		args = append(args, df)
	}

	return clauses, args, defaultFrom, nil
}
