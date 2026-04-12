package v1

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"japan_data_project/internal/domain/model"
)

func (h *Service) handleIngestionRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	limit := clampInt(parseIntOrDefault(r.URL.Query().Get("limit"), 50), 1, 500)
	offset := maxInt(parseIntOrDefault(r.URL.Query().Get("offset"), 0), 0)

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	var total int64
	if err := h.observeDB("ingestion_runs_count", func() error {
		return h.db.WithContext(ctx).Model(&model.IngestionRun{}).Count(&total).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "count failed")
		return
	}

	rows := make([]ingestionRunRow, 0, limit)
	if err := h.observeDB("ingestion_runs_list", func() error {
		return h.db.WithContext(ctx).Model(&model.IngestionRun{}).
			Order("id DESC").Limit(limit).Offset(offset).
			Find(&rows).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}

	writeOK(w, rows, apiMeta{"limit": limit, "offset": offset, "total": total})
}

func (h *Service) handleIngestionFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	limit := clampInt(parseIntOrDefault(r.URL.Query().Get("limit"), 100), 1, 1000)
	offset := maxInt(parseIntOrDefault(r.URL.Query().Get("offset"), 0), 0)
	runID := strings.TrimSpace(r.URL.Query().Get("run_id"))

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	query := h.db.WithContext(ctx).Model(&model.IngestionFile{})
	if runID != "" {
		n, err := strconv.Atoi(runID)
		if err != nil || n <= 0 {
			writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "run_id must be positive integer")
			return
		}
		query = query.Where("run_id = ?", n)
	}

	var total int64
	if err := h.observeDB("ingestion_files_count", func() error { return query.Count(&total).Error }); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "count failed")
		return
	}

	rows := make([]ingestionFileRow, 0, limit)
	if err := h.observeDB("ingestion_files_list", func() error {
		return query.Order("id DESC").Limit(limit).Offset(offset).Find(&rows).Error
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}

	writeOK(w, rows, apiMeta{"limit": limit, "offset": offset, "total": total})
}
