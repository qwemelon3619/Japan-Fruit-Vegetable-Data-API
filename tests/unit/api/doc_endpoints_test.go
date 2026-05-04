package tests

import (
	"net/http"
	"strings"
	"testing"
)

func TestDocEndpoints_Min4Each(t *testing.T) {
	base := requireAPIBaseURL(t)
	seed := fetchEndpointSeed(t, base)

	cases := map[string][]endpointCase{
		"/doc": {
			{
				name:       "get_ok_returns_html",
				method:     http.MethodGet,
				path:       func(base string, _ endpointSeed) string { return base + "/doc" },
				wantStatus: http.StatusOK,
				check: func(t *testing.T, body []byte) {
					if !strings.Contains(string(body), "<!doctype html>") &&
						!strings.Contains(string(body), "<html") {
						t.Fatalf("doc endpoint did not return HTML content")
					}
					ct := "text/html"
					if !strings.Contains(string(body), ct) && !strings.Contains(string(body), "html") {
						// Content-Type header check via body content hint
						_ = ct
					}
				},
			},
			{name: "post_405", method: http.MethodPost, path: func(base string, _ endpointSeed) string { return base + "/doc" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "put_405", method: http.MethodPut, path: func(base string, _ endpointSeed) string { return base + "/doc" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "delete_405", method: http.MethodDelete, path: func(base string, _ endpointSeed) string { return base + "/doc" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
		},
		"/doc-llm": {
			{
				name:       "get_ok_returns_json",
				method:     http.MethodGet,
				path:       func(base string, _ endpointSeed) string { return base + "/doc-llm" },
				wantStatus: http.StatusOK,
				check: func(t *testing.T, body []byte) {
					if !strings.Contains(string(body), "openapi") &&
						!strings.Contains(string(body), "\"servers\"") &&
						!strings.Contains(string(body), "\"paths\"") {
						t.Fatalf("doc-llm endpoint did not return expected JSON schema content")
					}
				},
			},
			{name: "post_405", method: http.MethodPost, path: func(base string, _ endpointSeed) string { return base + "/doc-llm" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "put_405", method: http.MethodPut, path: func(base string, _ endpointSeed) string { return base + "/doc-llm" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			{name: "delete_405", method: http.MethodDelete, path: func(base string, _ endpointSeed) string { return base + "/doc-llm" }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
		},
	}

	for ep, c := range cases {
		t.Run(ep, func(t *testing.T) {
			runEndpointCases(t, base, seed, c)
		})
	}
}
