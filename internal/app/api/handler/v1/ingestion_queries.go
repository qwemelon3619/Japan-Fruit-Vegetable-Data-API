package v1

import (
	"context"
	"time"
)

// IngestionRunRow holds a single ingestion run result row.
type IngestionRunRow struct {
	ID           uint       `gorm:"column:id"`
	RunType      string     `gorm:"column:run_type"`
	StartedAt    time.Time  `gorm:"column:started_at"`
	FinishedAt   *time.Time `gorm:"column:finished_at"`
	Status       string     `gorm:"column:status"`
	ErrorMessage *string    `gorm:"column:error_message"`
}

// IngestionRunsResult holds the paginated result of a list ingestion runs query.
type IngestionRunsResult struct {
	Rows   []IngestionRunRow
	Limit  int
	Offset int
	Total  int64
}

// ListIngestionRuns returns a paginated list of ingestion runs.
func (s *Service) ListIngestionRuns(ctx context.Context, limit, offset int) (*IngestionRunsResult, error) {
	l := clampInt(limit, 1, 500)
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

// IngestionFileRow holds a single ingestion file result row.
type IngestionFileRow struct {
	ID        uint   `gorm:"column:id"`
	RunID     uint   `gorm:"column:run_id"`
	FilePath  string `gorm:"column:file_path"`
	FileHash  string `gorm:"column:file_hash"`
	RowsTotal int    `gorm:"column:rows_total"`
	RowsOK    int    `gorm:"column:rows_ok"`
	RowsError int    `gorm:"column:rows_error"`
	Status    string `gorm:"column:status"`
}

// IngestionFilesResult holds the paginated result of a list ingestion files query.
type IngestionFilesResult struct {
	Rows   []IngestionFileRow
	Limit  int
	Offset int
	Total  int64
}

// ListIngestionFiles returns a paginated list of ingestion files, optionally filtered by run_id.
func (s *Service) ListIngestionFiles(ctx context.Context, runID *uint, limit, offset int) (*IngestionFilesResult, error) {
	l := clampInt(limit, 1, 1000)
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
