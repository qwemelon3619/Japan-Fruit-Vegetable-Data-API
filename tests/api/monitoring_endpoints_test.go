package tests

import (
	"net/http"
	"strings"
	"testing"
)

func TestMonitoringEndpoints_Min4Each(t *testing.T) {
	base := requireAPIBaseURL(t)
	seed := fetchEndpointSeed(t, base)

	cases := map[string][]endpointCase{
		"/monitoring/dashboard": {
			{
				name:       "get_ok",
				method:     http.MethodGet,
				path:       func(base string, _ endpointSeed) string { return base + "/monitoring/dashboard" },
				wantStatus: http.StatusOK,
				check: func(t *testing.T, body []byte) {
					if !strings.Contains(string(body), "<!doctype html>") {
						t.Fatalf("dashboard html not returned")
					}
				},
			},
			{name: "post_405", method: http.MethodPost, path: func(base string, _ endpointSeed) string { return base + "/monitoring/dashboard" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "put_405", method: http.MethodPut, path: func(base string, _ endpointSeed) string { return base + "/monitoring/dashboard" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "delete_405", method: http.MethodDelete, path: func(base string, _ endpointSeed) string { return base + "/monitoring/dashboard" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
		},
		"/monitoring/snapshots.csv": {
			{
				name:       "get_ok",
				method:     http.MethodGet,
				path:       func(base string, _ endpointSeed) string { return base + "/monitoring/snapshots.csv" },
				wantStatus: http.StatusOK,
				check: func(t *testing.T, body []byte) {
					header := strings.Split(strings.TrimSpace(string(body)), "\n")[0]
					const expected = "timestamp,ready_ok,http_total,http_5xx,db_errors_total,http_p95_ms,ingestion_latest_status"
					if header != expected {
						t.Fatalf("unexpected snapshots header: %q", header)
					}
				},
			},
			{name: "post_405", method: http.MethodPost, path: func(base string, _ endpointSeed) string { return base + "/monitoring/snapshots.csv" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "put_405", method: http.MethodPut, path: func(base string, _ endpointSeed) string { return base + "/monitoring/snapshots.csv" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "delete_405", method: http.MethodDelete, path: func(base string, _ endpointSeed) string { return base + "/monitoring/snapshots.csv" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
		},
	}

	for ep, c := range cases {
		t.Run(ep, func(t *testing.T) {
			runEndpointCases(t, base, seed, c)
		})
	}
}
