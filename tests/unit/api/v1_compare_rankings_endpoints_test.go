package tests

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestV1CompareAndRankingsEndpoints_Min4Each(t *testing.T) {
	base := requireAPIBaseURL(t)
	seed := fetchEndpointSeed(t, base)

	t.Run("/v1/compare/markets", func(t *testing.T) {
		runEndpointCases(t, base, seed, []endpointCase{
			{
				name:   "get_ok_seeded",
				method: http.MethodGet,
				path: func(base string, seed endpointSeed) string {
					if seed.TradeDate == "" || seed.ItemCode == "" {
						t.Skip("no seed row in /v1/prices/daily; skip success case")
					}
					return fmt.Sprintf("%s/v1/compare/markets?date=%s&item_code=%s", base, seed.TradeDate, url.QueryEscape(seed.ItemCode))
				},
				wantStatus: http.StatusOK,
			},
			{name: "missing_required_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/compare/markets" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
			{name: "invalid_date_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/compare/markets?date=20260401&item_code=30100" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
			{name: "invalid_metric_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/compare/markets?date=2026-04-01&item_code=30100&metric=volume" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
		})
	})

	t.Run("/v1/rankings/items", func(t *testing.T) {
		runEndpointCases(t, base, seed, []endpointCase{
			{
				name:   "get_ok_seeded",
				method: http.MethodGet,
				path: func(base string, seed endpointSeed) string {
					if seed.TradeDate == "" {
						t.Skip("no seed row in /v1/prices/daily; skip success case")
					}
					return fmt.Sprintf("%s/v1/rankings/items?date=%s&metric=arrival&limit=20", base, seed.TradeDate)
				},
				wantStatus: http.StatusOK,
			},
			{name: "missing_date_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/rankings/items" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
			{name: "invalid_date_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/rankings/items?date=20260401" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
			{name: "invalid_metric_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/rankings/items?date=2026-04-01&metric=price_high" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
		})
	})
}
