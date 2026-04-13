package tests

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestV1PricesEndpoints_Min4Each(t *testing.T) {
	base := requireAPIBaseURL(t)
	seed := fetchEndpointSeed(t, base)

	t.Run("/v1/coverage", func(t *testing.T) {
		runEndpointCases(t, base, seed, []endpointCase{
			{name: "get_ok", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/coverage" }, wantStatus: http.StatusOK, check: assertCoveragePayload},
			{name: "get_ok_repeat", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/coverage" }, wantStatus: http.StatusOK, check: assertCoveragePayload},
			{name: "post_405", method: http.MethodPost, path: func(base string, _ endpointSeed) string { return base + "/v1/coverage" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "delete_405", method: http.MethodDelete, path: func(base string, _ endpointSeed) string { return base + "/v1/coverage" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
		})
	})

	t.Run("/v1/prices/latest", func(t *testing.T) {
		runEndpointCases(t, base, seed, []endpointCase{
			{name: "get_ok_default", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/latest" }, wantStatus: http.StatusOK, check: assertLatestPayload},
			{name: "get_ok_with_filters", method: http.MethodGet, path: func(base string, seed endpointSeed) string { return base + "/v1/prices/latest?item_code=" + seed.ItemCode + "&market_code=" + seed.MarketCode + "&limit=10" }, wantStatus: http.StatusOK, check: assertLatestFilteredPayload(seed)},
			{name: "invalid_from_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/latest?from=2026/04/01" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
			{name: "post_405", method: http.MethodPost, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/latest" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
		})
	})

	t.Run("/v1/prices/daily", func(t *testing.T) {
		runEndpointCases(t, base, seed, []endpointCase{
			{name: "get_ok_default", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/daily" }, wantStatus: http.StatusOK},
			{name: "get_ok_with_range", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/daily?from=2026-04-01&to=2026-04-10&limit=10" }, wantStatus: http.StatusOK},
			{name: "invalid_date_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/daily?date=20260401" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
			{name: "to_before_from_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/daily?from=2026-04-10&to=2026-04-01" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
		})
	})

	t.Run("/v1/prices/trend", func(t *testing.T) {
		runEndpointCases(t, base, seed, []endpointCase{
			{name: "get_ok_default", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/trend" }, wantStatus: http.StatusOK},
			{name: "get_ok_with_range", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/trend?from=2026-04-01&to=2026-04-10" }, wantStatus: http.StatusOK},
			{name: "invalid_from_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/trend?from=2026/04/01" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
			{name: "range_too_large_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/trend?from=2020-01-01&to=2026-01-01" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
		})
	})

	t.Run("/v1/prices/summary", func(t *testing.T) {
		runEndpointCases(t, base, seed, []endpointCase{
			{name: "get_ok_default", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/summary" }, wantStatus: http.StatusOK},
			{name: "get_ok_group_month", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/summary?group_by=month&from=2026-04-01&to=2026-04-10" }, wantStatus: http.StatusOK},
			{name: "invalid_group_by_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/summary?group_by=yearly" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
			{name: "invalid_to_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/prices/summary?from=2026-04-01&to=20260410" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
		})
	})
}

type coverageResponse struct {
	EarliestTradeDate  string `json:"earliest_trade_date"`
	LatestTradeDate    string `json:"latest_trade_date"`
	FactRowsTotal      int64  `json:"fact_rows_total"`
	LastIngestionRunID *int64 `json:"last_ingestion_run_id"`
}

func assertCoveragePayload(t *testing.T, body []byte) {
	t.Helper()
	okBody := mustDecodeOKBody(t, body)

	var row coverageResponse
	if err := json.Unmarshal(okBody.Data, &row); err != nil {
		t.Fatalf("decode coverage payload failed: %v\nbody=%s", err, string(body))
	}
	if row.FactRowsTotal < 0 {
		t.Fatalf("expected non-negative fact_rows_total, got=%d", row.FactRowsTotal)
	}
	if row.EarliestTradeDate != "" && len(row.EarliestTradeDate) != len("2006-01-02") {
		t.Fatalf("unexpected earliest_trade_date format: %q", row.EarliestTradeDate)
	}
	if row.LatestTradeDate != "" && len(row.LatestTradeDate) != len("2006-01-02") {
		t.Fatalf("unexpected latest_trade_date format: %q", row.LatestTradeDate)
	}
	if row.EarliestTradeDate != "" && row.LatestTradeDate != "" && row.EarliestTradeDate > row.LatestTradeDate {
		t.Fatalf("expected earliest_trade_date <= latest_trade_date, got %q > %q", row.EarliestTradeDate, row.LatestTradeDate)
	}
}

func assertLatestPayload(t *testing.T, body []byte) {
	t.Helper()
	okBody := mustDecodeOKBody(t, body)

	latestTradeDate, _ := okBody.Meta["latest_trade_date"].(string)
	if latestTradeDate == "" {
		t.Fatalf("expected latest_trade_date in meta, got=%v", okBody.Meta["latest_trade_date"])
	}

	var rows []priceDailySeedRow
	if err := json.Unmarshal(okBody.Data, &rows); err != nil {
		t.Fatalf("decode latest payload failed: %v\nbody=%s", err, string(body))
	}
	if len(rows) == 0 {
		t.Fatalf("expected at least one latest row")
	}
	for _, row := range rows {
		if row.TradeDate != latestTradeDate {
			t.Fatalf("expected all rows to have latest trade_date=%q, got=%q", latestTradeDate, row.TradeDate)
		}
	}
}

func assertLatestFilteredPayload(seed endpointSeed) func(t *testing.T, body []byte) {
	return func(t *testing.T, body []byte) {
		t.Helper()
		okBody := mustDecodeOKBody(t, body)

		var rows []priceDailySeedRow
		if err := json.Unmarshal(okBody.Data, &rows); err != nil {
			t.Fatalf("decode filtered latest payload failed: %v\nbody=%s", err, string(body))
		}
		for _, row := range rows {
			if seed.ItemCode != "" && row.ItemCode != seed.ItemCode {
				t.Fatalf("expected item_code=%q, got=%q", seed.ItemCode, row.ItemCode)
			}
			if seed.MarketCode != "" && row.MarketCode != seed.MarketCode {
				t.Fatalf("expected market_code=%q, got=%q", seed.MarketCode, row.MarketCode)
			}
		}
	}
}
