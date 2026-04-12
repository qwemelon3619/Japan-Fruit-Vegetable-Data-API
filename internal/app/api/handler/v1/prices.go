package v1

import (
	"context"
	"math"
	"net/http"
	"strings"
	"time"
)

func (h *Service) handlePricesDaily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	limit := clampInt(parseIntOrDefault(r.URL.Query().Get("limit"), 100), 1, 2000)
	offset := maxInt(parseIntOrDefault(r.URL.Query().Get("offset"), 0), 0)
	sort := parseDailySort(r.URL.Query().Get("sort"))
	order := parseOrder(r.URL.Query().Get("order"))

	whereSQL, whereArgs, err := buildPriceFilters(r, true)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}

	query := `
SELECT
	to_char(f.trade_date, 'YYYY-MM-DD') AS trade_date,
	f.weekday_ja,
	m.market_code,
	m.market_name,
	i.item_code,
	i.item_name,
	o.origin_code,
	o.origin_name,
	g.grade,
	g.class,
	g.product_name,
	g.unit_weight,
	f.item_total_ton,
	f.arrival_ton,
	f.price_high_yen,
	f.price_mid_yen,
	f.price_low_yen,
	f.trend_label,
	f.source_file,
	f.source_row_no
FROM fact_prices_daily f
JOIN dim_market m ON m.id = f.market_id
JOIN dim_item i ON i.id = f.item_id
JOIN dim_origin o ON o.id = f.origin_id
JOIN dim_grade g ON g.id = f.grade_id
` + whereSQL + `
ORDER BY ` + sort + ` ` + order + `
LIMIT ? OFFSET ?`

	countQuery := `
SELECT COUNT(1)
FROM fact_prices_daily f
JOIN dim_market m ON m.id = f.market_id
JOIN dim_item i ON i.id = f.item_id
JOIN dim_origin o ON o.id = f.origin_id
` + whereSQL

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	rows := make([]dailyRow, 0, limit)
	if err := h.observeDB("prices_daily_list", func() error {
		return h.db.WithContext(ctx).Raw(query, append(whereArgs, limit, offset)...).Scan(&rows).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	for i := range rows {
		rows[i].ItemTotal = roundFloatPtr2(rows[i].ItemTotal)
		rows[i].ArrivalTon = roundFloatPtr2(rows[i].ArrivalTon)
	}

	var total int64
	if err := h.observeDB("prices_daily_count", func() error {
		return h.db.WithContext(ctx).Raw(countQuery, whereArgs...).Scan(&total).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "count failed")
		return
	}

	writeOK(w, rows, apiMeta{"limit": limit, "offset": offset, "total": total})
}

func (h *Service) handlePricesTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	whereSQL, whereArgs, err := buildPriceFilters(r, false)
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
JOIN dim_market m ON m.id = f.market_id
JOIN dim_item i ON i.id = f.item_id
JOIN dim_origin o ON o.id = f.origin_id
` + whereSQL + `
GROUP BY f.trade_date
HAVING COUNT(f.price_mid_yen) > 0
ORDER BY f.trade_date ASC`

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
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
	writeOK(w, rows, apiMeta{"rows": len(rows)})
}

func (h *Service) handlePricesSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
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

	whereSQL, whereArgs, err := buildPriceFilters(r, false)
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
JOIN dim_market m ON m.id = f.market_id
JOIN dim_item i ON i.id = f.item_id
JOIN dim_origin o ON o.id = f.origin_id
` + whereSQL + `
GROUP BY period
ORDER BY period ASC`

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
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

	writeOK(w, rows, apiMeta{"group_by": groupBy, "rows": len(rows)})
}

func (h *Service) handleCompareMarkets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	itemCode := strings.TrimSpace(r.URL.Query().Get("item_code"))
	if date == "" || itemCode == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "date and item_code are required")
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

	query := `
		SELECT
			m.market_code,
			m.market_name,
			COUNT(*) AS rows_count,
			` + metricExpr + ` AS metric_value
		FROM fact_prices_daily f
		JOIN dim_market m ON m.id = f.market_id
		JOIN dim_item i ON i.id = f.item_id
		WHERE f.trade_date = ? AND i.item_code = ?
		GROUP BY m.market_code, m.market_name
		ORDER BY metric_value ` + order + `, m.market_code ASC`

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	rows := make([]compareMarketRow, 0, 64)
	if err := h.observeDB("compare_markets", func() error {
		return h.db.WithContext(ctx).Raw(query, date, itemCode).Scan(&rows).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	for i := range rows {
		rows[i].Metric = roundFloatPtr2(rows[i].Metric)
	}
	writeOK(w, rows, apiMeta{"metric": metric, "rows": len(rows)})
}

func (h *Service) handleRankingsItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	if date == "" {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "date is required")
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

	where := "WHERE f.trade_date = ?"
	args := []any{date}
	marketCode := strings.TrimSpace(r.URL.Query().Get("market_code"))
	if marketCode != "" {
		if err := validateCode("market_code", marketCode); err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
			return
		}
		where += " AND m.market_code = ?"
		args = append(args, marketCode)
	}

	query := `
SELECT
	i.item_code,
	i.item_name,
	` + metricCountExpr + ` AS rows_count,
	` + metricExpr + ` AS metric_value
FROM fact_prices_daily f
JOIN dim_item i ON i.id = f.item_id
JOIN dim_market m ON m.id = f.market_id
` + where + `
GROUP BY i.item_code, i.item_name
HAVING ` + metricCountExpr + ` > 0
ORDER BY metric_value ` + order + ` NULLS LAST, i.item_code ASC
LIMIT ?`
	args = append(args, limit)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
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
	writeOK(w, rows, apiMeta{"metric": metric, "limit": limit, "rows": len(rows)})
}

func roundFloatPtr2(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := math.Round((*v)*100) / 100
	return &r
}
