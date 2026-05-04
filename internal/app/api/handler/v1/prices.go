package v1

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"japan_data_project/internal/app/query"
)

func (h *Service) handleCoverage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	result, err := h.q.GetCoverage(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}

	var finishedAt any
	if result.LastIngestionAt != nil {
		finishedAt = result.LastIngestionAt.Format("2006-01-02T15:04:05Z07:00")
	}

	writeOK(w, coverageRow{
		EarliestTradeDate:    result.EarliestTradeDate,
		LatestTradeDate:      result.LatestTradeDate,
		FactRowsTotal:        result.FactRowsTotal,
		LastIngestionRunID:   result.LastIngestionRunID,
		LastIngestionStatus:  result.LastIngestionStatus,
		LastIngestionRunType: result.LastIngestionRunType,
		LastIngestionAt:      finishedAt,
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

	result, err := h.q.GetDailyPrices(ctx, query.PriceFilter{
		ItemCode:   itemCode,
		MarketCode: strings.TrimSpace(r.URL.Query().Get("market_code")),
		OriginCode: strings.TrimSpace(r.URL.Query().Get("origin_code")),
		DateRange: query.DateRange{
			Date: strings.TrimSpace(r.URL.Query().Get("date")),
			From: strings.TrimSpace(r.URL.Query().Get("from")),
			To:   strings.TrimSpace(r.URL.Query().Get("to")),
		},
		Limit:  queryParseIntOrDefault(r.URL.Query().Get("limit"), 100),
		Offset: queryParseIntOrDefault(r.URL.Query().Get("offset"), 0),
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

	result, err := h.q.GetLatestPrices(ctx, query.PriceFilter{
		ItemCode:   itemCode,
		MarketCode: strings.TrimSpace(r.URL.Query().Get("market_code")),
		OriginCode: strings.TrimSpace(r.URL.Query().Get("origin_code")),
		Limit:      queryParseIntOrDefault(r.URL.Query().Get("limit"), 100),
		Offset:     queryParseIntOrDefault(r.URL.Query().Get("offset"), 0),
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

	result, err := h.q.GetPriceTrend(ctx, query.PriceFilter{
		ItemCode:   itemCode,
		MarketCode: strings.TrimSpace(r.URL.Query().Get("market_code")),
		OriginCode: strings.TrimSpace(r.URL.Query().Get("origin_code")),
		DateRange: query.DateRange{
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
	q := cloned.Query()
	q.Del("date")
	q.Set("from", from)
	q.Set("to", to)
	cloned.RawQuery = q.Encode()
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

	result, err := h.q.GetPriceSummary(ctx, query.PriceFilter{
		ItemCode:   itemCode,
		MarketCode: strings.TrimSpace(r.URL.Query().Get("market_code")),
		OriginCode: strings.TrimSpace(r.URL.Query().Get("origin_code")),
		DateRange: query.DateRange{
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

	metric := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("metric")))
	if metric == "" {
		metric = "price_mid"
	}
	if metric != "price_mid" && metric != "arrival" {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "metric must be price_mid|arrival")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := h.q.GetCompareMarkets(ctx, date, itemCode, metric, r.URL.Query().Get("order"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	writeOK(w, result.Rows, apiMeta{"metric": result.Metric, "total": result.Total})
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

	metric := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("metric")))
	if metric == "" {
		metric = "arrival"
	}
	if metric != "arrival" && metric != "price_mid" {
		writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "metric must be arrival|price_mid")
		return
	}

	limit := queryClampInt(queryParseIntOrDefault(r.URL.Query().Get("limit"), 50), 1, 500)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := h.q.GetRankItems(ctx, date, metric, strings.TrimSpace(r.URL.Query().Get("market_code")), r.URL.Query().Get("order"), limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}
	writeOK(w, result.Rows, apiMeta{"metric": result.Metric, "limit": result.Limit, "total": result.Total})
}

// queryParseIntOrDefault is a thin wrapper for parseIntOrDefault from the query package.
func queryParseIntOrDefault(v string, d int) int {
	return query.ParseIntOrDefault(v, d)
}

// queryClampInt is a thin wrapper for clampInt from the query package.
func queryClampInt(v, minV, maxV int) int {
	return query.ClampInt(v, minV, maxV)
}
