package tests

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestV1DimensionEndpoints_Min4Each(t *testing.T) {
	base := requireAPIBaseURL(t)
	seed := fetchEndpointSeed(t, base)
	longQ := strings.Repeat("a", 101)

	for _, ep := range []string{"/v1/markets", "/v1/items", "/v1/origins"} {
		ep := ep
		t.Run(ep, func(t *testing.T) {
			runEndpointCases(t, base, seed, []endpointCase{
				{name: "get_ok_default", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return base + ep }, wantStatus: http.StatusOK},
				{name: "get_ok_with_paging", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return fmt.Sprintf("%s%s?limit=5&offset=0&order=asc", base, ep) }, wantStatus: http.StatusOK},
				{name: "invalid_q_too_long_400", method: http.MethodGet, path: func(base string, _ endpointSeed) string { return fmt.Sprintf("%s%s?q=%s", base, ep, url.QueryEscape(longQ)) }, wantStatus: http.StatusBadRequest, check: assertInvalidArgument},
				{name: "post_405", method: http.MethodPost, path: func(base string, _ endpointSeed) string { return base + ep }, wantStatus: http.StatusMethodNotAllowed, check: assertMethodNotAllowed},
			})
		})
	}
}
