package v1

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *Service) handleIngestionRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	limit := queryClampInt(queryParseIntOrDefault(r.URL.Query().Get("limit"), 50), 1, 500)
	offset := queryParseIntOrDefault(r.URL.Query().Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	result, err := h.q.ListIngestionRuns(ctx, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}

	writeOK(w, result.Rows, apiMeta{"limit": result.Limit, "offset": result.Offset, "total": result.Total})
}

func (h *Service) handleIngestionFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	limit := queryClampInt(queryParseIntOrDefault(r.URL.Query().Get("limit"), 100), 1, 1000)
	offset := queryParseIntOrDefault(r.URL.Query().Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	runIDStr := strings.TrimSpace(r.URL.Query().Get("run_id"))

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	var runID *uint
	if runIDStr != "" {
		n, err := strconv.Atoi(runIDStr)
		if err != nil || n <= 0 {
			writeErr(w, http.StatusBadRequest, "INVALID_ARGUMENT", "run_id must be positive integer")
			return
		}
		id := uint(n)
		runID = &id
	}

	result, err := h.q.ListIngestionFiles(ctx, runID, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "DB_ERROR", "query failed")
		return
	}

	writeOK(w, result.Rows, apiMeta{"limit": result.Limit, "offset": result.Offset, "total": result.Total})
}
