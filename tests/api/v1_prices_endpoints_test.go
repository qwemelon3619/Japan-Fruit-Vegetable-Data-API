package tests

import (
	"net/http"
	"testing"
)

func TestV1PricesEndpoints_Min4Each(t *testing.T) {
	base := requireAPIBaseURL(t)
	seed := fetchEndpointSeed(t, base)

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
