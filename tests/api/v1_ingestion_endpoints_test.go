package tests

import (
	"net/http"
	"testing"
)

func TestV1IngestionEndpoints_Min4Each(t *testing.T) {
	base := requireAPIBaseURL(t)
	seed := fetchEndpointSeed(t, base)

	t.Run("/v1/ingestion/runs", func(t *testing.T) {
		runEndpointCases(t, base, seed, []endpointCase{
			{name: "get_ok_default", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/ingestion/runs" }, wantStatus: http.StatusOK},
			{name: "get_ok_with_paging", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/ingestion/runs?limit=10&offset=0" }, wantStatus: http.StatusOK},
			{name: "post_405", method: http.MethodPost, path: func(base string, _ endpointSeed) string { return base + "/v1/ingestion/runs" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "delete_405", method: http.MethodDelete, path: func(base string, _ endpointSeed) string { return base + "/v1/ingestion/runs" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
		})
	})

	t.Run("/v1/ingestion/files", func(t *testing.T) {
		runEndpointCases(t, base, seed, []endpointCase{
			{name: "get_ok_default", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/ingestion/files" }, wantStatus: http.StatusOK},
			{name: "get_ok_with_paging", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/ingestion/files?limit=20&offset=0" }, wantStatus: http.StatusOK},
			{name: "invalid_run_id_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + "/v1/ingestion/files?run_id=-1" }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
			{name: "post_405", method: http.MethodPost, path: func(base string, _ endpointSeed) string { return base + "/v1/ingestion/files" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
		})
	})
}
