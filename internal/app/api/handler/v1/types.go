package v1

// apiError represents an error response.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// apiMeta holds response metadata.
type apiMeta map[string]any

// coverageRow is the HTTP response shape for the coverage endpoint.
type coverageRow struct {
	EarliestTradeDate    *string `json:"earliest_trade_date"`
	LatestTradeDate      *string `json:"latest_trade_date"`
	FactRowsTotal        int64   `json:"fact_rows_total"`
	LastIngestionRunID   *uint   `json:"last_ingestion_run_id"`
	LastIngestionStatus  *string `json:"last_ingestion_status"`
	LastIngestionRunType *string `json:"last_ingestion_run_type"`
	LastIngestionAt      any     `json:"last_ingestion_finished_at"`
}
