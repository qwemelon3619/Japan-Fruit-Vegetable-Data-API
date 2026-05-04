package v1

import (
	"net/http"

	"japan_data_project/internal/app/query"
)

// Service provides HTTP handlers for v1 API endpoints.
// Query methods are delegated to *query.Service (shared with gRPC).
type Service struct {
	q *query.Service
}

// NewService creates a new v1 Service.
func NewService(q *query.Service) *Service {
	return &Service{q: q}
}

// Q returns the underlying query service (for gRPC access).
func (s *Service) Q() *query.Service {
	return s.q
}

func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/markets", s.handleMarkets)
	mux.HandleFunc("/v1/items", s.handleItems)
	mux.HandleFunc("/v1/origins", s.handleOrigins)
	mux.HandleFunc("/v1/coverage", s.handleCoverage)
	mux.HandleFunc("/v1/prices/daily", s.handlePricesDaily)
	mux.HandleFunc("/v1/prices/latest", s.handlePricesLatest)
	mux.HandleFunc("/v1/prices/trend", s.handlePricesTrend)
	mux.HandleFunc("/v1/prices/trend/1m", s.handlePricesTrend1Month)
	mux.HandleFunc("/v1/prices/trend/6m", s.handlePricesTrend6Months)
	mux.HandleFunc("/v1/prices/trend/1y", s.handlePricesTrend1Year)
	mux.HandleFunc("/v1/prices/summary", s.handlePricesSummary)
	mux.HandleFunc("/v1/compare/markets", s.handleCompareMarkets)
	mux.HandleFunc("/v1/rankings/items", s.handleRankingsItems)
	mux.HandleFunc("/ingestion/runs", s.handleIngestionRuns)
	mux.HandleFunc("/ingestion/files", s.handleIngestionFiles)
}
