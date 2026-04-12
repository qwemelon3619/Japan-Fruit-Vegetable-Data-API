package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

type smokeEnvelope struct {
	Data json.RawMessage `json:"data"`
	Meta map[string]any  `json:"meta"`
}

type smokeTrendRow struct {
	TradeDate string `json:"trade_date"`
	RowsCount int64  `json:"rows_count"`
}

func TestAPISmoke_Health(t *testing.T) {
	base := requireAPIBaseURL(t)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(base + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got=%d want=%d", resp.StatusCode, http.StatusOK)
	}
	var body smokeEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response failed: %v", err)
	}
	if len(body.Data) == 0 {
		t.Fatalf("health response data is empty")
	}
}

func TestAPISmoke_PricesTrend(t *testing.T) {
	base := requireAPIBaseURL(t)
	seed := fetchEndpointSeed(t, base)
	if seed.TradeDate == "" || seed.ItemCode == "" {
		t.Skip("no seed row in /v1/prices/daily")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	u := fmt.Sprintf("%s/v1/prices/trend?item_code=%s&from=%s&to=%s", base, seed.ItemCode, seed.TradeDate, seed.TradeDate)
	resp, err := client.Get(u)
	if err != nil {
		t.Fatalf("trend request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: got=%d want=%d", resp.StatusCode, http.StatusOK)
	}
	var env smokeEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode trend response failed: %v", err)
	}
	var rows []smokeTrendRow
	if err := json.Unmarshal(env.Data, &rows); err != nil {
		t.Fatalf("decode trend data failed: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("trend rows is empty")
	}
}
