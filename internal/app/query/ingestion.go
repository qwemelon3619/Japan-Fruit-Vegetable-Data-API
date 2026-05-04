package query

import (
	"context"
)

// ListIngestionRuns returns a paginated list of ingestion runs.
func (s *Service) ListIngestionRuns(ctx context.Context, limit, offset int) (*IngestionRunsResult, error) {
	l := ClampInt(limit, 1, 500)
	o := maxInt(offset, 0)

	var total int64
	if err := s.observeDB("ingestion_runs_count", func() error {
		return s.db.WithContext(ctx).Table("ingestion_runs").Count(&total).Error
	}); err != nil {
		return nil, err
	}

	rows := make([]IngestionRunRow, 0, l)
	if err := s.observeDB("ingestion_runs_list", func() error {
		return s.db.WithContext(ctx).Table("ingestion_runs").
			Order("id DESC").Limit(l).Offset(o).
			Scan(&rows).Error
	}); err != nil {
		return nil, err
	}

	return &IngestionRunsResult{Rows: rows, Limit: l, Offset: o, Total: total}, nil
}

// ListIngestionFiles returns a paginated list of ingestion files, optionally filtered by run_id.
func (s *Service) ListIngestionFiles(ctx context.Context, runID *uint, limit, offset int) (*IngestionFilesResult, error) {
	l := ClampInt(limit, 1, 1000)
	o := maxInt(offset, 0)

	query := s.db.WithContext(ctx).Table("ingestion_files")
	if runID != nil {
		query = query.Where("run_id = ?", *runID)
	}

	var total int64
	if err := s.observeDB("ingestion_files_count", func() error {
		return query.Count(&total).Error
	}); err != nil {
		return nil, err
	}

	rows := make([]IngestionFileRow, 0, l)
	if err := s.observeDB("ingestion_files_list", func() error {
		return query.Order("id DESC").Limit(l).Offset(o).Scan(&rows).Error
	}); err != nil {
		return nil, err
	}

	return &IngestionFilesResult{Rows: rows, Limit: l, Offset: o, Total: total}, nil
}
