package query

import (
	"context"

	"japan_data_project/internal/domain/model"
)

// GetCoverage returns aggregate coverage information from the database.
func (s *Service) GetCoverage(ctx context.Context) (*CoverageResult, error) {
	row := internalCoverageRow{}
	if err := s.observeDB("coverage_fact_summary", func() error {
		return s.db.WithContext(ctx).Raw(`
SELECT
	to_char(MIN(trade_date), 'YYYY-MM-DD') AS earliest_trade_date,
	to_char(MAX(trade_date), 'YYYY-MM-DD') AS latest_trade_date
FROM fact_prices_daily
`).Scan(&row).Error
	}); err != nil {
		return nil, err
	}

	if err := s.observeDB("coverage_fact_count_estimate", func() error {
		return s.db.WithContext(ctx).Raw(`
SELECT COALESCE(
	(SELECT GREATEST(reltuples, 0)::bigint
	 FROM pg_class
	 WHERE oid = 'fact_prices_daily'::regclass),
	0
) AS fact_rows_total
`).Scan(&row.FactRowsTotal).Error
	}); err != nil {
		return nil, err
	}

	var latestRun model.IngestionRun
	if err := s.observeDB("coverage_latest_ingestion", func() error {
		return s.db.WithContext(ctx).Model(&model.IngestionRun{}).
			Order("id DESC").Limit(1).Find(&latestRun).Error
	}); err != nil {
		return nil, err
	}

	result := &CoverageResult{
		EarliestTradeDate: row.EarliestTradeDate,
		LatestTradeDate:   row.LatestTradeDate,
		FactRowsTotal:     row.FactRowsTotal,
	}
	if latestRun.ID > 0 {
		id := latestRun.ID
		status := latestRun.Status
		runType := latestRun.RunType
		result.LastIngestionRunID = &id
		result.LastIngestionStatus = &status
		result.LastIngestionRunType = &runType
		result.LastIngestionAt = latestRun.FinishedAt
	}
	return result, nil
}
