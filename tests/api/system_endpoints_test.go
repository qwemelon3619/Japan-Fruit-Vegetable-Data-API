package tests

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSystemEndpoints_Min4Each(t *testing.T) {
	base := requireAPIBaseURL(t)
	seed := fetchEndpointSeed(t, base)

	cases := map[string][]endpointCase{
		"/health": {
			{
				name:       "get_ok",
				method:     http.MethodGet,
				path:       func(base string, _ endpointSeed) string { return base + "/health" },
				wantStatus: http.StatusOK,
				check: func(t *testing.T, body []byte) {
					okBody := mustDecodeOKBody(t, body)
					var d map[string]string
					_ = json.Unmarshal(okBody.Data, &d)
					if d["status"] != "ok" {
						t.Fatalf("unexpected status: %q", d["status"])
					}
				},
			},
			{name: "post_405", method: http.MethodPost, path: func(base string, _ endpointSeed) string { return base + "/health" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "put_405", method: http.MethodPut, path: func(base string, _ endpointSeed) string { return base + "/health" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "delete_405", method: http.MethodDelete, path: func(base string, _ endpointSeed) string { return base + "/health" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
		},
		"/ready": {
			{
				name:       "get_ok",
				method:     http.MethodGet,
				path:       func(base string, _ endpointSeed) string { return base + "/ready" },
				wantStatus: http.StatusOK,
				check: func(t *testing.T, body []byte) {
					okBody := mustDecodeOKBody(t, body)
					var d map[string]string
					_ = json.Unmarshal(okBody.Data, &d)
					if d["status"] != "ready" {
						t.Fatalf("unexpected status: %q", d["status"])
					}
				},
			},
			{name: "post_405", method: http.MethodPost, path: func(base string, _ endpointSeed) string { return base + "/ready" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "put_405", method: http.MethodPut, path: func(base string, _ endpointSeed) string { return base + "/ready" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "delete_405", method: http.MethodDelete, path: func(base string, _ endpointSeed) string { return base + "/ready" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
		},
		"/metrics": {
			{
				name:       "get_ok",
				method:     http.MethodGet,
				path:       func(base string, _ endpointSeed) string { return base + "/metrics" },
				wantStatus: http.StatusOK,
				check: func(t *testing.T, body []byte) {
					if !strings.Contains(string(body), "http_requests_total") {
						t.Fatalf("missing http_requests_total metric")
					}
				},
			},
			{name: "post_405", method: http.MethodPost, path: func(base string, _ endpointSeed) string { return base + "/metrics" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "put_405", method: http.MethodPut, path: func(base string, _ endpointSeed) string { return base + "/metrics" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "delete_405", method: http.MethodDelete, path: func(base string, _ endpointSeed) string { return base + "/metrics" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
		},
	}

	for ep, c := range cases {
		t.Run(ep, func(t *testing.T) {
			runEndpointCases(t, base, seed, c)
		})
	}
}
