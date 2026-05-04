package v1

import (
	"context"
	"errors"
	"fmt"
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

// Ready checks database connectivity. Used by gRPC GetReady and HTTP /ready.
func (s *Service) Ready(ctx context.Context) error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql db: %w", err)
	}
	return s.observeDB("grpc_ready", func() error {
		return sqlDB.PingContext(ctx)
	})
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
