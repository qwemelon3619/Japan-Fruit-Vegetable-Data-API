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

	result, err := h.GetCoverage(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}

	writeOK(w, coverageRow{
		EarliestTradeDate:    result.EarliestTradeDate,
		LatestTradeDate:      result.LatestTradeDate,
		FactRowsTotal:        result.FactRowsTotal,
		LastIngestionRunID:   result.LastIngestionRunID,
		LastIngestionStatus:  result.LastIngestionStatus,
		LastIngestionRunType: result.LastIngestionRunType,
		LastIngestionAt:      result.LastIngestionAt,
	}, apiMeta{})
}

func (h *Service) handlePricesDaily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	itemCode := strings.TrimSpace(r.URL.Query().Get("item_code"))
	if itemCode == "" {
		writeMissingRequiredParams(w, "item_code")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	result, err := h.GetDailyPrices(ctx, PriceFilter{
		ItemCode:   itemCode,
		MarketCode: strings.TrimSpace(r.URL.Query().Get("market_code")),
		OriginCode: strings.TrimSpace(r.URL.Query().Get("origin_code")),
		DateRange: DateRange{
			Date: strings.TrimSpace(r.URL.Query().Get("date")),
			From: strings.TrimSpace(r.URL.Query().Get("from")),
			To:   strings.TrimSpace(r.URL.Query().Get("to")),
		},
		Limit:  parseIntOrDefault(r.URL.Query().Get("limit"), 100),
		Offset: parseIntOrDefault(r.URL.Query().Get("offset"), 0),
		Sort:   r.URL.Query().Get("sort"),
		Order:  r.URL.Query().Get("order"),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}

	meta := apiMeta{"limit": result.Limit, "offset": result.Offset}
	if result.DefaultFrom != nil {
		meta["default_from"] = *result.DefaultFrom
		meta["default_window_days"] = result.DefaultWindowDays
	}
	writeOK(w, result.Rows, meta)
}

func (h *Service) handlePricesLatest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	itemCode := strings.TrimSpace(r.URL.Query().Get("item_code"))
	if itemCode == "" {
		writeMissingRequiredParams(w, "item_code")
		return
	}
	if strings.TrimSpace(r.URL.Query().Get("date")) != "" ||
		strings.TrimSpace(r.URL.Query().Get("from")) != "" ||
		strings.TrimSpace(r.URL.Query().Get("to")) != "" {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "date, from, and to are not supported for this endpoint")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	result, err := h.GetLatestPrices(ctx, PriceFilter{
		ItemCode:   itemCode,
		MarketCode: strings.TrimSpace(r.URL.Query().Get("market_code")),
		OriginCode: strings.TrimSpace(r.URL.Query().Get("origin_code")),
		Limit:      parseIntOrDefault(r.URL.Query().Get("limit"), 100),
		Offset:     parseIntOrDefault(r.URL.Query().Get("offset"), 0),
		Sort:       r.URL.Query().Get("sort"),
		Order:      r.URL.Query().Get("order"),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}

	meta := apiMeta{
		"latest_trade_date": result.LatestTradeDate,
		"limit":             result.Limit,
		"offset":            result.Offset,
		"total":             result.Total,
	}
	writeOK(w, result.Rows, meta)
}

func (h *Service) handlePricesTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	itemCode := strings.TrimSpace(r.URL.Query().Get("item_code"))
	if itemCode == "" {
		writeMissingRequiredParams(w, "item_code")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	result, err := h.GetPriceTrend(ctx, PriceFilter{
		ItemCode:   itemCode,
		MarketCode: strings.TrimSpace(r.URL.Query().Get("market_code")),
		OriginCode: strings.TrimSpace(r.URL.Query().Get("origin_code")),
		DateRange: DateRange{
			From: strings.TrimSpace(r.URL.Query().Get("from")),
			To:   strings.TrimSpace(r.URL.Query().Get("to")),
		},
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}

	meta := apiMeta{"total": len(result.Rows)}
	if result.DefaultFrom != nil {
		meta["default_from"] = *result.DefaultFrom
		meta["default_window_days"] = result.DefaultWindowDays
	}
	writeOK(w, result.Rows, meta)
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
	itemCode := strings.TrimSpace(r.URL.Query().Get("item_code"))
	if itemCode == "" {
		writeMissingRequiredParams(w, "item_code")
		return
	}
	groupBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("group_by")))
	if groupBy == "" {
		groupBy = "day"
	}
	if groupBy != "day" && groupBy != "week" && groupBy != "month" {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "group_by must be day|week|month")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	result, err := h.GetPriceSummary(ctx, PriceFilter{
		ItemCode:   itemCode,
		MarketCode: strings.TrimSpace(r.URL.Query().Get("market_code")),
		OriginCode: strings.TrimSpace(r.URL.Query().Get("origin_code")),
		DateRange: DateRange{
			From: strings.TrimSpace(r.URL.Query().Get("from")),
			To:   strings.TrimSpace(r.URL.Query().Get("to")),
		},
	}, groupBy)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}

	meta := apiMeta{"group_by": result.GroupBy, "total": len(result.Rows)}
	if result.DefaultFrom != nil {
		meta["default_from"] = *result.DefaultFrom
		meta["default_window_days"] = result.DefaultWindowDays
	}
	writeOK(w, result.Rows, meta)
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
WITH ranked AS (
	SELECT
		f.item_id,
		` + metricCountExpr + ` AS rows_count,
		` + metricExpr + ` AS metric_value
	FROM fact_prices_daily f
` + where + `
	GROUP BY f.item_id
	HAVING ` + metricCountExpr + ` > 0
	ORDER BY metric_value ` + order + ` NULLS LAST, f.item_id ASC
	LIMIT ?
)
SELECT
	i.item_code,
	i.item_name,
	r.rows_count,
	r.metric_value
FROM ranked r
JOIN dim_item i ON i.id = r.item_id
ORDER BY r.metric_value ` + order + ` NULLS LAST, i.item_code ASC`
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
