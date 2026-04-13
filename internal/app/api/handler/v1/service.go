package v1

import (
	"errors"
	"net/http"

	"gorm.io/gorm"
)

type dbObserver func(queryName string, fn func() error) error

type Service struct {
	db        *gorm.DB
	observeDB dbObserver
}

func NewService(db *gorm.DB, observer dbObserver) *Service {
	if observer == nil {
		observer = func(_ string, fn func() error) error {
			if fn == nil {
				return errors.New("nil db function")
			}
			return fn()
		}
	}

	return &Service{db: db, observeDB: observer}
}

func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/markets", s.handleMarkets)
	mux.HandleFunc("/v1/items", s.handleItems)
	mux.HandleFunc("/v1/origins", s.handleOrigins)
	mux.HandleFunc("/v1/coverage", s.handleCoverage)
	mux.HandleFunc("/v1/prices/daily", s.handlePricesDaily)
	mux.HandleFunc("/v1/prices/latest", s.handlePricesLatest)
	mux.HandleFunc("/v1/prices/trend", s.handlePricesTrend)
	mux.HandleFunc("/v1/prices/summary", s.handlePricesSummary)
	mux.HandleFunc("/v1/compare/markets", s.handleCompareMarkets)
	mux.HandleFunc("/v1/rankings/items", s.handleRankingsItems)
	mux.HandleFunc("/ingestion/runs", s.handleIngestionRuns)
	mux.HandleFunc("/ingestion/files", s.handleIngestionFiles)
}
