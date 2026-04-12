package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type apiErrBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type apiOKBody struct {
	Data json.RawMessage `json:"data"`
	Meta map[string]any  `json:"meta"`
}

type endpointSeed struct {
	TradeDate  string
	ItemCode   string
	MarketCode string
}

type priceDailySeedRow struct {
	TradeDate  string `json:"trade_date"`
	ItemCode   string `json:"item_code"`
	MarketCode string `json:"market_code"`
}

type endpointCase struct {
	name       string
	method     string
	path       func(base string, seed endpointSeed) string
	wantStatus int
	check      func(t *testing.T, body []byte)
}

func requireAPIBaseURL(t *testing.T) string {
	t.Helper()
	if strings.TrimSpace(strings.ToLower(os.Getenv("RUN_API_INTEGRATION_TESTS"))) != "1" {
		t.Skip("set RUN_API_INTEGRATION_TESTS=1 to run API integration tests")
	}
	base := strings.TrimSpace(os.Getenv("API_BASE_URL"))
	if base == "" {
		base = "http://localhost:8080"
	}
	return strings.TrimRight(base, "/")
}

func doAPIRequest(t *testing.T, client *http.Client, method, rawURL string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed (%s %s): %v", method, rawURL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body failed: %v", err)
	}
	return resp.StatusCode, body
}

func mustDecodeOKBody(t *testing.T, body []byte) apiOKBody {
	t.Helper()
	var out apiOKBody
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode ok body failed: %v\nbody=%s", err, string(body))
	}
	return out
}

func mustDecodeErrBody(t *testing.T, body []byte) apiErrBody {
	t.Helper()
	var out apiErrBody
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode error body failed: %v\nbody=%s", err, string(body))
	}
	return out
}

func assertMethodNotAllowed(t *testing.T, body []byte) {
	t.Helper()
	eb := mustDecodeErrBody(t, body)
	if eb.Error.Code != "METHOD_NOT_ALLOWED" {
		t.Fatalf("expected METHOD_NOT_ALLOWED, got=%q", eb.Error.Code)
	}
}

func assertInvalidArgument(t *testing.T, body []byte) {
	t.Helper()
	eb := mustDecodeErrBody(t, body)
	if eb.Error.Code != "INVALID_ARGUMENT" {
		t.Fatalf("expected INVALID_ARGUMENT, got=%q", eb.Error.Code)
	}
}

func fetchEndpointSeed(t *testing.T, base string) endpointSeed {
	t.Helper()
	client := &http.Client{Timeout: 15 * time.Second}
	status, body := doAPIRequest(t, client, http.MethodGet, base+"/v1/prices/daily?limit=1")
	if status != http.StatusOK {
		return endpointSeed{}
	}
	okBody := mustDecodeOKBody(t, body)
	var rows []priceDailySeedRow
	if err := json.Unmarshal(okBody.Data, &rows); err != nil || len(rows) == 0 {
		return endpointSeed{}
	}
	return endpointSeed{
		TradeDate:  rows[0].TradeDate,
		ItemCode:   rows[0].ItemCode,
		MarketCode: rows[0].MarketCode,
	}
}

func runEndpointCases(t *testing.T, base string, seed endpointSeed, cases []endpointCase) {
	t.Helper()
	client := &http.Client{Timeout: 20 * time.Second}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rawURL := tc.path(base, seed)
			status, body := doAPIRequest(t, client, tc.method, rawURL)
			if status != tc.wantStatus {
				t.Fatalf("status mismatch for %s %s: got=%d want=%d body=%s", tc.method, rawURL, status, tc.wantStatus, string(body))
			}
			if tc.check != nil {
				tc.check(t, body)
			}
		})
	}
}
